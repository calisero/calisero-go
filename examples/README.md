# SMS API Examples

This directory contains runnable examples for every operation of the Calisero SMS API Go library. They are organized
by functionality and demonstrate proper error handling. Each one is a program of its own: run it with `go run`.

## Directory Structure

```
examples/
├── messages/                        # SMS message operations
│   ├── send_simple_sms/             # Send a basic SMS message
│   ├── send_advanced_sms/           # Every option: scheduling, callbacks, sender, URL shortening
│   ├── send_bulk_sms/               # Several recipients, within the daily limit
│   ├── get_sms/                     # Retrieve a message and its delivery status
│   ├── list_sms/                    # List messages with pagination
│   └── delete_sms/                  # Cancel a scheduled message
├── optouts/                         # Opt-out management
│   ├── create_optout/               # Opt a phone number out
│   ├── get_optout/                  # Retrieve an opt-out
│   ├── list_optouts/                # List the opt-outs with pagination
│   ├── update_optout/               # Update an opt-out's reason
│   └── delete_optout/               # Remove an opt-out (re-enable SMS)
├── verifications/                   # Phone verifications (OTP)
│   ├── create_verification/         # Start a verification (send a code)
│   ├── get_verification/            # Retrieve a verification
│   ├── list_verifications/          # List verifications with pagination and a status filter
│   └── validate_verification/       # Validate a code
├── account/                         # Account information
│   ├── get_account/                 # Account details
│   └── check_balance/               # Credit, daily limit and readiness
├── webhooks/                        # Callbacks Calisero sends to your application
│   └── delivery_webhook/            # Receive delivery status callbacks
└── error_handling_complete/         # Every error type and how to handle it
```

## Quick Start

1. **Set your credentials** in the environment:
   - `CALISERO_API_KEY`: your API key (a sandbox key sends no real message and costs nothing)
   - `CALISERO_TEST_PHONE`: the phone number to send to, in E.164 format (e.g. `+40742123456`)
   - `CALISERO_ACCOUNT_ID`: your account ID, for the account examples
   - `CALISERO_SENDER`: a sender ID approved for your account, written exactly as approved, for `send_advanced_sms`;
     the default sender when it is empty (an unknown or unapproved sender is refused with a 422)

2. **Run an example** from the repository's root: the examples use the library of the working copy.
   ```bash
   CALISERO_API_KEY=your-api-key-here go run ./examples/messages/send_simple_sms
   ```

   The examples that read a resource take its ID as an argument:
   ```bash
   go run ./examples/messages/get_sms 9e2574e8-3615-4090-9b5a-0fc812079da8
   ```

## Phone Number Format

Without `CALISERO_TEST_PHONE`, the examples use masked phone numbers like `+40742***350`, which the API refuses.
Phone numbers must be mobile numbers in international E.164 format, starting with `+` (e.g. `+40742123350`).

## Error Handling

Each example handles the errors it can meet, testing for the error types with `errors.As`, the most specific first:

- **Authentication errors** (401) - Invalid or revoked API key
- **Validation errors** (422) - Invalid request data, an opted-out recipient
- **Not found errors** (404) - Non-existent resources
- **Daily sending limit** (429) - The account sent its daily limit; nothing is sent until midnight, Romania time
- **Rate limiting** (429) - More than 240 requests a minute
- **Server errors** (5xx) - API server issues
- **Transport errors** - No answer: network error, timeout or cancelled context

An example that fails prints the error and exits with the status 1. For comprehensive error handling patterns, see
`error_handling_complete`.

## Webhook Example

`webhooks/delivery_webhook` starts a server that receives the delivery status callbacks, with the library's
`DeliveryWebhookHandler`:

```bash
CALISERO_WEBHOOK_TOKEN=s3cr3t go run ./examples/webhooks/delivery_webhook

curl -X POST 'http://localhost:8080/webhooks/calisero?token=s3cr3t' -H 'Content-Type: application/json' \
     -d '{"messageId":"019961db-14a7-7348-963f-5a7a789a969f","recipient":"+40742***350","status":"delivered","price":0.0378}'
```

The server listens on port 8080; set `PORT` to use another one.

## Best Practices Demonstrated

1. **Proper Error Handling** - Specific error types first, then `*calisero.APIError`
2. **Trace ID Logging** - Trace IDs for support and the dashboard's Developers → Debug page
3. **Rate Limiting** - A delay between the requests of a bulk send
4. **Daily Limit** - Stopping a bulk send once the daily sending limit is used up
5. **Secure Logging** - `VisibleBody` to keep codes out of the API's answers and the dashboard
6. **Pagination** - Reading the following pages
7. **Webhook Security** - A secret token in the callback URL, a server with timeouts

## Testing

CI compiles and vets every example, against the library of the same commit, so they cannot drift from its API:

```bash
go build ./examples/... && go vet ./examples/...
```

## Contributing

When adding new examples:

1. Follow the existing directory structure: one directory and one `main.go` per example
2. Include error handling for the errors the operation can meet
3. Read credentials from the environment, never hard-code them
4. Use masked phone numbers as defaults
5. Run `go vet ./examples/...` and `golangci-lint run`
6. Update this README and the main README
