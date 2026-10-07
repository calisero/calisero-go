// Receive delivery status callbacks.
//
// Calisero posts a JSON payload to a message's callback URL each time its status
// changes (sent, delivered or undelivered). Answer with any 2xx status within
// 2 seconds: only a failed connection or a slower answer is retried, at most
// 5 attempts in total; a non-2xx answer is not retried.
//
// Calisero does not sign its callbacks: add a secret token to the callback URL
// you send (https://yourapp.com/webhooks/calisero?token=...) and set it here as
// CALISERO_WEBHOOK_TOKEN, so that only your callback URLs are accepted.
//
//	CALISERO_WEBHOOK_TOKEN=s3cr3t go run ./examples/webhooks/delivery_webhook
//
//	curl -X POST 'http://localhost:8080/webhooks/calisero?token=s3cr3t' -H 'Content-Type: application/json' \
//	     -d '{"messageId":"019961db-14a7-7348-963f-5a7a789a969f","recipient":"+40742***350","status":"delivered","price":0.0378}'
package main

import (
	"cmp"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func main() {
	port, err := strconv.Atoi(cmp.Or(os.Getenv("PORT"), "8080"))
	if err != nil {
		log.Fatalf("PORT must be a port number: %v", err)
	}

	handler := &calisero.DeliveryWebhookHandler{
		Token: os.Getenv("CALISERO_WEBHOOK_TOKEN"),

		// Keep the work here short (update a record, queue a job): the answer waits for it
		OnMessage: func(_ *http.Request, message *calisero.DeliveryWebhookMessage) error {
			var line string

			switch message.Status {
			case calisero.DeliveryStatusDelivered:
				line = fmt.Sprintf("📬 Message %s delivered to %s at %s", message.MessageID, message.Recipient, formatTime(message.DeliveredAt))
			case calisero.DeliveryStatusUndelivered:
				line = fmt.Sprintf("❌ Message %s could not be delivered to %s", message.MessageID, message.Recipient)
			default:
				line = fmt.Sprintf("📤 Message %s sent to %s at %s", message.MessageID, message.Recipient, formatTime(message.SentAt))
			}

			line += fmt.Sprintf(" | price: %.4f", message.Price)

			if message.RemainingBalance != nil {
				line += fmt.Sprintf(" | balance: %.2f", *message.RemainingBalance)
			} else {
				line += " | balance: N/A"
			}

			// The account's daily sending limit as it stood when the callback was sent
			if message.DailyLimit != nil && message.DailyRemaining != nil {
				line += fmt.Sprintf(" | daily limit: %d of %d messages left today", *message.DailyRemaining, *message.DailyLimit)
			}

			log.Println(line)

			return nil
		},

		// A refused callback (bad token, malformed payload) or a failed OnMessage
		OnError: func(_ *http.Request, err error) {
			log.Println("⚠️", err)
		},
	}

	mux := http.NewServeMux()
	mux.Handle("/webhooks/calisero", handler)

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("👂 Listening for delivery status callbacks on http://localhost:%d/webhooks/calisero", port)
	log.Fatal(server.ListenAndServe())
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "N/A"
	}

	return t.Format(time.RFC3339)
}
