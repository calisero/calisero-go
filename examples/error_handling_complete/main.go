// Every error the library returns, and how to handle each.
//
//	CALISERO_API_KEY=... go run ./examples/error_handling_complete
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

const traceID = "9b80eef1-49d4-4502-85a8-febb68cc11a7"

// handleSmsAPIError describes an error and what to do about it. Test for the most
// specific types first: a *DailyLimitExceededError is also a *RateLimitedError,
// and every type is an *APIError.
func handleSmsAPIError(err error) {
	var apiError *calisero.APIError
	if !errors.As(err, &apiError) {
		fmt.Printf("   🔧 Not an API error: %v\n", err)

		return
	}

	fmt.Printf("   📝 Type: %T\n", err)
	fmt.Printf("   💬 Message: %s\n", apiError.Message)
	fmt.Printf("   🔢 HTTP Status: %s\n", statusOf(apiError))
	fmt.Printf("   🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))

	var (
		unauthorized    *calisero.UnauthorizedError
		validationError *calisero.ValidationError
		notFound        *calisero.NotFoundError
		forbidden       *calisero.ForbiddenError
		dailyLimit      *calisero.DailyLimitExceededError
		rateLimit       *calisero.RateLimitedError
		serverError     *calisero.ServerError
		transport       *calisero.TransportError
	)

	switch {
	case errors.As(err, &unauthorized):
		fmt.Println("   🔧 Action: Check the API key")
	case errors.As(err, &validationError):
		fmt.Println("   🔧 Action: Fix the request data")

		for field, messages := range validationError.ValidationErrors {
			fmt.Printf("     - %s: %s\n", field, strings.Join(messages, ", "))
		}
	case errors.As(err, &notFound):
		fmt.Println("   🔧 Action: Verify the resource exists and its ID is correct")
	case errors.As(err, &forbidden):
		fmt.Println("   🔧 Action: Check the permissions, or whether the operation is still possible")
	case errors.As(err, &dailyLimit):
		fmt.Println("   🔧 Action: Nothing was sent or billed: stop sending until the limit resets, or ask Calisero to raise it")
		fmt.Printf("   📊 Daily limit: %d messages, %d left today\n", dailyLimit.DailyLimit, dailyLimit.DailyRemaining)

		if dailyLimit.ResetsAt.IsZero() {
			fmt.Println("   ⏰ Resets at: midnight, Romania time")
		} else {
			fmt.Printf("   ⏰ Resets at: %s\n", dailyLimit.ResetsAt.Format(time.RFC3339))
		}
	case errors.As(err, &rateLimit):
		fmt.Printf("   🔧 Action: Wait %s, then retry\n", cmp.Or(rateLimit.RetryAfter, time.Second))
		fmt.Printf("   📊 Requests left this minute: %d of %d\n", rateLimit.RateLimitRemaining, rateLimit.RateLimitLimit)
	case errors.As(err, &serverError):
		fmt.Println("   🔧 Action: Contact support with the trace ID if it persists")
		fmt.Println("   ⚠️ Do not resend a message blindly: it may have been sent")
	case errors.As(err, &transport):
		fmt.Printf("   🔧 Action: Check the network (%v)\n", transport.Err)

		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Println("   ⏱️ The context's deadline passed")
		}

		fmt.Println("   ⚠️ Do not resend a message blindly: it may have been sent")
	default:
		fmt.Println("   🔧 Action: See the API documentation for this status")
	}
}

func main() {
	apiKey := cmp.Or(os.Getenv("CALISERO_API_KEY"), "your-api-key-here")
	ctx := context.Background()

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Comprehensive Error Handling Examples ===\n\n")

	fmt.Println("1️⃣ Authentication error (a real request with an invalid key)...")

	invalid, err := calisero.NewClient("invalid-api-key")
	if err != nil {
		log.Fatal(err)
	}

	_, err = invalid.Messages.Create(ctx, &calisero.CreateMessageRequest{Recipient: "+40742***350", Body: "Test message"})
	handleSmsAPIError(err)

	fmt.Println("\n2️⃣ Validation error (a real request with an invalid recipient)...")

	_, err = client.Messages.Create(ctx, &calisero.CreateMessageRequest{Recipient: "invalid-phone", Body: ""})
	handleSmsAPIError(err)

	fmt.Println("\n3️⃣ Not found error (a real request for a message that does not exist)...")

	_, err = client.Messages.Get(ctx, "00000000-0000-0000-0000-000000000000")
	handleSmsAPIError(err)

	fmt.Println("\n4️⃣ Request rate limit (simulated)...")
	handleSmsAPIError(&calisero.RateLimitedError{
		APIError:           calisero.APIError{Message: "Too Many Attempts.", StatusCode: 429, TraceID: traceID},
		RetryAfter:         17 * time.Second,
		RateLimitLimit:     240,
		RateLimitRemaining: 0,
		RateLimitReset:     time.Now().Add(17 * time.Second),
	})

	fmt.Println("\n5️⃣ Daily sending limit (simulated)...")
	handleSmsAPIError(&calisero.DailyLimitExceededError{
		RateLimitedError: calisero.RateLimitedError{
			APIError: calisero.APIError{
				Message:    "This account can send at most 1,000 messages a day. The limit resets at midnight, Romania time (2026-10-01T00:00:00+03:00). Contact us to raise it.",
				StatusCode: 429,
				TraceID:    traceID,
				Details:    map[string]any{"code": calisero.DailyLimitExceededCode},
			},
			RetryAfter: 6 * time.Hour,
		},
		DailyLimit:     1000,
		DailyRemaining: 0,
		ResetsAt:       time.Date(2026, 10, 1, 0, 0, 0, 0, time.FixedZone("EEST", 3*60*60)),
	})

	fmt.Println("\n6️⃣ Server error (simulated)...")
	handleSmsAPIError(&calisero.ServerError{APIError: calisero.APIError{Message: "Server Error", StatusCode: 500, TraceID: traceID}})

	fmt.Println("\n7️⃣ Transport error (a real request to a host that does not answer)...")

	unreachable, err := calisero.NewClient(apiKey, calisero.WithBaseURL("http://127.0.0.1:9/api/v1"), calisero.WithTimeout(2*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	_, err = unreachable.Messages.List(ctx, nil)
	handleSmsAPIError(err)

	fmt.Println("\n✨ Error handling examples completed!")
	fmt.Println("\n💡 Best Practices:")
	fmt.Println("  1. Test for the specific error types first, then *calisero.APIError")
	fmt.Println("  2. Log trace IDs for support (Developers → Debug in the dashboard)")
	fmt.Println("  3. Wait RetryAfter on a *RateLimitedError")
	fmt.Println("  4. Stop sending on a *DailyLimitExceededError until it resets")
	fmt.Println("  5. Never retry a create after a timeout or a 5xx without checking: it may have been sent")
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
