package engine

import (
	"context"
	"log"
	"time"

	"github.com/loom/orchestration-engine/internal/db"
	"github.com/loom/orchestration-engine/internal/queue"
)

type OutboxRelay struct {
	store *db.Store
	rmq   *queue.RabbitMQ
}

func NewOutboxRelay(store *db.Store, rmq *queue.RabbitMQ) *OutboxRelay {
	return &OutboxRelay{
		store: store,
		rmq:   rmq,
	}
}

func (r *OutboxRelay) Start(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.processOutbox(ctx)
		}
	}
}

func (r *OutboxRelay) processOutbox(ctx context.Context) {
	err := r.store.WithTx(ctx, func(ctx context.Context, qtx *db.Queries) error {
		msgs, err := qtx.ClaimUnpublishedMessages(ctx, 100)
		if err != nil {
			return err
		}

		for _, msg := range msgs {
			err := r.rmq.PublishRaw(ctx, msg.Queue, msg.Payload)
			if err != nil {
				log.Printf("OutboxRelay: failed to publish msg %v: %v", msg.ID, err)
				continue
			}

			if err := qtx.MarkOutboxMessagePublished(ctx, msg.ID); err != nil {
				log.Printf("OutboxRelay: failed to mark msg %v as published: %v", msg.ID, err)
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("OutboxRelay: error processing outbox: %v", err)
	}
}
