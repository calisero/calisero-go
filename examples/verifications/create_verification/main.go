// Start a verification: Calisero generates a 6-character code and sends it by SMS.
//
//	CALISERO_API_KEY=... CALISERO_TEST_PHONE=+40742123456 go run ./examples/verifications/create_verification
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
	phone := cmp.Or(os.Getenv("CALISERO_TEST_PHONE"), "+40742***350")

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Create Verification (OTP) ===\n\n")

	response, err := client.Verifications.Create(context.Background(), &calisero.CreateVerificationRequest{
		Phone: phone,
		// The brand named in the default message; or pass a Template holding {code}
		Brand:     "Calisero",
		ExpiresIn: 5, // minutes, 1 to 10
	})
	if err != nil {
		report(err)
		os.Exit(1)
	}

	verification := response.Data

	fmt.Println("✅ Verification created successfully!")
	fmt.Printf("🆔 ID: %s\n", verification.ID)
	fmt.Printf("📱 Phone: %s\n", verification.Phone)
	fmt.Printf("🏷️ Brand: %s\n", cmp.Or(verification.Brand, "N/A"))
	fmt.Printf("📊 Status: %s\n", verification.Status)
	fmt.Printf("⏰ Expires at: %s\n", verification.ExpiresAt.Format(time.RFC3339))
	fmt.Printf("🧪 Attempts: %d\n", verification.Attempts)

	// The code's SMS counts towards the account's daily sending limit
	if remaining := response.ResponseMeta.DailyRemaining; remaining != nil {
		fmt.Printf("📊 Daily limit: %d messages left today\n", *remaining)
	}
}

// report prints an error of the client, the most specific types first.
func report(err error) {
	var dailyLimit *calisero.DailyLimitExceededError
	var validationError *calisero.ValidationError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &dailyLimit):
		fmt.Printf("❌ Daily sending limit reached, no code was sent: %s\n", dailyLimit.Message)

		if dailyLimit.ResetsAt.IsZero() {
			fmt.Println("⏰ Sending resumes at: midnight, Romania time")
		} else {
			fmt.Printf("⏰ Sending resumes at: %s\n", dailyLimit.ResetsAt.Format(time.RFC3339))
		}
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

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
