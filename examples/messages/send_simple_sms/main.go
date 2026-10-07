// Send a simple SMS message.
//
//	CALISERO_API_KEY=... CALISERO_TEST_PHONE=+40742123456 go run ./examples/messages/send_simple_sms
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func main() {
	// Your API key, from the dashboard's API Keys section
	apiKey := cmp.Or(os.Getenv("CALISERO_API_KEY"), "your-api-key-here")
	recipient := cmp.Or(os.Getenv("CALISERO_TEST_PHONE"), "+40742***350")

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Send Simple SMS ===\n\n")

	response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{
		Recipient: recipient,
		Body:      "Hello from Calisero! This is a simple test message.",
	})
	if err != nil {
		report(err)
		os.Exit(1)
	}

	message := response.Data

	fmt.Println("✅ Message sent successfully!")
	fmt.Printf("📨 Message ID: %s\n", message.ID)
	fmt.Printf("📱 Recipient: %s\n", message.Recipient)
	fmt.Printf("💬 Body: %s\n", message.Body)
	fmt.Printf("📊 Status: %s\n", message.Status)
	fmt.Printf("🧩 Parts: %d\n", message.Parts)
	fmt.Printf("⏰ Created: %s\n", message.CreatedAt.Format(time.RFC3339))
	fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(response.ResponseMeta.TraceID, "N/A"))
}

// report prints an error of the client, the most specific types first.
func report(err error) {
	var validationError *calisero.ValidationError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &validationError):
		fmt.Printf("❌ Validation error: %s\n", validationError.Message)

		for field, messages := range validationError.ValidationErrors {
			fmt.Printf("  - %s: %s\n", field, strings.Join(messages, ", "))
		}
	case errors.As(err, &apiError):
		fmt.Printf("❌ API error: %s\n", apiError.Message)
		fmt.Printf("🔢 Status Code: %s\n", statusOf(apiError))
		fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))
	default:
		log.Fatal(err)
	}
}

// statusOf is the error's HTTP status, or N/A when the request got no answer.
func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
