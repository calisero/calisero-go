// Retrieve the account: details, credit, status and daily sending limit.
//
//	CALISERO_API_KEY=... CALISERO_ACCOUNT_ID=... go run ./examples/account/get_account
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func main() {
	apiKey := cmp.Or(os.Getenv("CALISERO_API_KEY"), "your-api-key-here")
	// The account's ID, shown in the dashboard
	accountID := cmp.Or(os.Getenv("CALISERO_ACCOUNT_ID"), "9d0e0bd3-c805-4dd0-bafb-07c5ded0a8e1")

	if len(os.Args) > 1 {
		accountID = os.Args[1]
	}

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== Get Account Information ===\n\n")

	response, err := client.Accounts.Get(context.Background(), accountID)
	if err != nil {
		report(err, accountID)
		os.Exit(1)
	}

	account := response.Data

	fmt.Print("✅ Account information retrieved successfully!\n\n")

	fmt.Println("📋 Basic Information:")
	fmt.Printf("  🆔 Account ID: %s\n", account.ID)
	fmt.Printf("  🏷️ Account Code: %s\n", account.Code)
	fmt.Printf("  👤 Name: %s\n", account.Name)
	fmt.Printf("  📝 Description: %s\n", provided(account.Description))
	fmt.Printf("  📧 Email: %s\n", provided(account.Email))
	fmt.Printf("  📞 Phone: %s\n", provided(account.Phone))
	fmt.Printf("  👨‍💼 Contact Person: %s\n\n", provided(account.ContactPerson))

	fmt.Println("💰 Financial Information:")
	fmt.Printf("  💳 Current Credit: %.2f\n", account.Credit)
	fmt.Printf("  🏦 IBAN: %s\n", provided(account.IBAN))
	fmt.Printf("  📄 Fiscal Code: %s\n", provided(account.FiscalCode))
	fmt.Printf("  📋 Registry Number: %s\n\n", provided(account.RegistryNumber))

	fmt.Println("📍 Address Information:")
	fmt.Printf("  🏠 Address: %s\n", provided(account.Address))
	fmt.Printf("  🏙️ City: %s\n", provided(account.City))
	fmt.Printf("  🗺️ State: %s\n", provided(account.State))
	fmt.Printf("  🌍 Country: %s\n", provided(account.Country))
	fmt.Printf("  📮 Postal Code: %s\n\n", count(account.PostalCode, "Not provided"))

	fmt.Println("⚙️ Account Status:")
	fmt.Printf("  📊 Status: %s\n", account.Status)
	fmt.Printf("  🧪 Sandbox Mode: %s\n", yesNo(account.Sandbox))
	fmt.Printf("  ⏰ Created: %s\n\n", account.CreatedAt.Format(time.RFC3339))

	fmt.Println("📆 Daily Sending Limit:")
	fmt.Printf("  📊 Daily Limit: %s\n", count(account.DailyLimit, "None"))
	fmt.Printf("  📥 Left Today: %s\n", count(account.DailyRemaining, "Unlimited"))
	fmt.Printf("  📤 Sent Today: %d\n", account.SentToday)

	if account.Sandbox {
		fmt.Println("\n🧪 SANDBOX NOTICE:")
		fmt.Println("  This account is in sandbox mode: its messages are test messages,")
		fmt.Println("  marked delivered at once, never sent to a phone nor billed.")
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
		fmt.Println("💡 Please check your API key and ensure it's valid")
	case errors.As(err, &notFound):
		fmt.Printf("❌ Account not found: %s\n", notFound.Message)
		fmt.Printf("💡 Please check if the account ID '%s' is correct\n", accountID)
	case errors.As(err, &apiError):
		fmt.Printf("❌ API error: %s\n", apiError.Message)
		fmt.Printf("🔢 Status Code: %s\n", statusOf(apiError))
		fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))
	default:
		log.Fatal(err)
	}
}

func provided(value string) string {
	return cmp.Or(value, "Not provided")
}

func count(value *int, fallback string) string {
	if value == nil {
		return fallback
	}

	return strconv.Itoa(*value)
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
