package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/loom/node-worker-pool/internal/nodes" // Run init functions to register nodes
	"github.com/loom/node-worker-pool/internal/worker"
)

func main() {
	log.Println("Starting Node Worker Pool...")

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	w, err := worker.NewWorker(rabbitURL)
	if err != nil {
		log.Fatalf("Failed to initialize worker: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Handle graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		log.Println("Received shutdown signal")
		cancel()
	}()

	if err := w.Start(ctx); err != nil {
		log.Fatalf("Worker stopped with error: %v", err)
	}

	log.Println("Worker shut down gracefully.")
}
