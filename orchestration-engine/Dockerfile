FROM golang:1.23-alpine AS builder
WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy and build orchestration engine
COPY orchestration-engine/ ./orchestration-engine/
WORKDIR /app/orchestration-engine
RUN go mod download
RUN go build -o engine ./cmd/engine/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/orchestration-engine/engine .
CMD ["./engine"]
