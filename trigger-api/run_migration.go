package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://postgres:postgres_password@localhost:5440/trigger_db?sslmode=disable")
	if err != nil {
		fmt.Println("Failed to connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	sqlBytes, err := os.ReadFile("migrations/000002_add_template_support.up.sql")
	if err != nil {
		fmt.Println("Failed to read sql:", err)
		os.Exit(1)
	}
	
	_, err = pool.Exec(ctx, string(sqlBytes))
	if err != nil {
		fmt.Println("Error executing migration:", err)
		os.Exit(1)
	}
	fmt.Println("Migration 000002_add_template_support applied successfully")
}
