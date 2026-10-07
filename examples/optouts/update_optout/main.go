// Update the reason of an opt-out.
//
//	CALISERO_API_KEY=... go run ./examples/optouts/update_optout <opt-out-id>
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
	optOutID := "019999b5-5687-71cd-ae81-baf6eea9d86f"
	ctx := context.Background()

	if len(os.Args) > 1 {
		optOutID = os.Args[1]
	}

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Update OptOut ===\n\n")

	fmt.Println("1️⃣ Checking current opt-out status...")

	current, err := client.OptOuts.Get(ctx, optOutID)
	if err != nil {
		report(err, optOutID)
		os.Exit(1)
	}

	fmt.Println("✅ Current opt-out found:")
	fmt.Printf("   📱 Phone: %s\n", current.Data.Phone)
	fmt.Printf("   📝 Current reason: %s\n", cmp.Or(current.Data.Reason, "Not specified"))
	fmt.Printf("   ⏰ Created: %s\n", current.Data.CreatedAt.Format(time.RFC3339))
	fmt.Printf("   🔄 Last updated: %s\n\n", current.Data.UpdatedAt.Format(time.RFC3339))

	fmt.Println("2️⃣ Updating opt-out reason...")

	updated, err := client.OptOuts.Update(ctx, optOutID, &calisero.UpdateOptOutRequest{
		Phone:  current.Data.Phone,
		Reason: "Updated: Customer called support and confirmed they want to remain opted out",
	})
	if err != nil {
		report(err, optOutID)
		os.Exit(1)
	}

	fmt.Println("✅ Opt-out updated successfully!")
	fmt.Printf("🆔 OptOut ID: %s\n", updated.Data.ID)
	fmt.Printf("📱 Phone Number: %s\n", updated.Data.Phone)
	fmt.Printf("📝 Updated Reason: %s\n", updated.Data.Reason)
	fmt.Printf("🔄 Updated: %s\n", updated.Data.UpdatedAt.Format(time.RFC3339))
}

// report prints an error of the client, the most specific types first.
func report(err error, optOutID string) {
	var notFound *calisero.NotFoundError
	var validationError *calisero.ValidationError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &notFound):
		fmt.Printf("❌ No opt-out found for %s\n", optOutID)
		fmt.Println("💡 You need to create an opt-out first before updating it")
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
