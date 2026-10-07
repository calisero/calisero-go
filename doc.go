// Package calisero is the official Go library for the Calisero transactional SMS
// API (https://calisero.ro).
//
// Send SMS messages, verify phone numbers with one-time codes, manage opt-outs
// for GDPR compliance, receive delivery reports and monitor your account. The
// library has no dependency outside the standard library.
//
// # Getting Started
//
// Create a client with your API key, from the dashboard's API Keys section, then
// send a message:
//
//	client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"))
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	response, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
//		Recipient: "+40742123456",
//		Body:      "Hello from Calisero!",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Println(response.Data.ID, response.Data.Status)
//
// A Client is safe for concurrent use by multiple goroutines: create it once and
// share it. Every call takes a context, which cancels it.
//
// # Services
//
// The client groups the API's operations by resource:
//
//   - [Client.Messages] sends messages, now or later, lists them, reads their
//     delivery status and cancels the scheduled ones.
//   - [Client.Verifications] sends one-time codes (OTP, 2FA) and checks them.
//   - [Client.OptOuts] manages the phone numbers that asked to receive no message.
//   - [Client.Accounts] reads the account: credit, status and daily sending limit.
//
// A single resource comes as a [Response], a list as a [Page]. Both carry a
// [ResponseMeta], read from the answer's headers: the request's trace ID, the
// request rate limit and, for a created message or verification, what is left of
// the account's daily sending limit.
//
// # Errors
//
// Every error the API answers with is an [*APIError], wrapped in the type that
// matches its status: [*UnauthorizedError] (401), [*ForbiddenError] (403),
// [*NotFoundError] (404), [*ValidationError] (422), [*RateLimitedError] (429),
// [*DailyLimitExceededError] (429, the daily sending limit), [*ServerError] (5xx).
// A request that got no answer returns a [*TransportError]. Test for them with
// errors.As:
//
//	var validationError *calisero.ValidationError
//	if errors.As(err, &validationError) {
//		fmt.Println(validationError.ValidationErrors["recipient"])
//	}
//
// The client never retries a request: the API takes no idempotency key, so a
// message retried after a timeout or a server error may already have been sent.
//
// # Delivery Status Webhooks
//
// When a message has a callback URL, Calisero posts its delivery status updates
// to it. [ParseDeliveryWebhook] reads and validates a callback's body, and
// [DeliveryWebhookHandler] answers the callbacks for net/http:
//
//	http.Handle("POST /webhooks/calisero", &calisero.DeliveryWebhookHandler{
//		Token: os.Getenv("CALISERO_WEBHOOK_TOKEN"),
//		OnMessage: func(r *http.Request, message *calisero.DeliveryWebhookMessage) error {
//			log.Println(message.MessageID, message.Status)
//			return nil
//		},
//	})
package calisero
