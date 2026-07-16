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
	"github.com/go-chi/chi/v5/middleware"
	"github.com/loom/trigger-api/internal/db"
	"github.com/loom/trigger-api/internal/queue"
	"github.com/loom/trigger-api/internal/schedules"
	"github.com/loom/trigger-api/internal/webhooks"
	"github.com/loom/trigger-api/internal/workflows"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres_password@localhost:5440/trigger_db?sslmode=disable"
	}
	store, err := db.NewStore(ctx, dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	rmqURL := os.Getenv("RABBITMQ_URL")
	if rmqURL == "" {
		rmqURL = "amqp://guest:guest@localhost:5672/"
	}
	publisher, err := queue.NewPublisher(rmqURL, "trigger-to-orchestration")
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer publisher.Close()

	workflowService := workflows.NewService(store)
	workflowHandler := workflows.NewHandler(workflowService, store)

	webhookHandler := webhooks.NewHandler(store, publisher)
	schedulePoller := schedules.NewPoller(store, publisher)

	go schedulePoller.Start(ctx)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Post("/webhooks/{path}", webhookHandler.HandleIncomingWebhook)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/workflows", workflowHandler.CreateWorkflow)
		r.Post("/workflows/{id}/versions", workflowHandler.AddVersion)
		r.Get("/workflows/{id}", workflowHandler.GetWorkflow)
		r.Post("/workflows/{id}/webhooks", workflowHandler.CreateWebhook)
		r.Post("/workflows/{id}/schedules", workflowHandler.CreateSchedule)
		r.Get("/workflows/{id}/executions", workflowHandler.ListExecutions)
		r.Get("/executions/{id}", workflowHandler.GetExecution)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Println("Starting Trigger/API Service on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down gracefully...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Shutdown complete.")
}
