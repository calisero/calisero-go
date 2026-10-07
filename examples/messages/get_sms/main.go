// Retrieve a message and its delivery status.
//
//	CALISERO_API_KEY=... go run ./examples/messages/get_sms <message-id>
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func main() {
	apiKey := cmp.Or(os.Getenv("CALISERO_API_KEY"), "your-api-key-here")
	messageID := "9e2574e8-3615-4090-9b5a-0fc812079da8"

	if len(os.Args) > 1 {
		messageID = os.Args[1]
	}

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Get SMS Message ===\n\n")

	response, err := client.Messages.Get(context.Background(), messageID)

	var notFound *calisero.NotFoundError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &notFound):
		fmt.Printf("❌ Message not found: %s\n", notFound.Message)
		fmt.Printf("💡 Please check if the message ID '%s' exists\n", messageID)
		os.Exit(1)
	case errors.As(err, &apiError):
		fmt.Printf("❌ API error: %s\n", apiError.Message)
		fmt.Printf("🔢 Status Code: %s\n", statusOf(apiError))
		fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))
		os.Exit(1)
	case err != nil:
		log.Fatal(err)
	}

	message := response.Data

	fmt.Println("✅ Message retrieved successfully!")
	fmt.Printf("📨 Message ID: %s\n", message.ID)
	fmt.Printf("📱 Recipient: %s\n", message.Recipient)
	fmt.Printf("💬 Body: %s\n", message.Body)
	fmt.Printf("📊 Status: %s\n", message.Status)
	fmt.Printf("🧩 Parts: %d\n", message.Parts)
	fmt.Printf("⏰ Created: %s\n", message.CreatedAt.Format(time.RFC3339))
	fmt.Printf("📅 Scheduled: %s\n", formatTime(message.ScheduledAt, "Sent immediately"))
	fmt.Printf("📤 Sent: %s\n", formatTime(message.SentAt, "Not sent yet"))
	fmt.Printf("📥 Delivered: %s\n", formatTime(message.DeliveredAt, "Not delivered yet"))
	fmt.Printf("🔗 Callback URL: %s\n", cmp.Or(message.CallbackURL, "None"))
	fmt.Printf("👤 Sender: %s\n", cmp.Or(message.Sender, "Default"))

	for _, link := range message.ShortenedURLs {
		fmt.Printf("✂️ %s: %d clicks, last %s\n", link.ShortenedLink, link.ClickCount, formatTime(link.LastClick, "never"))
	}
}

func formatTime(t *time.Time, fallback string) string {
	if t == nil {
		return fallback
	}

	return t.Format(time.RFC3339)
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
