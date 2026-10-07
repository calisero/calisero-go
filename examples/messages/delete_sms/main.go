// Cancel a scheduled message: schedule one for tomorrow, then delete it.
//
//	CALISERO_API_KEY=... CALISERO_TEST_PHONE=+40742123456 go run ./examples/messages/delete_sms
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
	recipient := cmp.Or(os.Getenv("CALISERO_TEST_PHONE"), "+40742***350")
	ctx := context.Background()

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Delete SMS Message ===\n\n")

	fmt.Println("1️⃣ Creating a scheduled message for deletion demo...")

	response, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
		Recipient:  recipient,
		Body:       "This message will be deleted before sending.",
		ScheduleAt: time.Now().Add(24 * time.Hour), // tomorrow
	})
	if err != nil {
		report(err)
		os.Exit(1)
	}

	message := response.Data

	fmt.Println("✅ Scheduled message created!")
	fmt.Printf("📨 Message ID: %s\n", message.ID)

	if message.ScheduledAt != nil {
		fmt.Printf("📅 Scheduled for: %s\n", message.ScheduledAt.Format(time.RFC3339))
	}

	fmt.Printf("📊 Status: %s\n\n", message.Status)

	fmt.Println("2️⃣ Deleting the scheduled message...")

	if err := client.Messages.Delete(ctx, message.ID); err != nil {
		report(err)
		os.Exit(1)
	}

	fmt.Println("✅ Message deleted successfully!")
	fmt.Printf("💡 The message '%s' has been cancelled and will not be sent\n", message.ID)

	fmt.Println("\n3️⃣ Verifying deletion...")

	_, err = client.Messages.Get(ctx, message.ID)

	var notFound *calisero.NotFoundError

	switch {
	case err == nil:
		fmt.Println("⚠️ Message still exists")
	case errors.As(err, &notFound):
		fmt.Println("✅ Confirmed: a cancelled message is no longer found")
	default:
		report(err)
		os.Exit(1)
	}
}

// report prints an error of the client, the most specific types first.
func report(err error) {
	var forbidden *calisero.ForbiddenError
	var notFound *calisero.NotFoundError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &forbidden):
		fmt.Printf("❌ Cannot delete message: %s\n", forbidden.Message)
		fmt.Println("💡 Only scheduled messages that have not been sent can be deleted")
		fmt.Println("💡 Messages already queued, sent or delivered cannot be deleted")
	case errors.As(err, &notFound):
		fmt.Printf("❌ Message not found: %s\n", notFound.Message)
		fmt.Println("💡 The message may have already been deleted or doesn't exist")
	case errors.As(err, &apiError):
		fmt.Printf("❌ API error: %s\n", apiError.Message)
		fmt.Printf("🔢 Status Code: %s\n", statusOf(apiError))
		fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))
	default:
		log.Fatal(err)
	}
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
