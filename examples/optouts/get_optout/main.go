// Retrieve an opt-out by its ID.
//
//	CALISERO_API_KEY=... go run ./examples/optouts/get_optout <opt-out-id>
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
	optOutID := "019999b5-5687-71cd-ae81-baf6eea9d86f"

	if len(os.Args) > 1 {
		optOutID = os.Args[1]
	}

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Get OptOut ===\n\n")

	response, err := client.OptOuts.Get(context.Background(), optOutID)

	var notFound *calisero.NotFoundError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &notFound):
		fmt.Printf("❌ No opt-out found with ID: %s\n", optOutID)
		fmt.Println("💡 Make sure the opt-out ID exists")
		os.Exit(1)
	case errors.As(err, &apiError):
		fmt.Printf("❌ API error: %s\n", apiError.Message)
		fmt.Printf("🔢 Status Code: %s\n", statusOf(apiError))
		fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))
		os.Exit(1)
	case err != nil:
		log.Fatal(err)
	}

	optOut := response.Data

	fmt.Println("✅ Opt-out found!")
	fmt.Printf("🆔 OptOut ID: %s\n", optOut.ID)
	fmt.Printf("📱 Phone Number: %s\n", optOut.Phone)
	fmt.Printf("📝 Reason: %s\n", cmp.Or(optOut.Reason, "Not specified"))
	fmt.Printf("⏰ Created: %s\n", optOut.CreatedAt.Format(time.RFC3339))
	fmt.Printf("🔄 Last Updated: %s\n", optOut.UpdatedAt.Format(time.RFC3339))
	fmt.Println("\n🚫 This phone number is currently opted out from receiving SMS messages")
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
