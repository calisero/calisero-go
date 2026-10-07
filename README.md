# Calisero Go SMS API Library

[![Go Reference](https://pkg.go.dev/badge/github.com/calisero/calisero-go.svg)](https://pkg.go.dev/github.com/calisero/calisero-go)
[![CI](https://img.shields.io/github/actions/workflow/status/calisero/calisero-go/ci.yml?branch=main&label=tests&style=flat-square)](https://github.com/calisero/calisero-go/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/codecov/c/github/calisero/calisero-go?style=flat-square)](https://codecov.io/gh/calisero/calisero-go)
[![Go Version](https://img.shields.io/github/go-mod/go-version/calisero/calisero-go?style=flat-square)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/calisero/calisero-go?style=flat-square)](https://goreportcard.com/report/github.com/calisero/calisero-go)
[![Release](https://img.shields.io/github/v/release/calisero/calisero-go?style=flat-square)](https://github.com/calisero/calisero-go/releases)
[![Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen.svg?style=flat-square)](go.mod)
[![Code style: gofmt](https://img.shields.io/badge/code%20style-gofmt-00add8.svg?style=flat-square)](https://pkg.go.dev/cmd/gofmt)
[![License](https://img.shields.io/github/license/calisero/calisero-go?style=flat-square)](LICENSE.md)

**Official Go library for the [Calisero](https://calisero.ro) transactional SMS API.**

Send SMS messages, verify phone numbers with one-time codes, manage opt-outs for GDPR compliance, receive delivery reports and monitor your account, with an idiomatic, context-aware library that has no dependencies.

## Features

- 🚀 **Simple & Intuitive**: `calisero.NewClient(apiKey)`, then `client.Messages.Create(ctx, ...)`
- 🔒 **Type Safe**: typed requests, resources and errors, statuses as typed constants
- 🌐 **Zero Dependencies**: built on the standard library's `net/http`, nothing else to download
- ⚡ **Context-aware**: every call takes a `context.Context` that cancels it, and a client is safe for concurrent use
- 📱 **Complete API Coverage**: messages, verifications (OTP), opt-outs and accounts, checked against the API's OpenAPI document
- 🔐 **OTP / 2FA**: start a verification and validate the code in two calls
- ✂️ **URL Shortening**: shorten the links of a message body and track their clicks
- 📊 **Limits & Tracing**: daily sending limit, request rate limit and the trace ID of every answer and error
- 📬 **Delivery Webhooks**: typed, validated parsing of delivery status callbacks, and a ready `http.Handler`
- 🛡️ **Error Handling**: an error type for every kind of failure, found with `errors.As`, with the API's message and the trace ID
- 🕐 **Romania Time Handled**: pass a `time.Time` to schedule a message, it is converted to the time zone the API expects
- 🧪 **Thoroughly Tested**: unit, integration (real HTTP) and contract (OpenAPI) tests, with the race detector, on Go 1.22 to 1.27, Linux, macOS and Windows; 100% of the statements covered
- 📖 **Rich Examples**: 19 runnable examples covering every operation, and documentation examples on pkg.go.dev
- 🐘 **Sibling Libraries**: [`calisero/calisero-php`](https://github.com/calisero/calisero-php), [`@calisero/calisero-node`](https://github.com/calisero/calisero-node-js) and [`calisero/laravel-sms`](https://github.com/calisero/laravel-sms)

## Requirements

- Go **1.22 or later** — tested on **1.22, 1.23, 1.24, 1.25, 1.26 and 1.27**
- A Calisero API key

The library carries no dependency outside the Go standard library. It is meant for server-side code: never ship your
API key in a client application.

## Installation

```bash
go get github.com/calisero/calisero-go
```

The package is named `calisero`:

```go
import calisero "github.com/calisero/calisero-go"
```

## Getting Your API Key

To use this library, you'll need an API key from your Calisero account:

1. **Log in to your Calisero account** at [https://calisero.ro](https://calisero.ro)
2. **Navigate to the dashboard** and go to the **"API Keys"** section
3. **Click "Add Key"** to create a new API key
4. **Configure your key:**
   - Give it a descriptive name (e.g., "Production App", "Development")
   - Set IP filters if needed for additional security
   - Choose a sandbox key to test without sending or paying for real messages
5. **Copy your API key** and store it securely

> **Security Note**: Never commit API keys to version control. Use environment variables or a secret manager.

## Quick Start

### Basic SMS Sending

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	calisero "github.com/calisero/calisero-go"
)

func main() {
	// Create the SMS client
	client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	// Send a simple SMS
	response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{
		Recipient: "+40742***350",
		Body:      "Hello from Calisero!",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Message sent! ID: %s\n", response.Data.ID)
	fmt.Printf("Status: %s\n", response.Data.Status)
}
```

### Environment Configuration

For production applications, store your API key securely:

```bash
# .env file, or your secret manager
CALISERO_API_KEY=your-api-key-here
```

`calisero.NewClient()` returns an error when the key is missing or empty, so a misconfigured environment fails at
startup rather than on the first message. Create the client once and share it: a `Client` is safe for concurrent use
by multiple goroutines.

## 🎯 Common Use Cases

### OTP / 2FA Authentication

The Verifications API generates the code, sends it by SMS and checks it, so your application never stores it:

```go
ctx := context.Background()

// 1. Send a code (6 characters, valid for 5 minutes by default)
_, err := client.Verifications.Create(ctx, &calisero.CreateVerificationRequest{Phone: "+40742***350", Brand: "YourApp"})
if err != nil {
	return err
}

// 2. Check the code the user typed
response, err := client.Verifications.Validate(ctx, &calisero.VerificationCheckRequest{Phone: "+40742***350", Code: userInput})

var validationError *calisero.ValidationError

switch {
case errors.As(err, &validationError):
	// A wrong or expired code, or too many attempts (5)
	fmt.Println(validationError.Message)
case err != nil:
	return err
default:
	fmt.Println("Verified at", response.Data.VerifiedAt)
}
```

To send a code you generate yourself, hide it in the API's answers and in the dashboard with `VisibleBody`:

```go
_, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
	Recipient:   userPhoneNumber,
	Body:        "Your verification code is: AC3-4F6. Valid for 5 minutes.",
	VisibleBody: "Your verification code is: ******. Valid for 5 minutes.",
	Sender:      "YourApp",
})
if err != nil {
	return err
}
```

### Order Notifications

```go
_, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
	Recipient:   customerPhone,
	Body:        fmt.Sprintf("Order #%s confirmed! Estimated delivery: %s", orderNumber, deliveryDate),
	CallbackURL: "https://yourstore.com/webhooks/sms/dlr",
	Sender:      "YourStore",
})
if err != nil {
	return err
}
```

### Marketing Campaigns with Opt-out Compliance

The API refuses to send to a number that opted out: the message fails validation (422) and is neither sent nor
billed.

```go
_, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
	Recipient: phoneNumber,
	Body:      "Special offer! Get 20% off with code SAVE20. Reply STOP to opt out.",
	Sender:    "YourBrand",
})

var validationError *calisero.ValidationError

switch {
case errors.As(err, &validationError) && len(validationError.ValidationErrors["recipient"]) > 0:
	fmt.Printf("Not sent to %s: %s\n", phoneNumber, strings.Join(validationError.ValidationErrors["recipient"], " "))
case err != nil:
	return err
}

// When a customer asks to stop receiving messages
_, err = client.OptOuts.Create(ctx, &calisero.CreateOptOutRequest{Phone: phoneNumber, Reason: "Replied STOP"})
```

### Advanced Message Creation

```go
response, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
	Recipient:   "+40742***350",
	Body:        "Your code is 123456. Manage alerts: https://yoursite.com/alerts",
	VisibleBody: "Your code is ******. Manage alerts: https://yoursite.com/alerts", // Shown in answers and the dashboard
	Validity:    24,                                                                // Validity period, in hours
	ScheduleAt:  time.Date(2026, 12, 25, 8, 0, 0, 0, time.UTC),                     // Sent at 10:00, Romania time
	CallbackURL: "https://yoursite.com/webhook",                                    // Delivery reports
	Sender:      "Calisero",                                                        // One of your approved sender IDs
	ShortenURLs: true,                                                              // Shorten the links of the body
})
if err != nil {
	return err
}

fmt.Println(response.Data.ScheduledAt)
```

`ScheduleAt` takes a `time.Time` in any time zone, converted to the `YYYY-MM-DD HH:MM:SS` Romania time the API reads,
summer and winter time handled; the zero time sends the message at once. A time already past today sends it at once
too; the API refuses an earlier day, and accepts up to 5 years ahead.

## Authentication

### Using an API Key

```go
client, err := calisero.NewClient("your-api-key-here")
```

### Custom Authentication Provider

Pass an `AuthProvider` with `WithAuthProvider` instead of the key: its `Token` is called before every request, with
the request's context, e.g. to read the key from a secret manager and rotate it without a restart.

```go
provider := calisero.AuthProviderFunc(func(ctx context.Context) (string, error) {
	return secretManager.Read(ctx, "calisero-api-key")
})

client, err := calisero.NewClient("", calisero.WithAuthProvider(provider))
```

An error the provider returns fails the call before anything is sent.

## 📚 Examples

This library includes runnable examples for every operation in the [`examples/`](examples/) directory:

### 📱 Message Examples
- **[`examples/messages/send_simple_sms`](examples/messages/send_simple_sms/main.go)** - Send a basic SMS message
- **[`examples/messages/send_advanced_sms`](examples/messages/send_advanced_sms/main.go)** - Scheduling, callbacks, custom sender and URL shortening
- **[`examples/messages/send_bulk_sms`](examples/messages/send_bulk_sms/main.go)** - Several recipients, stopping at the daily sending limit
- **[`examples/messages/get_sms`](examples/messages/get_sms/main.go)** - Retrieve a message and its delivery status
- **[`examples/messages/list_sms`](examples/messages/list_sms/main.go)** - List messages with pagination
- **[`examples/messages/delete_sms`](examples/messages/delete_sms/main.go)** - Cancel a scheduled message

### 🚫 Opt-out Examples (GDPR Compliance)
- **[`examples/optouts/create_optout`](examples/optouts/create_optout/main.go)** - Opt a phone number out
- **[`examples/optouts/get_optout`](examples/optouts/get_optout/main.go)** - Retrieve an opt-out
- **[`examples/optouts/list_optouts`](examples/optouts/list_optouts/main.go)** - List the opt-outs with pagination
- **[`examples/optouts/update_optout`](examples/optouts/update_optout/main.go)** - Update an opt-out's reason
- **[`examples/optouts/delete_optout`](examples/optouts/delete_optout/main.go)** - Remove an opt-out (re-enable SMS)

### ✅ Verification Examples (OTP)
- **[`examples/verifications/create_verification`](examples/verifications/create_verification/main.go)** - Start a verification (send a code)
- **[`examples/verifications/get_verification`](examples/verifications/get_verification/main.go)** - Retrieve a verification
- **[`examples/verifications/list_verifications`](examples/verifications/list_verifications/main.go)** - List verifications with pagination and a status filter
- **[`examples/verifications/validate_verification`](examples/verifications/validate_verification/main.go)** - Validate a code

### 👤 Account Examples
- **[`examples/account/get_account`](examples/account/get_account/main.go)** - Account details
- **[`examples/account/check_balance`](examples/account/check_balance/main.go)** - Credit and daily sending limit, with an analysis

### 📬 Webhook Examples
- **[`examples/webhooks/delivery_webhook`](examples/webhooks/delivery_webhook/main.go)** - Receive delivery status callbacks

### 🛡️ Error Handling
- **[`examples/error_handling_complete`](examples/error_handling_complete/main.go)** - Every error type and how to handle it

### 🚀 Running Examples

```bash
git clone https://github.com/calisero/calisero-go.git
cd calisero-go
CALISERO_API_KEY=your-api-key-here CALISERO_TEST_PHONE=+40742123456 go run ./examples/messages/send_simple_sms
```

> **Note**: Examples use masked phone numbers (`+40742***350`) unless `CALISERO_TEST_PHONE` is set. Use a sandbox API
> key to try them without sending real messages.

The documentation on [pkg.go.dev](https://pkg.go.dev/github.com/calisero/calisero-go) shows short examples too, next
to the functions they use.

## API Reference

Every call takes a `context.Context` first, which cancels it or bounds it with a deadline. A single resource comes as a
`*calisero.Response[T]` (`Data`, `ResponseMeta`), a list as a `*calisero.Page[T]` (`Data`, `Links`, `Meta`,
`ResponseMeta`).

### Messages

#### Send a Message

```go
response, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
	Recipient: "+40742***350",
	Body:      "Hello World!",
})
if err != nil {
	return err
}

message := response.Data
fmt.Println(message.ID)     // Message UUID
fmt.Println(message.Status) // scheduled, then sent, delivered or undelivered
fmt.Println(message.Parts)  // Number of SMS parts

// What the answer's headers report
fmt.Println(response.ResponseMeta.TraceID) // The request's trace ID
if remaining := response.ResponseMeta.DailyRemaining; remaining != nil {
	fmt.Println(*remaining) // Messages left today (nil: no daily limit)
}
```

| Field | Type | Description |
|---|---|---|
| `Recipient` | `string` | **Required.** Mobile number in E.164 format, e.g. `+40742123456` |
| `Body` | `string` | **Required.** The text, at most 1600 characters |
| `VisibleBody` | `string` | Shown instead of `Body` in the API's answers and the dashboard |
| `Validity` | `int` | Validity period, in hours |
| `ScheduleAt` | `time.Time` | When to send it, converted to Romania time; the zero time sends it at once |
| `CallbackURL` | `string` | Where Calisero posts the delivery status updates |
| `Sender` | `string` | One of your approved sender IDs; the default sender when empty |
| `ShortenURLs` | `bool` | Shorten the `http(s)://` links of the body |

The optional fields left at their zero value are not sent.

#### Shorten URLs

Set `ShortenURLs` to `true` and Calisero replaces every `http://` and `https://` link of the body with a short one
before sending, so the SMS gets shorter (and may take fewer parts). Each shortened link comes back with its click
statistics:

```go
response, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
	Recipient:   "+40742***350",
	Body:        "Your order has shipped! Track it: https://yourstore.com/orders/12345/tracking",
	ShortenURLs: true,
})
if err != nil {
	return err
}

for _, link := range response.Data.ShortenedURLs {
	fmt.Println(link.OriginalLink)  // https://yourstore.com/orders/12345/tracking
	fmt.Println(link.ShortenedLink) // https://calisero.ro/s/ghJKPV
}

// Later: how many times the links were opened
later, err := client.Messages.Get(ctx, response.Data.ID)
if err != nil {
	return err
}

for _, link := range later.Data.ShortenedURLs {
	fmt.Println(link.ClickCount, link.LastClick) // Number of clicks, last click or nil
}
```

#### Get Message Details

```go
response, err := client.Messages.Get(ctx, "message-uuid-here")
if err != nil {
	return err
}

message := response.Data
fmt.Println(message.Recipient, message.Body, message.Status)
fmt.Println(message.SentAt, message.DeliveredAt) // nil until sent, until delivered
```

#### List Messages

```go
page, err := client.Messages.List(ctx, nil) // Newest first, 50 a page
if err != nil {
	return err
}

for _, message := range page.Data {
	fmt.Printf("%s: %s\n", message.ID, message.Body)
}

fmt.Printf("Current page: %d\n", page.Meta.CurrentPage)

if page.HasNextPage() {
	next, err := client.Messages.List(ctx, &calisero.ListOptions{Page: page.Meta.CurrentPage + 1})
	if err != nil {
		return err
	}

	fmt.Printf("Next page: %d messages\n", len(next.Data))
}
```

#### Delete a Message

Only a message still scheduled can be cancelled:

```go
err := client.Messages.Delete(ctx, "message-uuid-here")

var forbidden *calisero.ForbiddenError

switch {
case errors.As(err, &forbidden):
	fmt.Println("Cannot delete: the message is already queued, sent or delivered")
case err != nil:
	return err
default:
	fmt.Println("Message cancelled")
}
```

### Verifications (OTP)

#### Create a Verification

```go
response, err := client.Verifications.Create(ctx, &calisero.CreateVerificationRequest{
	Phone: "+40742***350",
	Brand: "Calisero", // Named in the default message; required unless a template is given
	// Template:  "Your Calisero code is {code}", // Or a message of your own, holding {code}
	// ExpiresIn: 10,                             // Minutes the code stays valid, 1 to 10 (5 by default)
})
if err != nil {
	return err
}

verification := response.Data
fmt.Println(verification.ID, verification.Status) // unverified
fmt.Println(verification.ExpiresAt)

// The code's SMS counts towards the account's daily sending limit
if remaining := response.ResponseMeta.DailyRemaining; remaining != nil {
	fmt.Println(*remaining, "messages left today")
}
```

#### Get Verification Details

```go
response, err := client.Verifications.Get(ctx, "verification-uuid-here")
if err != nil {
	return err
}

verification := response.Data
fmt.Println(verification.Status)     // unverified or verified
fmt.Println(verification.VerifiedAt) // Timestamp or nil
fmt.Println(verification.Expired, verification.Attempts)
```

#### List Verifications

```go
// Optionally only the verified or the unverified ones
page, err := client.Verifications.List(ctx, &calisero.VerificationListOptions{
	Page:   1,
	Status: calisero.VerificationStatusVerified,
})
if err != nil {
	return err
}

for _, verification := range page.Data {
	fmt.Printf("%s | %s | %s\n", verification.ID, verification.Phone, verification.Status)
}
```

#### Validate a Verification Code (OTP)

The code is checked against the latest verification of the phone number, in any case:

```go
response, err := client.Verifications.Validate(ctx, &calisero.VerificationCheckRequest{
	Phone: "+40742***350",
	Code:  "ABC123",
})
if err != nil {
	return err
}

fmt.Println(response.Data.Status) // verified (checking a verified code again succeeds)
```

A wrong or expired code, or a sixth attempt, returns a `*calisero.ValidationError` whose `ValidationErrors["code"]`
says which; a phone number without a verification a `*calisero.NotFoundError`.

### Opt-outs

#### Create an Opt-out

```go
response, err := client.OptOuts.Create(ctx, &calisero.CreateOptOutRequest{
	Phone:  "+40742***350",
	Reason: "User requested opt-out via website",
})
if err != nil {
	return err
}

fmt.Printf("Opt-out created with ID: %s\n", response.Data.ID)
```

#### List Opt-outs

```go
page, err := client.OptOuts.List(ctx, nil)
if err != nil {
	return err
}

for _, optOut := range page.Data {
	fmt.Printf("%s: %s\n", optOut.Phone, cmp.Or(optOut.Reason, "no reason"))
}
```

#### Get, Update and Delete an Opt-out

```go
response, err := client.OptOuts.Get(ctx, "optout-uuid-here")
if err != nil {
	return err
}

_, err = client.OptOuts.Update(ctx, "optout-uuid-here", &calisero.UpdateOptOutRequest{
	Phone:  response.Data.Phone,
	Reason: "Updated reason",
})
if err != nil {
	return err
}

// The number can receive messages again
err = client.OptOuts.Delete(ctx, "optout-uuid-here")
```

### Accounts

```go
response, err := client.Accounts.Get(ctx, "account-uuid-here")
if err != nil {
	return err
}

account := response.Data
fmt.Printf("Account: %s\n", account.Name)
fmt.Printf("Credit: %.2f\n", account.Credit)
fmt.Printf("Status: %s\n", account.Status) // active, pending or disabled
fmt.Printf("Sandbox: %t\n", account.Sandbox)

// Daily sending limit (nil when no daily limit applies)
if account.DailyLimit != nil && account.DailyRemaining != nil {
	fmt.Printf("Daily limit: %d, left today: %d\n", *account.DailyLimit, *account.DailyRemaining)
}

fmt.Printf("Sent today: %d\n", account.SentToday)
```

### Delivery Status Webhooks

When a message has a `CallbackURL`, Calisero posts a JSON payload to it each time the message's status changes
(`sent`, `delivered` or `undelivered`). Answer with any 2xx status within 2 seconds: only a failed connection or a
slower answer is retried (at most 5 attempts in total), a non-2xx answer is not.

`calisero.DeliveryWebhookHandler` is a ready `http.Handler`: it reads and validates the payload, hands it to your
`OnMessage` as a typed `*calisero.DeliveryWebhookMessage`, and answers.

```go
mux := http.NewServeMux()
mux.Handle("POST /webhooks/calisero", &calisero.DeliveryWebhookHandler{
	// Optional: accept only callback URLs that carry ?token=<this secret>
	Token: os.Getenv("CALISERO_WEBHOOK_TOKEN"),

	OnMessage: func(r *http.Request, message *calisero.DeliveryWebhookMessage) error {
		// message.MessageID        The message's UUID
		// message.Status           calisero.DeliveryStatusSent, DeliveryStatusDelivered or DeliveryStatusUndelivered
		// message.DeliveredAt      Set for delivered
		// message.Price            Price charged for the message's parts
		// message.RemainingBalance Account balance after billing
		// message.DailyRemaining   Messages left today when it was sent (nil: no daily limit)

		// Keep it short (update a record, queue a job): the answer waits for it
		return orders.UpdateDelivery(r.Context(), message.MessageID, message.Status)
	},

	OnError: func(r *http.Request, err error) { log.Println(err) }, // A refused callback, or a failed OnMessage
})
```

Routers built on `net/http` mount it as is (chi: `router.Method(http.MethodPost, "/webhooks/calisero", handler)`);
with gin, `router.POST("/webhooks/calisero", gin.WrapH(handler))`; with echo,
`e.POST("/webhooks/calisero", echo.WrapHandler(handler))`.

Then send the URL with the secret as each message's callback URL:
`CallbackURL: "https://yourapp.com/webhooks/calisero?token=" + os.Getenv("CALISERO_WEBHOOK_TOKEN")`.

| Answer | When |
|---|---|
| `200 {"received":true}` | `OnMessage` handled the update |
| `400` / `413` | The payload is not a delivery status update / is larger than `MaxBodySize` (64 KiB) |
| `401` | `Token` is set and the request URL lacks it (compared in constant time) |
| `405` | Not a POST |
| `500` | `OnMessage` returned an error |

Calisero does not sign its callbacks, which is why the handler checks a secret token in the URL instead. Elsewhere,
read the payload yourself with `calisero.ParseDeliveryWebhook(body)`, or let your framework bind it:
`DeliveryWebhookMessage` implements `json.Unmarshaler` with the same validation. Both return an
`*calisero.InvalidWebhookPayloadError` for a malformed payload (and for the `[]` Calisero sends when the message was
cancelled meanwhile).

## Daily Sending Limit

Every account has its own daily sending limit: how many messages it can send in a day. A new account starts with a
default limit and keeps it until Calisero raises it on request.

- Every real message counts once, whatever its number of parts: the messages of `Messages.Create()` and the codes of
  `Verifications.Create()` alike. A scheduled message counts on the day it is created.
- Test messages (a sandbox account or a sandbox API key) never count and are never refused.
- The day ends at midnight, Romania time (Europe/Bucharest), whatever time zone you send from.

You can see the limit and what is left of it in three places:

```go
// 1. On the account: DailyLimit and DailyRemaining (nil when no daily limit applies), and SentToday
account, err := client.Accounts.Get(ctx, "account-uuid-here")
if err != nil {
	return err
}

if limit, remaining := account.Data.DailyLimit, account.Data.DailyRemaining; limit != nil && remaining != nil {
	fmt.Println(*limit, *remaining, account.Data.SentToday) // e.g. 1000 873 127
}

// 2. After every message or verification created (X-Daily-Limit / X-Daily-Remaining)
response, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{Recipient: "+40742***350", Body: "Hello!"})
if err != nil {
	return err
}

if remaining := response.ResponseMeta.DailyRemaining; remaining != nil {
	fmt.Println(*remaining) // e.g. 872, after this message
}

// 3. In every delivery status callback: message.DailyLimit, message.DailyRemaining, message.SentToday
```

Once the limit is reached, new messages and verifications are refused with a `*calisero.DailyLimitExceededError`
until midnight, Romania time. Nothing is sent and nothing is billed:

```go
_, err := client.Messages.Create(ctx, request)

var dailyLimit *calisero.DailyLimitExceededError
if errors.As(err, &dailyLimit) {
	fmt.Println(dailyLimit.Message)        // What happened and when the limit resets
	fmt.Println(dailyLimit.DailyLimit)     // e.g. 1000
	fmt.Println(dailyLimit.DailyRemaining) // 0
	fmt.Println(dailyLimit.ResetsAt)       // e.g. 2026-10-01 00:00:00 +0300 +0300
	fmt.Println(dailyLimit.RetryAfter)     // How long until the reset, e.g. 6h0m0s
}
```

To raise the limit, contact Calisero with your account and the daily volume you expect.

## Rate Limits & Trace ID

### Request Rate Limit

Separately from the daily sending limit, each API user may make **240 requests a minute**, on every endpoint. Above
that the API answers `429` and the library returns a `*calisero.RateLimitedError`:

```go
_, err := client.Messages.Create(ctx, request)

var dailyLimit *calisero.DailyLimitExceededError
var rateLimit *calisero.RateLimitedError

switch {
case errors.As(err, &dailyLimit):
	// The daily sending limit: wait until dailyLimit.ResetsAt
case errors.As(err, &rateLimit):
	// The request rate limit: wait a few seconds and retry
	time.Sleep(cmp.Or(rateLimit.RetryAfter, time.Second))
	fmt.Println(rateLimit.RateLimitLimit)     // 240
	fmt.Println(rateLimit.RateLimitRemaining) // 0
	fmt.Println(rateLimit.RateLimitReset)     // When requests are accepted again
}
```

A `*calisero.DailyLimitExceededError` is also a `*calisero.RateLimitedError`, so test for it first. After any
successful call, `ResponseMeta.RateLimitRemaining` tells how many requests are left in the current minute.

### Trace ID

Every API answer carries the request's trace ID. Log it: quoting it to support lets Calisero find the request at once,
and the dashboard shows everything that happened to it under **Developers → Debug**.

```go
response, err := client.Messages.Create(ctx, request)

var apiError *calisero.APIError

switch {
case errors.As(err, &apiError):
	// On error
	log.Printf("Calisero request %s failed: %s", apiError.TraceID, apiError.Message)
case err == nil:
	// On success
	log.Println(response.ResponseMeta.TraceID)
}
```

## Error Handling

Every error the API answers with is a `*calisero.APIError`, wrapped in the type that matches its status. Test for them
with `errors.As`, the most specific first:

```go
_, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{Recipient: "+40742***350", Body: "Test message"})

var (
	unauthorized    *calisero.UnauthorizedError
	validationError *calisero.ValidationError
	dailyLimit      *calisero.DailyLimitExceededError
	rateLimit       *calisero.RateLimitedError
	serverError     *calisero.ServerError
	transportError  *calisero.TransportError
	apiError        *calisero.APIError
)

switch {
case err == nil:
	fmt.Println("Message sent")
case errors.As(err, &unauthorized):
	fmt.Println("Invalid or revoked API key") // 401
case errors.As(err, &validationError):
	fmt.Println("Validation error:", validationError.Message) // 422
	for field, messages := range validationError.ValidationErrors {
		fmt.Printf("%s: %s\n", field, strings.Join(messages, ", "))
	}
case errors.As(err, &dailyLimit):
	fmt.Println("Daily limit reached. Sending resumes at:", dailyLimit.ResetsAt) // 429
case errors.As(err, &rateLimit):
	fmt.Println("Rate limited. Retry after:", rateLimit.RetryAfter) // 429
case errors.As(err, &serverError):
	fmt.Println("Server error:", serverError.Message) // 5xx
case errors.As(err, &transportError):
	fmt.Println("Network error:", transportError.Err) // No answer
case errors.As(err, &apiError):
	fmt.Printf("API error %d: %s (trace ID: %s)\n", apiError.StatusCode, apiError.Message, apiError.TraceID)
default:
	return err // An invalid argument, or the error of your auth provider
}
```

Every `*calisero.APIError` carries the API's own error message (`Message`), the HTTP status (`StatusCode`), the
request's trace ID (`TraceID`; `RequestID()` returns the same value), the decoded error body (`Details`) and the
error that caused it, if any (`Err`). Its `Error()` names the status and the trace ID:
`calisero: Resource not found! (status 404, trace ID 9b80eef1-49d4-4502-85a8-febb68cc11a7)`.

| Error | When |
|---|---|
| `*UnauthorizedError` | `401`: invalid, revoked or missing API key, or a request from an IP the key does not allow |
| `*ForbiddenError` | `403`: not allowed, e.g. deleting a message already sent, or another account's resource |
| `*NotFoundError` | `404`: the resource does not exist |
| `*ValidationError` | `422`: invalid request data; field errors in `ValidationErrors` |
| `*DailyLimitExceededError` | `429`: the account reached its daily sending limit |
| `*RateLimitedError` | `429`: more than 240 requests a minute |
| `*ServerError` | `500`, `502`, `503`, `504` |
| `*APIError` | Any other API error, or an answer the library cannot read; wrapped by all of the above |
| `*TransportError` | The request got no answer: network error, timeout or cancelled context (the original error is its `Err`) |

Arguments that cannot be sent at all (a nil request, an empty ID, a negative page) return a plain error before any
request is made; so does an error of your auth provider.

### Retries

The library never retries a request on its own. The API takes no idempotency key, so a message retried after a
timeout or a server error may already have been sent: check with `Messages.List()` before sending it again. The API
itself refuses (422, `duplicate_sms`) the same message to the same recipient sent again within 5 seconds.

## Advanced Configuration

### Client Options

```go
client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"),
	calisero.WithTimeout(10*time.Second),                    // How long to wait for each answer (30 seconds by default)
	calisero.WithBaseURL("https://rest.calisero.ro/api/v1"), // The default
	calisero.WithAppInfo("MyShop", "2.1.0"),                 // Named in the User-Agent header
)
```

Every request goes to `https://rest.calisero.ro/api/v1` with a 30-second timeout and the headers
`Authorization: Bearer <key>`, `Accept: application/json`, `Content-Type: application/json` and the
[`User-Agent`](#user-agent) below. The default HTTP client follows no redirect: the API answers with one only when
something is wrong, and the API key stays on its host.

### User-Agent

Every request names the library and its version, then the runtime, so Calisero can tell its requests apart and see
which version sent them:

```
User-Agent: Calisero-SMS-Go/1.0.0 (Go 1.27.1; linux amd64)
```

A request whose `User-Agent` starts with `Calisero-SMS-Go/` comes from this library. Give your application's name
with `WithAppInfo` and it follows, so the requests of each of your applications can be told apart too:

```go
client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"), calisero.WithAppInfo("MyShop", "2.1.0"))
if err != nil {
	log.Fatal(err)
}

fmt.Println(client.UserAgent())
// Calisero-SMS-Go/1.0.0 (Go 1.27.1; linux amd64) MyShop/2.1.0
```

Characters a header does not allow (spaces, diacritics…) become `-` in the name and the version.

### Through a Proxy

The default HTTP client uses Go's default transport, which reads the `HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY`
environment variables: a proxy set there needs no code. Otherwise, pass an `*http.Client` of your own:

```go
proxyURL, err := url.Parse("http://proxy.internal:3128")
if err != nil {
	log.Fatal(err)
}

client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"), calisero.WithHTTPClient(&http.Client{
	Timeout:   30 * time.Second,
	Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	// Answer a redirect as an error, as the default client does
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}))
```

A client given with `WithHTTPClient` is used as configured, its timeout and its redirect policy included.

### Cancelling a Request

```go
// Give up after 5 seconds, or when the incoming request is cancelled
ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
defer cancel()

page, err := client.Messages.List(ctx, nil)
```

A cancelled call returns a `*calisero.TransportError`: `errors.Is(err, context.Canceled)` or
`errors.Is(err, context.DeadlineExceeded)` tells why.

### Custom HTTP Client

`WithHTTPClient` takes any `calisero.HTTPDoer`: an `*http.Client`, or anything with its `Do` method. Wrap the
transport to log the traffic, add headers or plug in instrumentation:

```go
type loggingTransport struct {
	next http.RoundTripper
}

func (t loggingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	startedAt := time.Now()
	response, err := t.next.RoundTrip(request)

	if err == nil {
		log.Printf("%s %s → %d in %s", request.Method, request.URL, response.StatusCode, time.Since(startedAt))
	}

	return response, err
}

client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY"), calisero.WithHTTPClient(&http.Client{
	Timeout:       30 * time.Second,
	Transport:     loggingTransport{next: http.DefaultTransport},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}))
```

With OpenTelemetry, the transport is `otelhttp.NewTransport(http.DefaultTransport)`.

### Concurrency

A `Client` keeps no state between requests: create it once, at startup, and share it between goroutines. Each call's
answer, headers included, is in its own result.

## Types

Requests, resources, pages and errors are all typed. Statuses are typed string constants (`MessageStatus`,
`VerificationStatus`, `AccountStatus`, `DeliveryStatus`) that still hold the values a future API version may add.

What the API sends as `null` reads as the zero value: an empty string, or a `nil` pointer for the dates and the
numbers where zero means something (`DailyRemaining` is 0 once the limit is reached, `nil` when no limit applies).

The resources carry the API's field names in JSON (`created_at`, `daily_limit`…) and read the API's JSON with
`json.Unmarshal` the way the client does: a required field missing or malformed is an error naming it, and amounts
are read whether they come as numbers or as strings.

## Testing

```bash
go test ./...                   # Run the tests
go test -race ./...             # With the race detector, as CI runs them
go test -cover .                # With the coverage of the library
go vet ./...                    # Vet the library and the examples
golangci-lint run               # Lint (.golangci.yml)
```

[TESTING.md](TESTING.md) describes the suite: unit tests, integration tests over real HTTP against a local server,
contract tests against the API's OpenAPI document, and the examples of the documentation.

### Testing Your Implementation

Point the client at an `httptest.Server`: your tests then run without the network and see every request.

```go
func TestSendsTheOrderConfirmation(t *testing.T) {
	var received map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data": {"id": "9e2574e8-3615-4090-9b5a-0fc812079da8", "recipient": "+40742***350",
			"body": "Order #1001 confirmed!", "parts": 1, "created_at": "2025-02-06T10:18:43.000000Z", "status": "scheduled"}}`)
	}))
	defer server.Close()

	client, err := calisero.NewClient("test-key", calisero.WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	if err := sendOrderConfirmation(context.Background(), client, order); err != nil { // Your code
		t.Fatal(err)
	}

	if received["body"] != "Order #1001 confirmed!" {
		t.Errorf("sent %v", received)
	}
}
```

To test error handling, answer with the error the API sends, e.g. `w.WriteHeader(422)` and
`{"message": "The recipient field is required.", "errors": {"recipient": ["The recipient field is required."]}}`.

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to contribute to this project.

## Security

If you discover any security-related issues, please email support@calisero.ro instead of using the issue tracker.

## License

This library is open-sourced software licensed under the [MIT license](LICENSE.md).

## Support

- 📧 Email: support@calisero.ro
- 📖 Documentation: [https://docs.calisero.ro](https://docs.calisero.ro)
- 📦 Package documentation: [pkg.go.dev](https://pkg.go.dev/github.com/calisero/calisero-go)
- 🐛 Issues: [GitHub Issues](https://github.com/calisero/calisero-go/issues)

## Changelog

Please see [CHANGELOG.md](CHANGELOG.md) for more information on what has changed recently.
