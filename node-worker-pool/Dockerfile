FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum from shared
COPY shared/ ../shared/

# Copy worker pool source
COPY node-worker-pool/ .

RUN go mod download
RUN go build -o worker ./cmd/worker

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /app/worker .

CMD ["./worker"]
