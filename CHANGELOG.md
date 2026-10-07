# Changelog

All notable changes to `github.com/calisero/calisero-go` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-10-06

First release of the official Go library for the Calisero SMS API, covering version 1.0.14 of the API: the same API surface as `calisero/calisero-php` 2.3.1 and `@calisero/calisero-node` 1.0.1, with an idiomatic Go interface.

### Added
- **`Client`** with `calisero.NewClient(apiKey, options...)`: `WithBaseURL`, `WithTimeout` (30 s by default), `WithHTTPClient` (an `*http.Client` with a proxy, TLS settings or instrumentation of your own, or any `HTTPDoer`), `WithAppInfo` and `WithAuthProvider`. A `Client` is safe for concurrent use; every call takes a `context.Context`, which cancels it.
- **Messages** (`client.Messages`): `Create`, `Get`, `List` and `Delete`. `Create` accepts every option of the API, `ScheduleAt` included as a `time.Time`, converted to the `YYYY-MM-DD HH:MM:SS` Romania time the API expects (summer and winter time handled, with or without a time zone database on the machine).
- **URL shortening**: `ShortenURLs: true` and `Message.ShortenedURLs`, each `ShortenedLink` with its click statistics.
- **Opt-outs** (`client.OptOuts`): `Create`, `Get`, `List`, `Update` and `Delete`.
- **Verifications (OTP)** (`client.Verifications`): `Create`, `Get`, `List` (with a status filter) and `Validate`.
- **Accounts** (`client.Accounts`): `Get`, with the credit, the status and the daily sending limit (`DailyLimit`, `DailyRemaining`, `SentToday`).
- **Responses**: a single resource comes as a `Response[T]`, a list as a `Page[T]` (with `HasNextPage()`); both carry a `ResponseMeta` read from the answer's headers: the trace ID, the request rate limit and, for a created message or verification, what is left of the daily sending limit.
- **Errors**: `*APIError` and the types that wrap it, `*UnauthorizedError` (401), `*ForbiddenError` (403), `*NotFoundError` (404), `*ValidationError` (422, with `ValidationErrors`), `*RateLimitedError` (429, request rate limit), `*DailyLimitExceededError` (429, daily sending limit, wraps `*RateLimitedError`), `*ServerError` (5xx) and `*TransportError` (no answer: network error, timeout, cancelled context). Each carries the API's message, the HTTP status, the trace ID and the decoded error body, and is found with `errors.As`.
- **Delivery status webhooks**: `ParseDeliveryWebhook()` reads and validates the callback payload (also through `json.Unmarshal`, for the web frameworks that bind the body themselves), and `DeliveryWebhookHandler` answers the callbacks for `net/http`, with an optional secret `Token` checked in constant time.
- **User-Agent**: every request names the library, its version and the runtime, e.g. `Calisero-SMS-Go/1.0.0 (Go 1.27.1; linux amd64)`, so the platform can tell its requests apart. `WithAppInfo` (name, version) appends your application, e.g. `MyShop/2.1.0`; `client.UserAgent()` shows the header sent.
- **Strict reading of the answers**: a required field missing or malformed is an `*APIError` naming it ("Unexpected API response"), amounts are read whether the API sends them as numbers or as strings, and the resources read the API's JSON with `json.Unmarshal` the same way.
- **Zero dependencies**: only the Go standard library; Go 1.22 or later.
- Runnable examples for every operation in `examples/`, and documentation examples for pkg.go.dev.
- Tests: unit tests, integration tests over real HTTP against a local server, contract tests against the API's OpenAPI document and webhook handler tests over real HTTP, run with the race detector. CI runs them on Go 1.22, 1.23, 1.24, 1.25, 1.26 and 1.27, on Linux, macOS and Windows.

### Notes
- The library sends no `Idempotency-Key` header and never retries a request: the API takes no idempotency key, so a create retried after a timeout or a server error could send the message twice. The API refuses (422, `duplicate_sms`) the same message to the same recipient sent again within 5 seconds.
- The library does not embed the time zone database (`time/tzdata` would add 450 KB to every program that imports it): without one on the machine, Romania time follows the rules of the European Union, which Romania applies.
