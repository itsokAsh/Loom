package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/loom/sdk-go/loom"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: go run main.go <webhook_path> <webhook_secret>")
	}
	
	webhookPath := os.Args[1]
	webhookSecret := os.Args[2]

	client, err := loom.NewWebhookClient(
		"http://localhost:8080/v1",
		webhookPath,
		webhookSecret,
		loom.WithAllowInsecure(true),
		loom.WithMaxRetries(3),
	)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	payload := map[string]interface{}{
		"event": "user_signup",
		"data": map[string]string{
			"email": "test@example.com",
		},
	}

	fmt.Println("Triggering webhook...")
	res, err := client.Trigger(context.Background(), payload)
	
	if err != nil {
		var rl *loom.RateLimitedError
		if errors.As(err, &rl) {
			log.Fatalf("Rate limited! Try again after: %v", rl.RetryAfter)
		}
		
		var isErr *loom.InvalidSignatureError
		if errors.As(err, &isErr) {
			log.Fatalf("Invalid signature - did you provide the right secret?")
		}
		
		log.Fatalf("Failed to trigger webhook: %v", err)
	}

	fmt.Printf("Successfully triggered workflow!\nExecution ID: %s\nStatus: %s\n", res.ExecutionID, res.Status)
}
