package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/loom/orchestration-engine/internal/api"
	"github.com/loom/orchestration-engine/internal/dag"
	"github.com/loom/orchestration-engine/internal/db"
	"github.com/loom/orchestration-engine/internal/engine"
	"github.com/loom/orchestration-engine/internal/queue"
	"github.com/loom/shared/dburl"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres_password@localhost:5440/orchestration_db?sslmode=disable"
	}
	dsn = dburl.WithDatabaseName(dsn, os.Getenv("DATABASE_NAME"))
	store, err := db.NewStore(ctx, dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	rmqURL := os.Getenv("RABBITMQ_URL")
	if rmqURL == "" {
		rmqURL = "amqp://guest:guest@localhost:5672/"
	}
	
	rmq, err := queue.NewRabbitMQ(rmqURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rmq.Close()

	evaluator := dag.NewEvaluator()
	orchestrator := engine.NewOrchestrator(store, rmq, evaluator)
	
	relay := engine.NewOutboxRelay(store, rmq)
	go relay.Start(ctx)

	if err := rmq.ConsumeNewRuns(orchestrator.HandleNewRun); err != nil {
		log.Fatalf("Failed to start ConsumeNewRuns: %v", err)
	}
	log.Println("Listening for NewRunMessages...")

	if err := rmq.ConsumeNodeResults(orchestrator.HandleNodeResult); err != nil {
		log.Fatalf("Failed to start ConsumeNodeResults: %v", err)
	}
	log.Println("Listening for NodeResultMessages...")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Health + execution API server
	apiHandler := api.NewHandler(store)
	go func() {
		r := chi.NewRouter()
		// Lightweight check only — do NOT open new RabbitMQ/DB pools per probe
		// (Render health checks every few seconds; free CloudAMQP has low connection limits).
		r.Get("/healthz", func(res http.ResponseWriter, req *http.Request) {
			if err := store.Ping(req.Context()); err != nil {
				res.WriteHeader(http.StatusServiceUnavailable)
				res.Write([]byte("Database unreachable\n"))
				return
			}
			res.WriteHeader(http.StatusOK)
			res.Write([]byte("OK\n"))
		})
		r.Get("/executions/{id}", apiHandler.GetExecution)
		r.Get("/executions/{id}/nodes", apiHandler.ListNodeExecutions)

		port := os.Getenv("PORT")
		if port == "" {
			port = "8081"
		}
		addr := ":" + port

		server := &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
		log.Printf("API server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server error: %v", err)
		}
	}()

	<-stop

	log.Println("Shutting down gracefully...")
	cancel()
}
