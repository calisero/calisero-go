// Send the same message to several recipients, one request each, stopping once
// the account's daily sending limit is used up.
//
//	CALISERO_API_KEY=... go run ./examples/messages/send_bulk_sms
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

// result is what happened to the message of one recipient.
type result struct {
	recipient string
	status    string
	messageID string
	parts     int
	err       string
}

func main() {
	apiKey := cmp.Or(os.Getenv("CALISERO_API_KEY"), "your-api-key-here")

	recipients := []string{"+40742***350", "+40742***351", "+40742***352", "+40742***353", "+40742***354"}
	body := "Important announcement: Our office will be closed tomorrow for maintenance. Thank you for your understanding."
	callbackURL := "https://yourapp.com/webhooks/bulk-sms"

	// One client for all the requests: it is safe for concurrent use too
	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	var results []result

	successCount, failureCount := 0, 0

	fmt.Print("=== Send Bulk SMS Messages ===\n\n")
	fmt.Printf("📱 Sending messages to %d recipients...\n\n", len(recipients))

send:
	for index, recipient := range recipients {
		fmt.Printf("Sending to %s... ", recipient)

		response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{
			Recipient:   recipient,
			Body:        body,
			Validity:    24,
			CallbackURL: callbackURL,
		})

		var dailyLimit *calisero.DailyLimitExceededError
		var validationError *calisero.ValidationError
		var apiError *calisero.APIError

		switch {
		case errors.As(err, &dailyLimit):
			// Nothing was sent or billed, and every further message would be refused too
			fmt.Println("❌ Daily sending limit reached")
			fmt.Printf("  💬 Error: %s\n\n", dailyLimit.Message)
			results = append(results, result{recipient: recipient, status: "daily_limit_exceeded", err: dailyLimit.Message})
			failureCount++

			break send
		case errors.As(err, &validationError):
			fmt.Println("❌ Validation error")
			fmt.Printf("  💬 Error: %s\n\n", validationError.Message)
			results = append(results, result{recipient: recipient, status: "validation_error", err: validationError.Message})
			failureCount++

			continue
		case errors.As(err, &apiError):
			fmt.Println("❌ API error")
			fmt.Printf("  💬 Error: %s\n", apiError.Message)
			fmt.Printf("  🔢 Status: %s\n", statusOf(apiError))
			fmt.Printf("  🆔 Trace ID: %s\n\n", cmp.Or(apiError.TraceID, "N/A"))
			results = append(results, result{recipient: recipient, status: "api_error", err: apiError.Message})
			failureCount++

			continue
		case err != nil:
			log.Fatal(err)
		}

		message := response.Data

		fmt.Println("✅ Success")
		fmt.Printf("  📨 Message ID: %s\n", message.ID)
		fmt.Printf("  📊 Status: %s\n", message.Status)

		// What is left of the account's daily sending limit (nil when it has none)
		dailyRemaining := response.ResponseMeta.DailyRemaining

		if dailyRemaining != nil {
			fmt.Printf("  📉 Daily limit: %d messages left today\n", *dailyRemaining)
		}

		fmt.Println()

		results = append(results, result{recipient: recipient, status: "success", messageID: message.ID, parts: message.Parts})
		successCount++

		// Every further message would be refused until midnight, Romania time
		if dailyRemaining != nil && *dailyRemaining == 0 {
			fmt.Print("⛔ Daily sending limit used up: the remaining recipients are skipped\n\n")

			break
		}

		// Stay well under the request rate limit (240 requests a minute)
		if index < len(recipients)-1 {
			time.Sleep(250 * time.Millisecond)
		}
	}

	fmt.Println("=== Bulk SMS Summary ===")
	fmt.Printf("✅ Successful: %d\n", successCount)
	fmt.Printf("❌ Failed: %d\n", failureCount)
	fmt.Printf("⏭️ Skipped: %d\n", len(recipients)-successCount-failureCount)
	fmt.Printf("📊 Total: %d\n", len(recipients))
	fmt.Printf("📈 Success Rate: %.2f%%\n\n", float64(successCount)/float64(len(recipients))*100)

	fmt.Println("📝 Detailed Results:")

	for _, outcome := range results {
		if outcome.status == "success" {
			fmt.Printf("  📱 %s: ✅ Sent (ID: %s, Parts: %d)\n", outcome.recipient, outcome.messageID, outcome.parts)
		} else {
			fmt.Printf("  📱 %s: ❌ %s - %s\n", outcome.recipient, outcome.status, outcome.err)
		}
	}

	if successCount > 0 {
		fmt.Printf("\n💡 Tip: Monitor the callback URL '%s' for delivery updates\n", callbackURL)
	}
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
