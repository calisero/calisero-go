// Check the code a user received against the latest verification of their phone.
//
//	CALISERO_API_KEY=... CALISERO_TEST_PHONE=+40742123456 go run ./examples/verifications/validate_verification <code>
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
	code := "ABC123" // the 6-character code, in any case

	if len(os.Args) > 1 {
		code = os.Args[1]
	}

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Validate Verification (OTP) ===\n\n")

	response, err := client.Verifications.Validate(context.Background(), &calisero.VerificationCheckRequest{Phone: phone, Code: code})

	var validationError *calisero.ValidationError
	var notFound *calisero.NotFoundError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &validationError):
		// A wrong code, an expired one, or too many attempts (5)
		fmt.Printf("❌ Validation failed: %s\n", validationError.Message)

		for field, messages := range validationError.ValidationErrors {
			fmt.Printf("  - %s: %s\n", field, strings.Join(messages, ", "))
		}

		os.Exit(1)
	case errors.As(err, &notFound):
		fmt.Printf("❌ No verification was started for %s\n", phone)
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

	fmt.Println("✅ Verification validated successfully!")
	fmt.Printf("📱 Phone: %s\n", verification.Phone)
	fmt.Printf("📊 Status: %s\n", verification.Status)

	if verification.VerifiedAt != nil {
		fmt.Printf("✅ Verified at: %s\n", verification.VerifiedAt.Format(time.RFC3339))
	} else {
		fmt.Println("✅ Verified at: Just verified / already verified")
	}

	fmt.Printf("🧪 Attempts used: %d\n", verification.Attempts)
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
