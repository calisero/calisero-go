// Delete an opt-out: the number can receive messages again.
//
//	CALISERO_API_KEY=... go run ./examples/optouts/delete_optout <opt-out-id>
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"

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

	fmt.Print("=== Delete OptOut ===\n\n")

	fmt.Println("1️⃣ Checking current opt-out status...")

	response, err := client.OptOuts.Get(ctx, optOutID)
	if err != nil {
		report(err, optOutID)
		os.Exit(1)
	}

	optOut := response.Data

	fmt.Println("✅ Opt-out found:")
	fmt.Printf("   🆔 ID: %s\n", optOut.ID)
	fmt.Printf("   📱 Phone: %s\n", optOut.Phone)
	fmt.Printf("   📝 Reason: %s\n\n", cmp.Or(optOut.Reason, "Not specified"))

	fmt.Println("2️⃣ Removing opt-out (re-enabling SMS delivery)...")

	if err := client.OptOuts.Delete(ctx, optOutID); err != nil {
		report(err, optOutID)
		os.Exit(1)
	}

	fmt.Println("✅ Opt-out deleted successfully!")
	fmt.Printf("💡 %s can now receive SMS messages again\n", optOut.Phone)

	fmt.Println("\n3️⃣ Verifying deletion...")

	_, err = client.OptOuts.Get(ctx, optOutID)

	var notFound *calisero.NotFoundError

	switch {
	case err == nil:
		fmt.Println("⚠️ Opt-out still exists")
	case errors.As(err, &notFound):
		fmt.Println("✅ Confirmed: Opt-out has been removed")
	default:
		report(err, optOutID)
		os.Exit(1)
	}
}

// report prints an error of the client, the most specific types first.
func report(err error, optOutID string) {
	var notFound *calisero.NotFoundError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &notFound):
		fmt.Printf("❌ No opt-out found with ID: %s\n", optOutID)
		fmt.Println("💡 The phone number may already be able to receive messages")
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
