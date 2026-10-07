// List the verifications, newest first, optionally only the verified or the
// unverified ones.
//
//	CALISERO_API_KEY=... go run ./examples/verifications/list_verifications [page] [verified|unverified]
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	calisero "github.com/calisero/calisero-go"
)

func main() {
	apiKey := cmp.Or(os.Getenv("CALISERO_API_KEY"), "your-api-key-here")
	options := &calisero.VerificationListOptions{Page: 1}

	if len(os.Args) > 1 {
		page, err := strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatalf("the page must be a number: %v", err)
		}

		options.Page = page
	}

	if len(os.Args) > 2 {
		options.Status = calisero.VerificationStatus(os.Args[2])
	}

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== List Verifications ===\n\n")

	response, err := client.Verifications.List(context.Background(), options)
	if err != nil {
		var apiError *calisero.APIError
		if !errors.As(err, &apiError) {
			log.Fatal(err)
		}

		fmt.Printf("❌ API error: %s\n", apiError.Message)
		fmt.Printf("🔢 Status Code: %s\n", statusOf(apiError))
		fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))
		os.Exit(1)
	}

	verifications, meta := response.Data, response.Meta

	fmt.Println("✅ Verifications listed successfully!")
	fmt.Printf("📊 Total on page: %d\n", len(verifications))
	fmt.Printf("📄 Current page: %d\n", meta.CurrentPage)
	fmt.Printf("📄 Per page: %d\n", meta.PerPage)
	fmt.Printf("📄 From record: %s\n", orNA(meta.From))
	fmt.Printf("📄 To record: %s\n\n", orNA(meta.To))

	if len(verifications) > 0 {
		fmt.Println("📝 Recent verifications:")

		for index, verification := range verifications[:min(5, len(verifications))] {
			fmt.Printf("  %d. ✅ %s | %s | %s | attempts: %d | expired: %s\n",
				index+1, verification.ID, verification.Phone, verification.Status, verification.Attempts, yesNo(verification.Expired))
		}

		if len(verifications) > 5 {
			fmt.Printf("  ... and %d more verifications\n", len(verifications)-5)
		}
	} else {
		fmt.Println("📭 No verifications found")
	}

	if response.HasNextPage() {
		fmt.Printf("\n💡 Tip: run it with the page %d to fetch the next page.\n", meta.CurrentPage+1)
	}
}

func orNA(value *int) string {
	if value == nil {
		return "N/A"
	}

	return strconv.Itoa(*value)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
