package schedules

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/robfig/cron/v3"
	"github.com/loom/trigger-api/internal/db"
	"github.com/loom/trigger-api/internal/queue"
	contracts "github.com/loom/shared/queue-contracts"
)

type Poller struct {
	store     *db.Store
	publisher *queue.Publisher
	parser    cron.Parser
}

func NewPoller(store *db.Store, publisher *queue.Publisher) *Poller {
	return &Poller{
		store:     store,
		publisher: publisher,
		parser:    cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	leaseDuration := 5 * time.Minute
	leasedBy := pgtype.Text{String: "instance-1", Valid: true}
	now := time.Now()
	expiresAt := pgtype.Timestamptz{Time: now.Add(leaseDuration), Valid: true}
	nowTs := pgtype.Timestamptz{Time: now, Valid: true}

	// Since sqlc params type mappings depend on pgtype/sql package, we use placeholders.
	schedules, err := p.store.ClaimDueSchedules(ctx, db.ClaimDueSchedulesParams{
		LeasedBy:       leasedBy,
		LeaseExpiresAt: expiresAt,
		NextRunAt:      nowTs,
		Limit:          50,
	})

	if err != nil {
		log.Printf("Failed to claim schedules: %v", err)
		return
	}

	for _, s := range schedules {
		p.processSchedule(ctx, s)
	}
}

func (p *Poller) processSchedule(ctx context.Context, s db.Schedule) {
	wv, err := p.store.GetWorkflowVersion(ctx, db.GetWorkflowVersionParams{
		WorkflowID: s.WorkflowID,
		Version:    1, // Fetch latest
	})
	if err != nil {
		log.Printf("Failed to fetch workflow version for schedule %v: %v", s.ID, err)
		return
	}

	var dag contracts.DAGDefinition
	if err := json.Unmarshal(wv.DagDefinition, &dag); err != nil {
		log.Printf("Failed to parse DAG for schedule %v: %v", s.ID, err)
		return
	}

	idempKey := fmt.Sprintf("cron-%v-%v", s.ID, s.NextRunAt.Time.Unix())

	exec, err := p.store.CreateExecution(ctx, db.CreateExecutionParams{
		WorkflowID:      s.WorkflowID,
		WorkflowVersion: 1,
		IdempotencyKey:  idempKey,
		Status:          "PENDING",
	})
	if err != nil {
		return // Likely duplicate, DO NOTHING caught it
	}

	msg := contracts.NewRunMessage{
		ExecutionID:     fmt.Sprintf("%x", exec.ID.Bytes),
		WorkflowID:      fmt.Sprintf("%x", exec.WorkflowID.Bytes),
		WorkflowVersion: int(exec.WorkflowVersion),
		IdempotencyKey:  idempKey,
		TriggerData:     map[string]interface{}{"cron_time": s.NextRunAt.Time},
		WorkflowDAG:     dag,
	}

	if err := p.publisher.PublishNewRun(ctx, msg); err != nil {
		log.Printf("Failed to publish run for schedule %v: %v", s.ID, err)
		return
	}

	schedule, err := p.parser.Parse(s.CronExpression)
	if err != nil {
		log.Printf("Invalid cron expression %v: %v", s.ID, err)
		return
	}

	nextRun := schedule.Next(time.Now())

	err = p.store.UpdateScheduleNextRun(ctx, db.UpdateScheduleNextRunParams{
		ID:        s.ID,
		NextRunAt: pgtype.Timestamptz{Time: nextRun, Valid: true},
	})
	if err != nil {
		log.Printf("Failed to update next run for schedule %v: %v", s.ID, err)
	}
}
