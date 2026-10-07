// Retrieve a verification by its ID.
//
//	CALISERO_API_KEY=... go run ./examples/verifications/get_verification <verification-id>
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
	verificationID := "019a62f1-66b7-7387-a64f-2742c12a2860"

	if len(os.Args) > 1 {
		verificationID = os.Args[1]
	}

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Get Verification ===\n\n")

	response, err := client.Verifications.Get(context.Background(), verificationID)

	var notFound *calisero.NotFoundError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &notFound):
		fmt.Printf("❌ Verification not found: %s\n", notFound.Message)
		fmt.Printf("💡 Please check if the verification ID '%s' is correct\n", verificationID)
		os.Exit(1)
	case errors.As(err, &apiError):
		fmt.Printf("❌ API error: %s\n", apiError.Message)
		fmt.Printf("🔢 Status Code: %s\n", statusOf(apiError))
		fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))
		os.Exit(1)
	case err != nil:
		log.Fatal(err)
	}

	verification := response.Data

	fmt.Println("✅ Verification retrieved successfully!")
	fmt.Printf("🆔 ID: %s\n", verification.ID)
	fmt.Printf("📱 Phone: %s\n", verification.Phone)
	fmt.Printf("📊 Status: %s\n", verification.Status)

	if verification.VerifiedAt != nil {
		fmt.Printf("✅ Verified at: %s\n", verification.VerifiedAt.Format(time.RFC3339))
	} else {
		fmt.Println("✅ Verified at: Not verified yet")
	}

	fmt.Printf("⏰ Expires at: %s\n", verification.ExpiresAt.Format(time.RFC3339))
	fmt.Printf("🧪 Attempts: %d\n", verification.Attempts)
	fmt.Printf("⌛ Expired: %s\n", yesNo(verification.Expired))
}

func yesNo(value bool) string {
	if value {
		return "Yes"
	}

	return "No"
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
