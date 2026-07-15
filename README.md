# Loom

Loom is a backend-focused, microservice-based workflow execution engine inspired by tools like n8n and Zapier. It is strictly focused on solving the complex distributed-systems challenges of executing JSON-defined DAG (Directed Acyclic Graph) workflows reliably, featuring built-in retries, delays, conditional branching, and crash-safe resumption.

## Architecture

The system is decoupled into three independent Go microservices that communicate asynchronously via RabbitMQ and persist state securely in PostgreSQL:

1. **Trigger/API Service**: The entry point. Handles workflow CRUD operations, ingests webhooks, manages cron schedules via `SKIP LOCKED` polling, and provides execution history.
2. **Orchestration Engine**: The "brain". A state machine that traverses the workflow DAG, evaluates conditions, and decides what node executes next without losing state during crashes.
3. **Node Worker Pool**: A stateless, horizontally scalable pool of workers that execute the actual node logic (e.g., HTTP requests, data transformations).

For detailed documentation, refer to the [docs folder](./docs).

## Prerequisites

- [Docker](https://www.docker.com/) and Docker Compose
- Go 1.23+ (If compiling or running locally outside of Docker)
- [sqlc](https://sqlc.dev/) (For generating database queries)

## How to Run

Loom relies on Docker Compose to orchestrate its infrastructure dependencies (PostgreSQL, RabbitMQ).

1. Start the infrastructure in the background:
   ```bash
   docker-compose up -d
   ```

2. (Optional) Re-generate the typed database models if you modify the SQL schema:
   ```bash
   cd trigger-api
   sqlc generate
   ```

3. Build and run the API service:
   ```bash
   cd trigger-api
   go run ./cmd/api/main.go
   ```
   
*(Note: The Orchestration Engine and Node Worker Pool are currently under active development).*
