FROM golang:1.23-alpine AS builder
WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy and build trigger api
COPY trigger-api/ ./trigger-api/
WORKDIR /app/trigger-api
RUN go mod download
RUN go build -o api ./cmd/api/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/trigger-api/api .
CMD ["./api"]
