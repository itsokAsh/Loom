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
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/loom/shared/dburl"
	"github.com/loom/trigger-api/internal/credentials"
	"github.com/loom/trigger-api/internal/db"
	"github.com/loom/trigger-api/internal/middleware"
	"github.com/loom/trigger-api/internal/orchestration"
	"github.com/loom/trigger-api/internal/queue"
	"github.com/loom/trigger-api/internal/schedules"
	"github.com/loom/trigger-api/internal/templates"
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
	publisher, err := queue.NewPublisher(rmqURL, "trigger-to-orchestration")
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer publisher.Close()

	workflowService := workflows.NewService(store)
	orchClient := orchestration.NewClient()
	workflowHandler := workflows.NewHandler(workflowService, store, orchClient, publisher)

	credStore, err := credentials.NewStore(store.Pool)
	if err != nil {
		log.Fatalf("Failed to init credentials store: %v", err)
	}
	credHandler := credentials.NewHandler(credStore)

	publicAPIBase := os.Getenv("TRIGGER_PUBLIC_URL")
	if publicAPIBase == "" {
		publicAPIBase = "http://localhost:3000/v1"
	}
	templateHandler := templates.NewTemplateHandler(templates.NewStoreAdapter(store), publicAPIBase)
	webhookHandler := webhooks.NewHandler(store, publisher)
	schedulePoller := schedules.NewPoller(store, publisher)

	rateLimiter := middleware.NewWebhookRateLimiter()

	consumer, err := queue.NewConsumer(rmqURL, "orchestration-to-trigger-status")
	if err != nil {
		log.Fatalf("Failed to create status consumer: %v", err)
	}
	defer consumer.Close()

	go func() {
		if err := consumer.Start(ctx, workflows.NewStatusHandler(store)); err != nil {
			log.Printf("Status consumer error: %v", err)
		}
	}()

	go schedulePoller.Start(ctx)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))
	r.Use(middleware.CORS(os.Getenv("CORS_ORIGINS")))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Old webhook endpoint deprecation redirect
	r.Post("/webhooks/{path}", func(w http.ResponseWriter, r *http.Request) {
		path := chi.URLParam(r, "path")
		http.Redirect(w, r, "/v1/webhooks/"+path, http.StatusPermanentRedirect)
	})

	r.Route("/v1", func(r chi.Router) {
		// Webhook trigger route
		r.With(
			middleware.WebhookAuthMiddleware(store),
			rateLimiter.Middleware(),
		).Post("/webhooks/{path}", webhookHandler.HandleIncomingWebhook)

		// Worker-only secret resolve (service token, not admin key)
		r.Get("/internal/credentials/{id}", credHandler.ResolveSecret)

		// Management routes protected by API key
		r.Group(func(r chi.Router) {
			adminKey := os.Getenv("ADMIN_API_KEY")
			if adminKey == "" {
				adminKey = "dev-admin-key" // Default for local dev
			}
			r.Use(middleware.AdminAPIKeyMiddleware(adminKey))

			r.Post("/workflows", workflowHandler.CreateWorkflow)
			r.Get("/workflows", workflowHandler.ListWorkflows)
			r.Post("/workflows/validate", workflowHandler.ValidateWorkflowDAG)
			r.Post("/workflows/{id}/versions", workflowHandler.AddVersion)
			r.Get("/workflows/{id}", workflowHandler.GetWorkflow)
			r.Patch("/workflows/{id}", workflowHandler.UpdateWorkflow)
			r.Delete("/workflows/{id}", workflowHandler.DeleteWorkflow)
			r.Post("/workflows/{id}/execute", workflowHandler.ExecuteWorkflow)
			r.Post("/workflows/{id}/webhooks", workflowHandler.CreateWebhook)
			r.Get("/workflows/{id}/webhooks", workflowHandler.ListWebhooks)
			r.Post("/workflows/{id}/schedules", workflowHandler.CreateSchedule)
			r.Get("/workflows/{id}/schedules", workflowHandler.ListSchedules)
			r.Delete("/workflows/{id}/schedules/{scheduleId}", workflowHandler.DeleteSchedule)
			r.Get("/workflows/{id}/executions", workflowHandler.ListExecutions)
			r.Get("/executions/{id}", workflowHandler.GetExecution)
			r.Get("/executions/{id}/nodes", workflowHandler.GetExecutionNodes)

			r.Get("/credentials", credHandler.List)
			r.Post("/credentials", credHandler.Create)
			r.Delete("/credentials/{id}", credHandler.Delete)

			// Templates
			r.Get("/templates", templateHandler.ListTemplates)
			r.Post("/templates/{id}/create", templateHandler.CreateFromTemplate)
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Println("Starting Trigger/API Service on :" + port)
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