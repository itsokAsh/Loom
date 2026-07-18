module github.com/loom/orchestration-engine

go 1.23

replace github.com/loom/shared => ../shared

require (
	github.com/jackc/pgx/v5 v5.6.0
	github.com/loom/shared v0.0.0-00010101000000-000000000000
	github.com/rabbitmq/amqp091-go v1.12.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)
