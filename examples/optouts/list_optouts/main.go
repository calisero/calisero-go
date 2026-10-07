// List the opt-outs, 50 a page.
//
//	CALISERO_API_KEY=... go run ./examples/optouts/list_optouts [page]
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
	page := 1

	if len(os.Args) > 1 {
		number, err := strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatalf("the page must be a number: %v", err)
		}

		page = number
	}

	client, err := calisero.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("=== List OptOuts ===\n\n")

	response, err := client.OptOuts.List(context.Background(), &calisero.ListOptions{Page: page})
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

	optOuts, meta, links := response.Data, response.Meta, response.Links

	fmt.Println("✅ OptOuts listed successfully!")
	fmt.Printf("📊 OptOuts on this page: %d\n", len(optOuts))
	fmt.Printf("📄 Current page: %d\n", meta.CurrentPage)
	fmt.Printf("📄 Per page: %d\n", meta.PerPage)
	fmt.Printf("📄 From record: %s\n", orNA(meta.From))
	fmt.Printf("📄 To record: %s\n\n", orNA(meta.To))

	if len(optOuts) > 0 {
		fmt.Println("🚫 Current opt-outs:")

		for index, optOut := range optOuts[:min(10, len(optOuts))] {
			reason := []rune(cmp.Or(optOut.Reason, "No reason provided"))
			if len(reason) > 40 {
				reason = append(reason[:40], []rune("...")...)
			}

			fmt.Printf("  %d. 📱 %s [ID: %s]\n", index+1, optOut.Phone, optOut.ID)
			fmt.Printf("     📝 %s\n", string(reason))
			fmt.Printf("     ⏰ %s\n\n", optOut.CreatedAt.Format(time.RFC3339))
		}

		if len(optOuts) > 10 {
			fmt.Printf("  ... and %d more opt-outs\n\n", len(optOuts)-10)
		}

		withReasons := 0

		for _, optOut := range optOuts {
			if optOut.Reason != "" {
				withReasons++
			}
		}

		fmt.Println("📊 Statistics for this page:")
		fmt.Printf("  - With reasons: %d\n", withReasons)
		fmt.Printf("  - Without reasons: %d\n", len(optOuts)-withReasons)
	} else {
		fmt.Println("✅ No opt-outs found - all numbers can receive messages")
	}

	fmt.Println("\n🔗 Pagination:")

	if links.Prev != "" {
		fmt.Println("  - Previous page: Available")
	} else {
		fmt.Println("  - Previous page: N/A")
	}

	if links.Next != "" {
		fmt.Printf("  - Next page: go run ./examples/optouts/list_optouts %d\n", page+1)
	} else {
		fmt.Println("  - Next page: N/A")
	}
}

func orNA(value *int) string {
	if value == nil {
		return "N/A"
	}

	return strconv.Itoa(*value)
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
