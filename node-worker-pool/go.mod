module github.com/loom/node-worker-pool

go 1.25.0

require (
	github.com/loom/shared v0.0.0-00010101000000-000000000000
	github.com/rabbitmq/amqp091-go v1.9.0
	golang.org/x/time v0.9.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace github.com/loom/shared => ../shared
