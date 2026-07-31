FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy and build trigger api
COPY trigger-api/ ./trigger-api/
WORKDIR /app/trigger-api
RUN go mod download
RUN go build -o api ./cmd/api/main.go

FROM alpine:latest
RUN apk add --no-cache wget ca-certificates
WORKDIR /root/
COPY --from=builder /app/trigger-api/api .
CMD ["./api"]
