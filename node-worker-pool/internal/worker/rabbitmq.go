package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/loom/node-worker-pool/internal/nodes"
	contracts "github.com/loom/shared/queue-contracts"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Worker struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	taskQueue  string
	resultQueue string
}

func NewWorker(url string) (*Worker, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	taskQueue := "orchestration-to-worker"
	_, err = ch.QueueDeclare(
		taskQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
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
	err = ch.Qos(
		50,    // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	return &Worker{
		conn:        conn,
		channel:     ch,
		taskQueue:   taskQueue,
		resultQueue: resultQueue,
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
	log.Printf("Received a message: %s", d.Body)

	var task contracts.NodeTaskMessage
	if err := json.Unmarshal(d.Body, &task); err != nil {
		log.Printf("Error unmarshaling task: %v", err)
		d.Nack(false, false) // Reject and drop invalid message
		return
	}

	executor, err := nodes.Get(task.NodeType)
	var resultMsg contracts.NodeResultMessage
	resultMsg.ExecutionID = task.ExecutionID
	resultMsg.NodeID = task.NodeID
	resultMsg.DispatchID = task.DispatchID
	resultMsg.CompletedAt = time.Now()

	if err != nil {
		resultMsg.Status = "ERROR"
		resultMsg.ErrorMessage = err.Error()
	} else {
		// Execute the node
		output, execErr := executor.Execute(ctx, task.Config)
		if execErr != nil {
			resultMsg.Status = "ERROR"
			resultMsg.ErrorMessage = execErr.Error()
			if output != nil {
				resultMsg.OutputData = output
			}
		} else {
			resultMsg.Status = "SUCCESS"
			resultMsg.OutputData = output
		}
	}

	// Publish result
	if err := w.publishResult(ctx, resultMsg); err != nil {
		log.Printf("Failed to publish result for task %s: %v", task.DispatchID, err)
		d.Nack(false, true) // Requeue if we can't publish result
		return
	}

	// Acknowledge the task message
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
