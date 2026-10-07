package calisero_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func ExampleNewClient() {
	client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{
		Recipient: "+40742123456",
		Body:      "Hello from Calisero!",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response.Data.ID, response.Data.Status)
}

func ExampleNewClient_options() {
	client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"),
		calisero.WithTimeout(10*time.Second),          // 30 seconds by default
		calisero.WithAppInfo("MyShop", "2.1.0"),       // Named in the User-Agent header
		calisero.WithBaseURL(calisero.DefaultBaseURL), // The default
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(client.UserAgent()) // Calisero-SMS-Go/1.0.0 (Go 1.27.1; linux amd64) MyShop/2.1.0
}

func ExampleWithAuthProvider() {
	// Called before every request: the key can rotate without a restart
	provider := calisero.AuthProviderFunc(func(_ context.Context) (string, error) {
		return os.Getenv("CALISERO_API_KEY"), nil
	})

	client, err := calisero.NewClient("", calisero.WithAuthProvider(provider))
	if err != nil {
		log.Fatal(err)
	}

	_ = client
}

func ExampleMessageService_Create() {
	client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{
		Recipient:   "+40742123456",
		Body:        "Your code is 123456. Manage alerts: https://yoursite.com/alerts",
		VisibleBody: "Your code is ******. Manage alerts: https://yoursite.com/alerts", // Shown in answers and the dashboard
		Validity:    24,                                                                // Hours
		ScheduleAt:  time.Now().Add(time.Hour),                                         // Converted to Romania time
		CallbackURL: "https://yoursite.com/webhooks/calisero",                          // Delivery reports
		Sender:      "Calisero",                                                        // One of your approved sender IDs
		ShortenURLs: true,                                                              // Shorten the links of the body
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response.Data.ID, response.Data.Status)
	fmt.Println(response.ResponseMeta.TraceID)

	// What is left of the account's daily sending limit (nil: no daily limit)
	if remaining := response.ResponseMeta.DailyRemaining; remaining != nil {
		fmt.Println(*remaining, "messages left today")
	}
}

func ExampleMessageService_List() {
	client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	// Every page, newest first, 50 messages each
	for page := 1; ; page++ {
		messages, err := client.Messages.List(context.Background(), &calisero.ListOptions{Page: page})
		if err != nil {
			log.Fatal(err)
		}

		for _, message := range messages.Data {
			fmt.Println(message.ID, message.Status)
		}

		if !messages.HasNextPage() {
			break
		}
	}
}

func ExampleVerificationService_Validate() {
	client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// 1. Send a code (6 characters, valid for 5 minutes by default)
	_, err = client.Verifications.Create(ctx, &calisero.CreateVerificationRequest{Phone: "+40742123456", Brand: "YourApp"})
	if err != nil {
		log.Fatal(err)
	}

	// 2. Check the code the user typed
	userInput := "ABC123"

	response, err := client.Verifications.Validate(ctx, &calisero.VerificationCheckRequest{Phone: "+40742123456", Code: userInput})

	var validationError *calisero.ValidationError

	switch {
	case errors.As(err, &validationError):
		// A wrong or expired code, or too many attempts (5)
		fmt.Println(validationError.ValidationErrors["code"])
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Println("Verified at", response.Data.VerifiedAt)
	}
}

func ExampleDailyLimitExceededError() {
	client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: "+40742123456", Body: "Hello!"})

	// A *DailyLimitExceededError is also a *RateLimitedError: test for it first
	var dailyLimit *calisero.DailyLimitExceededError
	var rateLimit *calisero.RateLimitedError

	switch {
	case errors.As(err, &dailyLimit):
		fmt.Println("Nothing sent; the daily limit resets at", dailyLimit.ResetsAt)
	case errors.As(err, &rateLimit):
		fmt.Println("Too many requests; retry in", rateLimit.RetryAfter)
	case err != nil:
		log.Fatal(err)
	}
}

func ExampleAPIError() {
	client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.Messages.Get(context.Background(), "9e2574e8-3615-4090-9b5a-0fc812079da8")

	var apiError *calisero.APIError
	if errors.As(err, &apiError) {
		// Quote the trace ID to support, or look it up under Developers → Debug
		log.Printf("Calisero request %s failed with %d: %s", apiError.TraceID, apiError.StatusCode, apiError.Message)
	}
}

func ExampleParseDeliveryWebhook() {
	body := []byte(`{"messageId":"019961db-14a7-7348-963f-5a7a789a969f","recipient":"+40742***350","status":"delivered",
		"price":0.0378,"deliveredAt":"2025-09-19T12:02:53.000000Z","dailyLimit":1000,"dailyRemaining":588}`)

	message, err := calisero.ParseDeliveryWebhook(body)
	if err != nil {
		fmt.Println(err)

		return
	}

	fmt.Println(message.MessageID, message.Status)
	fmt.Println(message.DeliveredAt.Format(time.RFC3339))
	fmt.Printf("%.4f\n", message.Price)
	fmt.Println(*message.DailyRemaining, "of", *message.DailyLimit, "messages left today")

	// What Calisero sends for a message cancelled in the meantime
	_, err = calisero.ParseDeliveryWebhook([]byte(`[]`))
	fmt.Println(err)

	// Output:
	// 019961db-14a7-7348-963f-5a7a789a969f delivered
	// 2025-09-19T12:02:53Z
	// 0.0378
	// 588 of 1000 messages left today
	// calisero: invalid delivery webhook payload: expected an object, got an array
}

func ExampleDeliveryWebhookHandler() {
	http.Handle("POST /webhooks/calisero", &calisero.DeliveryWebhookHandler{
		// Accept only the callback URLs that carry ?token=<this secret>
		Token: os.Getenv("CALISERO_WEBHOOK_TOKEN"),

		// Keep it short (update a record, queue a job): the answer waits for it
		OnMessage: func(_ *http.Request, message *calisero.DeliveryWebhookMessage) error {
			log.Println(message.MessageID, message.Status, message.Price)

			return nil
		},

		OnError: func(_ *http.Request, err error) {
			log.Println(err)
		},
	})

	server := &http.Server{Addr: ":8080", ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}
