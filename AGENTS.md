# AGENTS.md: calisero-go

Guidance for AI coding agents. Calisero is a Romanian transactional SMS API (base URL
`https://rest.calisero.ro/api/v1`, `Authorization: Bearer <API key>`). This module, `github.com/calisero/calisero-go`
(package `calisero`), is its official Go library: no dependencies, a `context.Context` on every call, typed errors.

- **Using this library**: you are adding Calisero to an application.
- **Working on this library**: you are changing this repository.

Every name below exists in this repository's source; `README.md` documents each call in full.

## Using this library

### Install and configure

```bash
go get github.com/calisero/calisero-go
```

The library reads no environment variable and no file: pass the API key to `NewClient`. By convention the
application reads it from `CALISERO_API_KEY` (the examples do). Never hard-code the key, never ship it to a client
application, use one key per environment.

```go
import calisero "github.com/calisero/calisero-go"

client, err := calisero.NewClient(os.Getenv("CALISERO_API_KEY")) // an empty key is an error
if err != nil {
	log.Fatal(err)
}
```

Create the client once and share it: it is safe for concurrent use. Options: `calisero.WithTimeout(d)` (30 s by
default), `calisero.WithBaseURL(url)`, `calisero.WithHTTPClient(doer)`, `calisero.WithAppInfo(name, version)`,
`calisero.WithAuthProvider(provider)` (a key read from a secret manager before each request).

### Send an SMS

```go
response, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
	Recipient: "+40712345678",                 // E.164
	Body:      "Your order #1001 has shipped.", // at most 1600 characters
	// Sender:      "MyShop",                    // only a sender approved for the account
	// ScheduleAt:  time.Now().Add(time.Hour),   // any time zone, converted to Romania time
	// CallbackURL: "https://example.com/webhooks/calisero?token=" + os.Getenv("CALISERO_WEBHOOK_TOKEN"),
	// ShortenURLs: true, VisibleBody: "...", Validity: 24 (hours)
})
if err != nil {
	return err
}

log.Printf("message %s %s, %d part(s), trace %s",
	response.Data.ID, response.Data.Status, response.Data.Parts, response.ResponseMeta.TraceID)
```

Keep `response.Data.ID`: it matches the delivery callbacks. `response.ResponseMeta.DailyRemaining` (`*int`, nil
without a daily limit) says how many messages the account can still send today.

### OTP: send and validate

Calisero generates the code (6 characters, letters and digits, case-insensitive), sends it and checks it: never
generate or store it yourself.

```go
_, err := client.Verifications.Create(ctx, &calisero.CreateVerificationRequest{
	Phone: "+40712345678",
	Brand: "MyShop", // or Template: "Your MyShop code is {code}"; ExpiresIn: 1 to 10 minutes (5 by default)
})
if err != nil {
	return err
}

response, err := client.Verifications.Validate(ctx, &calisero.VerificationCheckRequest{Phone: phone, Code: input})

var invalid *calisero.ValidationError
switch {
case errors.As(err, &invalid):
	// wrong or expired code, or too many attempts (422): invalid.Message says which
case err != nil:
	return err
default:
	// response.Data.Status == calisero.VerificationStatusVerified
}
```

### Read a message's status

```go
response, err := client.Messages.Get(ctx, messageID)
if err == nil && response.Data.Status == calisero.MessageStatusDelivered {
	// delivered; SentAt and DeliveredAt are *time.Time
}
```

`client.Messages.List(ctx, &calisero.ListOptions{Page: 2})` lists messages (50 a page, newest first);
`client.Messages.Delete(ctx, id)` cancels a message that is still scheduled. A message that stays `sent` was not
confirmed by the carrier, which does not mean it failed.

### Delivery webhook

Set `CallbackURL` on the message, then mount `calisero.DeliveryWebhookHandler`, a ready `http.Handler`:

```go
mux.Handle("POST /webhooks/calisero", &calisero.DeliveryWebhookHandler{
	Token: os.Getenv("CALISERO_WEBHOOK_TOKEN"), // the callback URL must carry ?token=<this>
	OnMessage: func(r *http.Request, m *calisero.DeliveryWebhookMessage) error {
		// m.MessageID, m.Status (DeliveryStatusSent, DeliveryStatusDelivered, DeliveryStatusUndelivered),
		// m.SentAt, m.DeliveredAt, m.Price, m.RemainingBalance, m.DailyRemaining
		return deliveries.Enqueue(r.Context(), m.MessageID, m.Status) // store or enqueue, then return
	},
	OnError: func(r *http.Request, err error) { log.Println(err) },
})
```

The handler answers before Calisero's 2-second deadline only if `OnMessage` is fast. An error from `OnMessage` is
answered with `500`, which Calisero does not retry: persist first, process later. Outside `net/http`, parse the body
with `calisero.ParseDeliveryWebhook(body)` (an `*calisero.InvalidWebhookPayloadError` for a malformed one).

### Errors and the trace ID

Every API error is an `*calisero.APIError` (`Message`, `StatusCode`, `TraceID`, `Details`) wrapped in the type of its
status. Test with `errors.As`, the most specific first:

| Type | When |
| --- | --- |
| `*calisero.UnauthorizedError` | 401: missing, invalid or revoked API key |
| `*calisero.ForbiddenError` | 403: not allowed, e.g. deleting a message that is no longer scheduled |
| `*calisero.NotFoundError` | 404: unknown ID |
| `*calisero.ValidationError` | 422: `ValidationErrors` maps each field to its messages |
| `*calisero.DailyLimitExceededError` | 429 with `daily_limit_exceeded`; also a `*RateLimitedError`, so test it first |
| `*calisero.RateLimitedError` | 429: request rate limit, wait `RetryAfter` |
| `*calisero.ServerError` | 500, 502, 503, 504 |
| `*calisero.TransportError` | no answer: network error, timeout, cancelled context (`StatusCode` 0) |

Log `apiErr.TraceID` with every failure and `response.ResponseMeta.TraceID` with every success. Every answer carries
it as `X-Trace-Id` (an error body also as `trace_id`); the dashboard's Developers → Debug page shows everything that
happened to that request.

### Tests

- Unit tests: point the client at an `httptest.Server` with `calisero.WithBaseURL(server.URL)` (see "Testing Your
  Implementation" in `README.md`).
- Staging and CI against the real API: use a sandbox API key. It goes through the same validation but sends nothing,
  bills nothing and never counts towards the daily limit; the message is `delivered` at once, callbacks still arrive,
  and a test OTP code shows in the dashboard.

### Rules that matter

- **Phone numbers** in E.164: `+40712345678`, not `0712345678`. Normalise user input before sending.
- **Length and cost**: one part holds 160 GSM-7 characters, or 70 when the text has Romanian diacritics (ă, â, î, ș,
  ț) or emoji (UCS-2). A longer text is split into parts, each billed as one SMS (`Parts` on the message); `Body`
  takes at most 1600 characters. When cost matters, a text without diacritics keeps 160-character parts: an option,
  not a rule.
- **Sender ID**: alphanumeric, at most 11 characters, approved for the account and written exactly as approved;
  otherwise 422 and nothing is sent. Leave `Sender` empty for the default sender. It cannot receive replies. Never
  take it from user input. OTP codes go out from the shared "Verify" sender and need no sender of your own.
- **Scheduling**: the API reads `schedule_at` as `YYYY-MM-DD HH:MM:SS`, Romania time (Europe/Bucharest);
  `ScheduleAt` converts any `time.Time`, and the zero time sends at once.
- **Webhooks**: answer any 2xx within 2 seconds, do the work afterwards. Only a timeout or a failed connection is
  retried (at most 5 attempts); a non-2xx answer is not. The same update can arrive twice and out of order: key your
  processing on `MessageID` + `Status`, and never move a message back from `delivered`. `CallbackURL` must be a public
  http(s) URL: private, loopback and link-local addresses are refused.
- **429 has two meanings**: the request rate limit (240 requests a minute; retry after `RetryAfter`) and the daily
  sending limit (`*DailyLimitExceededError`: nothing sent, retrying the same day will not help; it resets at
  midnight, Romania time, `ResetsAt`). Created messages and verifications report `X-Daily-Limit` /
  `X-Daily-Remaining` in `ResponseMeta`.
- **No idempotency key**: the library never retries, and you should not retry `Messages.Create` blindly after a
  timeout or a `*TransportError`: the SMS may have been sent and billed. Check `Messages.List` first.
- **OTP**: `Brand` or a `Template` holding `{code}`, neither with links. Each code is a billed SMS that counts towards
  the daily limit: rate-limit resends in your application.

## Working on this library

Read `CONTRIBUTING.md` (standards, release) and `TESTING.md` (the suite) before changing code.

- **Layout**: one resource and its service per file (`messages.go`, `verifications.go`, `optouts.go`, `accounts.go`),
  its tests next to it; HTTP in `http.go`, errors in `errors.go`, the webhook in `webhook.go`, Romania time in
  `schedule.go`; runnable programs in `examples/`.
- **OpenAPI**: `resources/openApi/api-v1.json` is the API's contract. Field names, types and status codes come from
  it, and `contract_test.go` checks the library against it.
- **Checks** (what CI runs):

  ```bash
  test -z "$(gofmt -l .)"
  go vet ./...
  golangci-lint run          # .golangci.yml
  go mod tidy -diff
  go test -race ./...        # CI fails below 98% statement coverage
  go build ./...             # the examples compile
  govulncheck ./...
  ```

  If a local `automation/` folder exists (it is not committed), `make -f automation/Makefile qa` runs the same in
  Docker.
- **Conventions**: Go 1.22 compatible (`go.mod` says `go 1.22.0`), standard library only, a `context.Context` first
  in every call, errors prefixed `calisero: ` and wrapped with `%w`, every exported identifier documented. No test
  may reach the network.
- **Changelog and version**: note changes under `[Unreleased]` in `CHANGELOG.md`; a release sets `Version` in
  `version.go`, which a test matches against the latest changelog version.
