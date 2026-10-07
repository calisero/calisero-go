// List the messages, newest first, 50 a page.
//
//	CALISERO_API_KEY=... go run ./examples/messages/list_sms [page]
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

	fmt.Print("=== List SMS Messages ===\n\n")

	response, err := client.Messages.List(context.Background(), &calisero.ListOptions{Page: page})
	if err != nil {
		report(err)
		os.Exit(1)
	}

	messages, meta, links := response.Data, response.Meta, response.Links

	fmt.Println("✅ Messages listed successfully!")
	fmt.Printf("📊 Messages on this page: %d\n", len(messages))
	fmt.Printf("📄 Current page: %d\n", meta.CurrentPage)
	fmt.Printf("📄 Per page: %d\n", meta.PerPage)
	fmt.Printf("📄 From record: %s\n", orNA(meta.From))
	fmt.Printf("📄 To record: %s\n\n", orNA(meta.To))

	if len(messages) > 0 {
		fmt.Println("📝 Recent messages:")

		for index, message := range messages[:min(5, len(messages))] {
			body := []rune(message.Body)
			if len(body) > 30 {
				body = append(body[:30], []rune("...")...)
			}

			fmt.Printf("  %d. 📨 %s -> %s: %s [%s]\n", index+1, message.ID, message.Recipient, string(body), message.Status)
		}

		if len(messages) > 5 {
			fmt.Printf("  ... and %d more messages\n", len(messages)-5)
		}
	} else {
		fmt.Println("📭 No messages found")
	}

	fmt.Println("\n🔗 Pagination:")
	fmt.Printf("  - Previous page: %s\n", available(links.Prev))
	fmt.Printf("  - Next page: %s\n", available(links.Next))

	if response.HasNextPage() {
		fmt.Println("\n📄 Getting next page...")

		next, err := client.Messages.List(context.Background(), &calisero.ListOptions{Page: meta.CurrentPage + 1})
		if err != nil {
			report(err)
			os.Exit(1)
		}

		fmt.Printf("✅ Next page retrieved with %d messages\n", len(next.Data))
	}
}

// report prints an error of the client.
func report(err error) {
	var apiError *calisero.APIError

	if !errors.As(err, &apiError) {
		log.Fatal(err)
	}

	fmt.Printf("❌ API error: %s\n", apiError.Message)
	fmt.Printf("🔢 Status Code: %s\n", statusOf(apiError))
	fmt.Printf("🆔 Trace ID: %s\n", cmp.Or(apiError.TraceID, "N/A"))
}

func orNA(value *int) string {
	if value == nil {
		return "N/A"
	}

	return strconv.Itoa(*value)
}

func available(link string) string {
	if link == "" {
		return "N/A"
	}

	return "Available"
}

func statusOf(apiError *calisero.APIError) string {
	if apiError.StatusCode == 0 {
		return "N/A"
	}

	return fmt.Sprint(apiError.StatusCode)
}
