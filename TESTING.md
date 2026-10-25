# Testing

The library's tests live next to its code, in the `*_test.go` files, and run with `go test`. None of them reaches the
network: the services run against a stub HTTP client, and the HTTP layer and the webhook handler against local HTTP
servers (`net/http/httptest`) on `127.0.0.1`. They use the standard library only, like the library itself.

As of 1.0.1 the suite has **178 tests in 14 files** (299 with their subtests), plus 10 documentation examples, and
covers 100% of the statements of the library. CI runs it with the race detector, and fails below 98% of the
statements.

## Running the Tests

```bash
# All tests
go test ./...

# With the race detector, as CI runs them
go test -race ./...

# With a coverage report: the total in the terminal, the details in coverage.html
go test -coverprofile=coverage.out . && go tool cover -func=coverage.out | tail -n 1
go tool cover -html=coverage.out -o coverage.html

# One test, or the tests whose names match a pattern
go test -run TestMessagesCreateSendsEveryOption .
go test -run 'DailyLimit' -v .

# The contract tests alone
go test -run '^TestContract' -v .
```

`go vet ./...` and `golangci-lint run` check the code, the tests and the examples; CONTRIBUTING.md lists everything CI
runs.

## Supported Versions

CI runs the suite on Go 1.22, 1.23, 1.24, 1.25, 1.26 and 1.27 on Linux, with the race detector, and on Go 1.27 on
macOS and Windows. Every job sets `GOTOOLCHAIN=local`, so that the go command runs the release of the job rather than
a newer one it would download.

- The library and its tests may use only what Go 1.22 offers (`go.mod` says `go 1.22.0`): no iterator functions
  (`maps.Keys`, `slices.Collect`), no `t.Context()`, no `omitzero`. The test matrix fails the build otherwise.
- The tests import `time/tzdata`, so that the time zone database is there on every machine; the library does not.

## Test Files

### Client and HTTP Layer

| File | Tests | Covers |
|---|---|---|
| `client_test.go` | 17 | `NewClient()`: the services, independent clients, the default base URL and timeout, the API key (refused when empty), an auth provider asked before each request, its errors and empty tokens; the `WithBaseURL`, `WithHTTPClient`, `WithTimeout` and `WithAppInfo` options and what they refuse; the User-Agent |
| `http_test.go` | 29 | The requests: method, URL, query, headers, JSON written as is (UTF-8, `&` and `<` unescaped), no Idempotency-Key; the answers: not JSON, empty, without the resource; every error status mapped to its error type; the message and the trace ID (header in any case, body, `X-Request-ID`); error bodies that are not JSON; 422 field errors; the request rate limit's 429 against the daily sending limit's; requests that got no answer, bodies that cannot be read, cancelled and nil contexts |
| `errors_test.go` | 8 | The error types: `Error()` with the status and the trace ID, `RequestID()`, every type found as an `*APIError` with `errors.As`, a daily limit refusal found as a rate limit refusal, causes found with `errors.Is`, errors wrapped by your code; `InvalidWebhookPayloadError` |
| `internal_test.go` | 14 | The unexported helpers: the Go release in the User-Agent (development builds included), the Romania time rules checked against the time zone database every quarter of an hour from 2020 to 2035, the conversion without a database, numbers, integers and date-times read from JSON, header lookups whatever their case, the reader keeping its first error, a body that cannot be encoded |

### Resources (`decode_test.go`)

| File | Tests | Covers |
|---|---|---|
| `decode_test.go` | 20 | Messages with every field and minimal ones, a status of a future API version, errors naming the field at fault (nested shortened links included), optional fields of the wrong type read as empty; accounts without their optional details, credits as decimal strings; opt-outs and verifications; `ResponseMeta` ignoring headers that are not counts; `json.Unmarshal` of the resources (the JSON `null`, malformed JSON) and their round trip |

### Services

Each test answers with a stub HTTP client (`stubDoer` in `helpers_test.go`) and checks the request the service sends
(method, path, query, body) and what it reads from the answer, headers included.

| File | Tests | Covers |
|---|---|---|
| `messages_test.go` | 21 | `Create()` with every option and with the minimal ones, `ScheduleAt` in Romania time (summer and winter time, the switches between them, midnight, other zones), shortened links, its `ResponseMeta`, the daily limit and validation errors; `Get()`, IDs escaped, empty IDs refused, unexpected answers; `List()` and its pages, malformed pages; `Delete()` and its answers |
| `optouts_test.go` | 10 | `Create()` with and without a reason, a duplicate; `Get()`, `List()`, `Update()`, `Delete()`; invalid arguments |
| `verifications_test.go` | 11 | `List()` with and without the status filter; `Create()` with a brand or a template, the daily limit; `Get()`; `Validate()` and each of its 422s, and its 404; invalid arguments |
| `accounts_test.go` | 5 | `Get()` for a typical account and a sandbox one without a daily limit; 401 and 403; an empty ID |

### Webhooks (`webhook_test.go`)

| File | Tests | Covers |
|---|---|---|
| `webhook_test.go` | 20 | `ParseDeliveryWebhook()`: full, minimal and older payloads, amounts as strings, optional fields of the wrong type, and every way a payload is refused; `json.Unmarshal` of a callback; `DeliveryWebhookHandler`: the answers (200, 400, 401, 405, 413, 500), the token, the body size limit, `OnMessage` and `OnError`, and a run over real HTTP behind a `net/http` router |

### Integration Tests (`integration_test.go`)

| File | Tests | Covers |
|---|---|---|
| `integration_test.go` | 12 | The client with its default HTTP client against a local server: headers (the User-Agent included) and UTF-8 bodies on the wire, answers and their headers, `[]` deletes, error mapping, HTML error pages, redirects not followed, timeouts, cancelled contexts, refused connections, and 20 concurrent requests each reading its own answer |

### Contract Tests (`contract_test.go`)

| File | Tests | Covers |
|---|---|---|
| `contract_test.go` | 8 (116 subtests) | The client against the API's OpenAPI document (`resources/openApi/api-v1.json`): every documented operation is implemented, and none other, with its method, path and query; every documented example answer is read; every documented field of every resource is read and written back; every documented request field can be sent, and nothing else; every documented error example maps to its error type with its message |

When the API changes, replace `resources/openApi/api-v1.json` with the new document: the contract tests then show
what the library lacks.

### Version and Documentation Examples

| File | Tests | Covers |
|---|---|---|
| `version_test.go` | 3 | `Version` matches the latest version of CHANGELOG.md, is a semantic version, and is named in the User-Agent |
| `example_test.go` | 10 examples | The examples pkg.go.dev shows next to the functions they use: all of them compile, and `ExampleParseDeliveryWebhook`, which needs no network, runs and checks its output |

The runnable programs of `examples/` have no tests of their own: `go vet ./...` and `go build ./...` compile and check
them, so they cannot drift from the library's API.

## Writing Tests

- Build fixtures from what the API really sends: the fixtures of `helpers_test.go` (`messageData()`, `pageOf()`…) and
  the OpenAPI examples.
- Answer with `jsonResponse()` through `clientAnswering()`, or with a local server (`startServer()` in
  `integration_test.go`) to test what goes over the wire.
- Cover the error paths as well as the successful ones: each error and the headers and body fields it reads.
- Keep the tests on the standard library: no assertion library, no dependency.
- Run the whole quality suite before opening a pull request (CONTRIBUTING.md).
