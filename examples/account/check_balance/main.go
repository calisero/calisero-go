// Check the credit and the daily sending limit before a campaign.
//
//	CALISERO_API_KEY=... CALISERO_ACCOUNT_ID=... go run ./examples/account/check_balance
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"

	calisero "github.com/calisero/calisero-go"
)

// averageCostPerSms is an average price of an SMS part, to estimate what the
// credit covers: use your own pricing.
const averageCostPerSms = 0.05

func main() {
	apiKey := cmp.Or(os.Getenv("CALISERO_API_KEY"), "your-api-key-here")
	accountID := cmp.Or(os.Getenv("CALISERO_ACCOUNT_ID"), "9d0e0bd3-c805-4dd0-bafb-07c5ded0a8e1")

	if len(os.Args) > 1 {
		accountID = os.Args[1]
	}

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Account Balance Check ===\n\n")

	response, err := client.Accounts.Get(context.Background(), accountID)
	if err != nil {
		report(err, accountID)
		os.Exit(1)
	}

	account := response.Data
	credit := account.Credit

	fmt.Println("💰 Account Balance Information")
	fmt.Printf("📋 Account: %s (ID: %s)\n", account.Name, account.ID)
	fmt.Printf("💳 Current Credit: %.2f\n", credit)
	fmt.Printf("📊 Account Status: %s\n", account.Status)

	if account.Sandbox {
		fmt.Print("🧪 Environment: Sandbox\n\n")
	} else {
		fmt.Print("🧪 Environment: Production\n\n")
	}

	fmt.Println("📈 Balance Analysis:")

	switch {
	case credit > 100:
		fmt.Println("  🟢 Status: EXCELLENT - healthy for regular SMS operations")
	case credit > 50:
		fmt.Println("  🟡 Status: GOOD - consider topping up for high-volume campaigns")
	case credit > 10:
		fmt.Println("  🟠 Status: LOW - top up your account soon")
	default:
		fmt.Println("  🔴 Status: CRITICAL - top up immediately to maintain service")
	}

	estimatedMessages := int(math.Floor(credit / averageCostPerSms))

	fmt.Println("\n📱 Estimated Message Capacity:")
	fmt.Printf("  💸 Estimated cost per SMS: %.2f\n", averageCostPerSms)
	fmt.Printf("  📊 Approximate messages remaining: ~%d\n", estimatedMessages)

	fmt.Println("\n📆 Daily Sending Limit:")

	if account.DailyLimit == nil {
		fmt.Println("  ✅ No daily limit applies to this account")
	} else {
		dailyLimit := *account.DailyLimit

		fmt.Printf("  📊 Limit: %d messages a day (resets at midnight, Romania time)\n", dailyLimit)
		fmt.Printf("  📤 Sent today: %d\n", account.SentToday)

		if remaining := account.DailyRemaining; remaining != nil {
			fmt.Printf("  📥 Left today: %d\n", *remaining)

			switch {
			case *remaining == 0:
				fmt.Println("  🔴 The limit is reached: new messages are refused until midnight")
			case float64(*remaining) < float64(dailyLimit)*0.2:
				fmt.Println("  🟠 Less than 20% of the limit is left for today")
			}

			if *remaining < estimatedMessages {
				fmt.Println("  💡 The daily limit, not the credit, caps what you can send today")
			}
		}
	}

	fmt.Println("\n🔍 Service Readiness Check:")

	var issues []string

	if credit <= 0 {
		issues = append(issues, "Insufficient credit balance")
	}

	if account.DailyRemaining != nil && *account.DailyRemaining == 0 {
		issues = append(issues, "Daily sending limit reached until midnight, Romania time")
	}

	if account.Status != calisero.AccountStatusActive {
		issues = append(issues, fmt.Sprintf("Account status is %s, not active", account.Status))
	}

	if len(issues) == 0 {
		fmt.Println("  ✅ All systems ready for SMS operations")

		return
	}

	fmt.Println("  ❌ Issues detected:")

	for _, issue := range issues {
		fmt.Printf("    - %s\n", issue)
	}
}

// report prints an error of the client, the most specific types first.
func report(err error, accountID string) {
	var unauthorized *calisero.UnauthorizedError
	var notFound *calisero.NotFoundError
	var apiError *calisero.APIError

	switch {
	case errors.As(err, &unauthorized):
		fmt.Printf("❌ Authentication error: %s\n", unauthorized.Message)
		fmt.Println("💡 Please verify your API key is valid and not expired")
	case errors.As(err, &notFound):
		fmt.Printf("❌ Account not found: %s\n", notFound.Message)
		fmt.Printf("💡 Please verify the account ID '%s' is correct\n", accountID)
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
