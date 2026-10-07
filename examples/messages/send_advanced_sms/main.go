// Send an SMS with every option: a body shown masked, a validity, a scheduled
// time, a delivery status callback, a sender ID and URL shortening.
//
//	CALISERO_API_KEY=... CALISERO_TEST_PHONE=+40742123456 go run ./examples/messages/send_advanced_sms
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
	apiKey := cmp.Or(os.Getenv("CALISERO_API_KEY"), "your-api-key-here")
	recipient := cmp.Or(os.Getenv("CALISERO_TEST_PHONE"), "+40742***350")

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Send Advanced SMS ===\n\n")

	response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{
		Recipient: recipient,
		Body:      "Your code is 123456. Manage alerts: https://yourapp.com/account/alerts",
		// What the API shows instead of the body in its answers and in the dashboard
		VisibleBody: "Your code is ******. Manage alerts: https://yourapp.com/account/alerts",
		Validity:    24, // hours
		// A time.Time is converted to the Romania time the API expects
		ScheduleAt:  time.Now().Add(time.Hour),
		CallbackURL: "https://yourapp.com/webhooks/sms",
		// One of your approved sender IDs; leave it out for the default sender
		Sender:      "Calisero",
		ShortenURLs: true,
	})
	if err != nil {
		report(err)
		os.Exit(1)
	}

	message := response.Data

	fmt.Println("✅ Advanced message sent successfully!")
	fmt.Printf("📨 Message ID: %s\n", message.ID)
	fmt.Printf("📱 Recipient: %s\n", message.Recipient)
	fmt.Printf("💬 Body: %s\n", message.Body)
	fmt.Printf("📊 Status: %s\n", message.Status)
	fmt.Printf("🧩 Parts: %d\n", message.Parts)
	fmt.Printf("⏰ Created: %s\n", message.CreatedAt.Format(time.RFC3339))
	fmt.Printf("📅 Scheduled: %s\n", formatTime(message.ScheduledAt, "Send immediately"))
	fmt.Printf("🔗 Callback URL: %s\n", cmp.Or(message.CallbackURL, "None"))
	fmt.Printf("👤 Sender: %s\n", cmp.Or(message.Sender, "Default"))

	// The links of the body that were replaced by short ones
	if len(message.ShortenedURLs) == 0 {
		fmt.Println("✂️ Shortened URLs: None")
	} else {
		fmt.Printf("✂️ Shortened URLs: %d\n", len(message.ShortenedURLs))
	}

	for _, link := range message.ShortenedURLs {
		fmt.Printf("  - %s → %s\n", link.ShortenedLink, link.OriginalLink)
	}

	// What the answer's headers report: trace ID and the account's daily sending limit
	meta := response.ResponseMeta
	fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(meta.TraceID, "N/A"))

	if meta.DailyLimit != nil && meta.DailyRemaining != nil {
		fmt.Printf("📊 Daily limit: %d of %d messages left today\n", *meta.DailyRemaining, *meta.DailyLimit)
	}
}

// report prints an error of the client, the most specific types first.
func report(err error) {
	var validationError *calisero.ValidationError
	var dailyLimit *calisero.DailyLimitExceededError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &validationError):
		fmt.Printf("❌ Validation error: %s\n", validationError.Message)

		for field, messages := range validationError.ValidationErrors {
			fmt.Printf("  - %s: %s\n", field, strings.Join(messages, ", "))
		}
	case errors.As(err, &dailyLimit):
		fmt.Printf("❌ Daily sending limit reached: %s\n", dailyLimit.Message)
		fmt.Printf("⏰ Sending resumes at: %s\n", resetsAt(dailyLimit))
	case errors.As(err, &apiError):
		fmt.Printf("❌ API error: %s\n", apiError.Message)
		fmt.Printf("🔢 Status Code: %s\n", statusOf(apiError))
		fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))
	default:
		log.Fatal(err)
	}
}

func formatTime(t *time.Time, fallback string) string {
	if t == nil {
		return fallback
	}

	return t.Format(time.RFC3339)
}

func resetsAt(err *calisero.DailyLimitExceededError) string {
	if err.ResetsAt.IsZero() {
		return "midnight, Romania time"
	}

	return err.ResetsAt.Format(time.RFC3339)
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
