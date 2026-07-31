package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	contracts "github.com/loom/shared/queue-contracts"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewRabbitMQ(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	if err := ch.Qos(10, 0, false); err != nil {
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	// Worker task queue uses a dead-letter exchange (must match node-worker-pool).
	if err := setupWorkerTaskQueue(ch); err != nil {
		return nil, err
	}

	queues := []string{
		"trigger-to-orchestration",
		"worker-to-orchestration",
		"orchestration-to-trigger-status",
	}

	for _, q := range queues {
		_, err = ch.QueueDeclare(
			q,
			true,  // durable
			false, // delete when unused
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return nil, fmt.Errorf("failed to declare queue %s: %w", q, err)
		}
	}

	if err := ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("failed to put channel in confirm mode: %w", err)
	}

	return &RabbitMQ{
		conn: conn,
		ch:   ch,
	}, nil
}

func (r *RabbitMQ) Close() {
	r.ch.Close()
	r.conn.Close()
}

func (r *RabbitMQ) ConsumeNewRuns(handler func(contracts.NewRunMessage) error) error {
	msgs, err := r.ch.Consume(
		"trigger-to-orchestration",
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for d := range msgs {
			var msg contracts.NewRunMessage
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				log.Printf("Failed to unmarshal NewRunMessage: %v", err)
				d.Nack(false, false)
				continue
			}

			if err := handler(msg); err != nil {
				log.Printf("Failed to handle NewRunMessage: %v", err)
				d.Nack(false, true)
			} else {
				d.Ack(false)
			}
		}
	}()
	return nil
}

func (r *RabbitMQ) ConsumeNodeResults(handler func(contracts.NodeResultMessage) error) error {
	msgs, err := r.ch.Consume(
		"worker-to-orchestration",
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for d := range msgs {
			var msg contracts.NodeResultMessage
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				log.Printf("Failed to unmarshal NodeResultMessage: %v", err)
				d.Nack(false, false)
				continue
			}

			if err := handler(msg); err != nil {
				log.Printf("Failed to handle NodeResultMessage: %v", err)
				d.Nack(false, true)
			} else {
				d.Ack(false)
			}
		}
	}()
	return nil
}

func (r *RabbitMQ) PublishNodeTask(ctx context.Context, msg contracts.NodeTaskMessage) error {
	body, _ := json.Marshal(msg)
	return r.PublishRaw(ctx, "orchestration-to-worker", body)
}

func (r *RabbitMQ) PublishExecutionStatus(ctx context.Context, msg contracts.ExecutionStatusMessage) error {
	body, _ := json.Marshal(msg)
	return r.PublishRaw(ctx, "orchestration-to-trigger-status", body)
}

func (r *RabbitMQ) PublishRaw(ctx context.Context, queue string, body []byte) error {
	err := r.ch.PublishWithContext(
		ctx,
		"",
		queue,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

// setupWorkerTaskQueue declares DLX/DLQ and orchestration-to-worker with the same args as node-worker-pool.
func setupWorkerTaskQueue(ch *amqp.Channel) error {
	dlxName := "worker-dlx"
	if err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("failed to declare DLX: %w", err)
	}

	dlqName := "worker-dlq"
	if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	if err := ch.QueueBind(dlqName, "worker-dlq-key", dlxName, false, nil); err != nil {
		return fmt.Errorf("failed to bind DLQ: %w", err)
	}

	args := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": "worker-dlq-key",
	}

	// Do not use QueueDeclarePassive here: a missing queue returns 404 and
	// closes the channel, so the following Declare fails with 504.
	_, err := ch.QueueDeclare(
		"orchestration-to-worker",
		true,
		false,
		false,
		false,
		args,
	)
	if err != nil {
		return fmt.Errorf("failed to declare orchestration-to-worker: %w", err)
	}
	return nil
}
