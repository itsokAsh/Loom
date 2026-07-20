package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/loom/node-worker-pool/internal/db"
	"github.com/loom/node-worker-pool/internal/executor"
	"github.com/loom/node-worker-pool/internal/nodes"
	contracts "github.com/loom/shared/queue-contracts"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Worker struct {
	conn        *amqp.Connection
	channel     *amqp.Channel
	taskQueue   string
	resultQueue string
	store       *db.Store
}

func NewWorker(url string, store *db.Store) (*Worker, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	// Setup DLQ
	dlxName := "worker-dlx"
	err = ch.ExchangeDeclare(
		dlxName,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare DLX: %w", err)
	}

	dlqName := "worker-dlq"
	_, err = ch.QueueDeclare(
		dlqName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare DLQ: %w", err)
	}

	err = ch.QueueBind(
		dlqName,
		"worker-dlq-key",
		dlxName,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to bind DLQ: %w", err)
	}

	taskQueue := "orchestration-to-worker"
	
	args := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": "worker-dlq-key",
	}

	_, err = ch.QueueDeclare(
		taskQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		args,  // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare task queue: %w", err)
	}

	resultQueue := "worker-to-orchestration"
	_, err = ch.QueueDeclare(
		resultQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare result queue: %w", err)
	}

	// Fair dispatch
	err = ch.Qos(50, 0, false)
	if err != nil {
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	return &Worker{
		conn:        conn,
		channel:     ch,
		taskQueue:   taskQueue,
		resultQueue: resultQueue,
		store:       store,
	}, nil
}

func (w *Worker) Start(ctx context.Context) error {
	msgs, err := w.channel.Consume(
		w.taskQueue,
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	log.Printf("Worker started, waiting for messages on %s...", w.taskQueue)

	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down worker...")
			return nil
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("rabbitmq channel closed")
			}

			go w.processMessage(ctx, d)
		}
	}
}

func (w *Worker) processMessage(ctx context.Context, d amqp.Delivery) {
	var task contracts.NodeTaskMessage
	if err := json.Unmarshal(d.Body, &task); err != nil {
		log.Printf("Error unmarshaling task: %v", err)
		d.Reject(false) // Reject entirely (to DLQ) if invalid JSON
		return
	}

	exec, err := nodes.Get(task.NodeType)
	var resultMsg contracts.NodeResultMessage
	resultMsg.ExecutionID = task.ExecutionID
	resultMsg.NodeID = task.NodeID
	resultMsg.DispatchID = task.DispatchID
	resultMsg.CompletedAt = time.Now()

	if err != nil {
		resultMsg.Status = "ERROR"
		resultMsg.ErrorMessage = err.Error()
	} else {
		// Execute the node wrapped with the Idempotency Decorator
		output, skipped, execErr := executor.IdempotentExecute(ctx, w.store, task, exec.Execute)
		
		if execErr != nil {
			resultMsg.Status = "ERROR"
			resultMsg.ErrorMessage = execErr.Error()
			if output != nil {
				resultMsg.OutputData = output
			}
			
			// Custom Max Retry Logic (RabbitMQ headers checking for death count)
			var deathCount int64 = 0
			if deaths, ok := d.Headers["x-death"].([]interface{}); ok && len(deaths) > 0 {
				if death, ok := deaths[0].(amqp.Table); ok {
					if count, ok := death["count"].(int64); ok {
						deathCount = count
					}
				}
			}
			
			if deathCount >= 3 { // Max Retries = 3
				log.Printf("Task %s exceeded max retries. Sending to DLQ.", task.DispatchID)
				d.Reject(false) // Reject without requeueing (goes to DLQ)
				return
			}
			
		} else {
			resultMsg.Status = "SUCCESS"
			resultMsg.OutputData = output
			if skipped {
				log.Printf("Task %s was skipped (already COMPLETED)", task.DispatchID)
			}
		}
	}

	// Publish result (Skip path STILL publishes result to advance workflow)
	if err := w.publishResult(ctx, resultMsg); err != nil {
		log.Printf("Failed to publish result for task %s: %v", task.DispatchID, err)
		// We failed to publish the result, so we MUST NOT ack the message.
		d.Nack(false, true) // Requeue
		return
	}

	// Ack ordering: ONLY ack AFTER successfully executing and publishing the result.
	d.Ack(false)
	log.Printf("Successfully processed task %s (Node: %s, Status: %s)", task.DispatchID, task.NodeID, resultMsg.Status)
}

func (w *Worker) publishResult(ctx context.Context, result contracts.NodeResultMessage) error {
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result message: %w", err)
	}

	err = w.channel.PublishWithContext(ctx,
		"",            // exchange
		w.resultQueue, // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		})
	return err
}

func (w *Worker) Close() error {
	if err := w.channel.Close(); err != nil {
		return err
	}
	return w.conn.Close()
}
