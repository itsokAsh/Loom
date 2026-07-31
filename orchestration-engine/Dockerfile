FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy and build orchestration engine
COPY orchestration-engine/ ./orchestration-engine/
WORKDIR /app/orchestration-engine
RUN go mod download
RUN go build -o engine ./cmd/engine/main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
WORKDIR /root/
COPY --from=builder /app/orchestration-engine/engine .
CMD ["./engine"]
