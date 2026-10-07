// Opt a phone number out: the API then refuses to send it messages.
//
//	CALISERO_API_KEY=... CALISERO_TEST_PHONE=+40742123456 go run ./examples/optouts/create_optout
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

	fmt.Print("=== Create OptOut ===\n\n")

	response, err := client.OptOuts.Create(context.Background(), &calisero.CreateOptOutRequest{
		Phone:  phone,
		Reason: "Customer requested to stop receiving marketing messages",
	})
	if err != nil {
		report(err)
		os.Exit(1)
	}

	optOut := response.Data

	fmt.Println("✅ Opt-out created successfully!")
	fmt.Printf("🆔 OptOut ID: %s\n", optOut.ID)
	fmt.Printf("📱 Phone Number: %s\n", optOut.Phone)
	fmt.Printf("📝 Reason: %s\n", cmp.Or(optOut.Reason, "Not specified"))
	fmt.Printf("⏰ Created: %s\n", optOut.CreatedAt.Format(time.RFC3339))
	fmt.Printf("🔄 Updated: %s\n", optOut.UpdatedAt.Format(time.RFC3339))
	fmt.Println("\n💡 This phone number will no longer receive SMS messages")
}

// report prints an error of the client, the most specific types first.
func report(err error) {
	var validationError *calisero.ValidationError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &validationError):
		// e.g. a number already opted out
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
