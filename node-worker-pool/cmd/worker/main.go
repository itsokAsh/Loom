package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/loom/node-worker-pool/internal/db"
	_ "github.com/loom/node-worker-pool/internal/nodes" // Run init functions to register nodes
	"github.com/loom/node-worker-pool/internal/worker"
	"github.com/loom/shared/dburl"
)

func main() {
	log.Println("Starting Node Worker Pool...")
	
	ctx, cancel := context.WithCancel(context.Background())

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres_password@localhost:5440/worker_db?sslmode=disable"
	}
	dsn = dburl.WithDatabaseName(dsn, os.Getenv("DATABASE_NAME"))
	store, err := db.NewStore(ctx, dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	w, err := worker.NewWorker(rabbitURL, store)
	if err != nil {
		log.Fatalf("Failed to initialize worker: %v", err)
	}
	defer w.Close()

	// Health check server
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(res http.ResponseWriter, req *http.Request) {
			// Actively ping DB
			if _, pingErr := db.NewStore(req.Context(), dsn); pingErr != nil {
				res.WriteHeader(http.StatusServiceUnavailable)
				res.Write([]byte("Database unreachable\n"))
				return
			}
			
			// We ideally want to ping RabbitMQ but w.conn isn't exposed and amqp doesn't have an easy ping.
			// Re-dialing to verify connection works as a readiness check.
			if _, dialErr := worker.NewWorker(rabbitURL, store); dialErr != nil {
				res.WriteHeader(http.StatusServiceUnavailable)
				res.Write([]byte("RabbitMQ unreachable\n"))
				return
			}
			
			res.WriteHeader(http.StatusOK)
			res.Write([]byte("OK\n"))
		})
		
		port := os.Getenv("PORT")
		if port == "" {
			port = "8081"
		}
		addr := ":" + port

		server := &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
		log.Printf("Health server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Health server error: %v", err)
		}
	}()

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
