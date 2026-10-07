package calisero_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

// errorFor is the error a GET answered with answer returns.
func errorFor(t *testing.T, answer any) error {
	t.Helper()

	client, _ := clientAnswering(t, answer)

	_, err := client.Messages.Get(context.Background(), messageID)
	if err == nil {
		t.Fatal("the request was expected to fail")
	}

	return err
}

func TestRequestsCarryTheMethodURLAndHeaders(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, pageOf(nil, "messages", 2, false)))

	_, err := client.Messages.List(context.Background(), &calisero.ListOptions{Page: 2})
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.Method, http.MethodGet, "method")
	equal(t, request.URL.String(), "https://rest.calisero.ro/api/v1/messages?page=2", "URL")
	equal(t, request.Header.Get("Accept"), "application/json", "Accept")
	equal(t, request.Header.Get("Content-Type"), "application/json", "Content-Type")
	equal(t, request.Header.Get("Authorization"), "Bearer test-api-key", "Authorization")
	equal(t, request.Header.Get("User-Agent"), client.UserAgent(), "User-Agent")
	equal(t, request.Body, "", "body")
}

func TestRequestBodiesAreJSONWrittenAsIs(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(201, object{"data": messageData(nil)}))

	_, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{
		Recipient: phone,
		Body:      "Bună ziua & <bine> ați venit, Ștefan!",
	})
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.Method, http.MethodPost, "method")
	equal(t, request.Body, `{"recipient":"+40742***350","body":"Bună ziua & <bine> ați venit, Ștefan!"}`, "body")
}

func TestRequestsCarryNoIdempotencyKey(t *testing.T) {
	client, stub := clientAnswering(t,
		jsonResponse(201, object{"data": messageData(nil)}),
		jsonResponse(201, object{"data": verificationData(nil)}),
	)

	_, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: phone, Body: "Hello!"})
	noError(t, err)
	_, err = client.Verifications.Create(context.Background(), &calisero.CreateVerificationRequest{Phone: phone, Brand: "Calisero"})
	noError(t, err)

	for _, request := range stub.requests {
		if values := request.Header.Values("Idempotency-Key"); len(values) > 0 {
			t.Errorf("%s %s sent an Idempotency-Key: %v", request.Method, request.URL, values)
		}
	}
}

func TestAnAnswerThatIsNotJSONIsAnAPIError(t *testing.T) {
	err := errorFor(t, rawResponse(200, "<html>Maintenance</html>", "X-Trace-Id", traceID))

	apiError := errorAs[*calisero.APIError](t, err)
	equal(t, strings.HasPrefix(apiError.Message, "Failed to decode JSON response: "), true, "message")
	equal(t, apiError.StatusCode, 200, "status")
	equal(t, apiError.TraceID, traceID, "trace ID")

	var syntaxError *json.SyntaxError
	equal(t, errors.As(err, &syntaxError), true, "the JSON error is the cause")
}

func TestAnAnswerWithoutTheResourceIsAnAPIError(t *testing.T) {
	cases := map[string]struct {
		answer  *http.Response
		message string
	}{
		"an empty body": {
			rawResponse(200, "", "X-Trace-Id", traceID),
			"Unexpected API response: invalid message answer: expected an object, got null",
		},
		"a list": {
			jsonResponse(200, []any{}, "X-Trace-Id", traceID),
			"Unexpected API response: invalid message answer: expected an object, got an array",
		},
		"no data": {
			jsonResponse(200, object{"message": "OK"}, "X-Trace-Id", traceID),
			`Unexpected API response: invalid message answer: "data" must be an object, got nothing`,
		},
		"a malformed resource": {
			jsonResponse(200, object{"data": messageData(object{"id": nil})}, "X-Trace-Id", traceID),
			`Unexpected API response: invalid message: "id" must be a string, got null`,
		},
	}

	for name, tc := range cases {
		err := errorFor(t, tc.answer)

		apiError := errorAs[*calisero.APIError](t, err)
		equal(t, apiError.Message, tc.message, name+": message")
		equal(t, apiError.StatusCode, 200, name+": status")
		equal(t, apiError.TraceID, traceID, name+": trace ID")
		equal(t, apiError.Err != nil, true, name+": the cause is kept")
	}
}

func TestAnUnexpectedAnswerKeepsItsBody(t *testing.T) {
	err := errorFor(t, jsonResponse(200, object{"data": "not a message"}))

	apiError := errorAs[*calisero.APIError](t, err)
	deepEqual(t, apiError.Details, map[string]any{"data": "not a message"}, "details")
}

func TestErrorAnswersCarryTheMessageAndTheTraceIDOfTheBody(t *testing.T) {
	err := errorFor(t, jsonResponse(404, object{"message": "Resource not found!", "trace_id": traceID}))

	notFound := errorAs[*calisero.NotFoundError](t, err)
	equal(t, notFound.Message, "Resource not found!", "message")
	equal(t, notFound.StatusCode, 404, "status")
	equal(t, notFound.TraceID, traceID, "trace ID")
	equal(t, notFound.RequestID(), traceID, "request ID")
	deepEqual(t, notFound.Details, map[string]any{"message": "Resource not found!", "trace_id": traceID}, "details")
	equal(t, err.Error(), "calisero: Resource not found! (status 404, trace ID "+traceID+")", "Error()")
}

func TestTheTraceIDHeaderIsReadWhateverItsCaseBeforeTheBody(t *testing.T) {
	for _, name := range []string{"X-Trace-Id", "x-trace-id", "X-TRACE-ID"} {
		err := errorFor(t, jsonResponse(401, object{"message": "Unauthenticated.", "trace_id": "from-body"}, name, traceID))

		unauthorized := errorAs[*calisero.UnauthorizedError](t, err)
		equal(t, unauthorized.TraceID, traceID, name)
	}
}

func TestAnXRequestIDHeaderIsReadWithoutATraceID(t *testing.T) {
	err := errorFor(t, rawResponse(404, "", "X-Request-ID", "req_123"))

	equal(t, errorAs[*calisero.APIError](t, err).TraceID, "req_123", "trace ID")
}

func TestAnErrorWithoutAJSONBodyGetsAGenericMessage(t *testing.T) {
	err := errorFor(t, rawResponse(502, "<html>Bad Gateway</html>", "Content-Type", "text/html"))

	serverError := errorAs[*calisero.ServerError](t, err)
	equal(t, serverError.Message, "HTTP error 502", "message")
	equal(t, serverError.StatusCode, 502, "status")
	equal(t, serverError.TraceID, "", "trace ID")
	equal(t, serverError.Details == nil, true, "no details")
	equal(t, err.Error(), "calisero: HTTP error 502 (status 502)", "Error()")
}

func TestAnErrorBodyThatIsNotAnObjectIsIgnored(t *testing.T) {
	err := errorFor(t, rawResponse(500, `["Server Error"]`))

	serverError := errorAs[*calisero.ServerError](t, err)
	equal(t, serverError.Message, "HTTP error 500", "message")
	equal(t, serverError.Details == nil, true, "no details")
}

func TestTheMessageIsReadFromErrorMessageWithoutAMessage(t *testing.T) {
	err := errorFor(t, jsonResponse(400, object{"error": object{"message": "Bad request body"}}))

	equal(t, errorAs[*calisero.APIError](t, err).Message, "Bad request body", "message")
}

func TestAnEmptyMessageGetsAGenericOne(t *testing.T) {
	err := errorFor(t, jsonResponse(403, object{"message": "", "error": object{"message": ""}}))

	equal(t, errorAs[*calisero.ForbiddenError](t, err).Message, "HTTP error 403", "message")
}

func TestEveryErrorStatusMapsToItsErrorType(t *testing.T) {
	cases := []struct {
		status int
		is     func(error) bool
	}{
		{401, func(err error) bool { var target *calisero.UnauthorizedError; return errors.As(err, &target) }},
		{403, func(err error) bool { var target *calisero.ForbiddenError; return errors.As(err, &target) }},
		{404, func(err error) bool { var target *calisero.NotFoundError; return errors.As(err, &target) }},
		{422, func(err error) bool { var target *calisero.ValidationError; return errors.As(err, &target) }},
		{429, func(err error) bool { var target *calisero.RateLimitedError; return errors.As(err, &target) }},
		{500, func(err error) bool { var target *calisero.ServerError; return errors.As(err, &target) }},
		{502, func(err error) bool { var target *calisero.ServerError; return errors.As(err, &target) }},
		{503, func(err error) bool { var target *calisero.ServerError; return errors.As(err, &target) }},
		{504, func(err error) bool { var target *calisero.ServerError; return errors.As(err, &target) }},
	}

	for _, tc := range cases {
		err := errorFor(t, jsonResponse(tc.status, object{"message": "Status " + http.StatusText(tc.status)}))

		if !tc.is(err) {
			t.Errorf("%d: got %T", tc.status, err)
		}

		apiError := errorAs[*calisero.APIError](t, err)
		equal(t, apiError.StatusCode, tc.status, "status")
		equal(t, apiError.Message, "Status "+http.StatusText(tc.status), "message")
	}
}

func TestOtherStatusesArePlainAPIErrors(t *testing.T) {
	for _, status := range []int{302, 400, 405, 409, 418, 501} {
		err := errorFor(t, jsonResponse(status, object{"message": "Refused"}))

		if _, ok := err.(*calisero.APIError); !ok { //nolint:errorlint // A plain *APIError, none of the types that wrap it
			t.Errorf("%d: got %T, want a plain *APIError", status, err)
		}

		equal(t, errorAs[*calisero.APIError](t, err).StatusCode, status, "status")
	}
}

func TestValidationErrorsCarryTheFieldErrors(t *testing.T) {
	err := errorFor(t, jsonResponse(422, object{
		"message": "The recipient field is required. (and 1 more error)",
		"errors": object{
			"recipient": []any{"The recipient field is required."},
			"body":      []any{"The body field is required."},
		},
		"trace_id": traceID,
	}, "X-Trace-Id", traceID))

	validation := errorAs[*calisero.ValidationError](t, err)
	equal(t, validation.Message, "The recipient field is required. (and 1 more error)", "message")
	equal(t, validation.TraceID, traceID, "trace ID")
	deepEqual(t, validation.ValidationErrors, map[string][]string{
		"recipient": {"The recipient field is required."},
		"body":      {"The body field is required."},
	}, "field errors")
}

func TestValidationErrorsReadTheDuplicateMessageGuard(t *testing.T) {
	err := errorFor(t, jsonResponse(422, object{
		"message": "Duplicate SMS request detected.",
		"errors": object{
			"duplicate_sms": []any{"A similar SMS message has been recently submitted. Please wait a moment before trying again."},
		},
	}))

	validation := errorAs[*calisero.ValidationError](t, err)
	equal(t, len(validation.ValidationErrors), 1, "fields")
	equal(t, len(validation.ValidationErrors["duplicate_sms"]), 1, "duplicate_sms")
}

func TestValidationErrorsReadValidationErrorsWithoutErrors(t *testing.T) {
	err := errorFor(t, jsonResponse(422, object{"message": "Invalid.", "validation_errors": object{"code": []any{"Wrong code."}}}))

	deepEqual(t, errorAs[*calisero.ValidationError](t, err).ValidationErrors, map[string][]string{"code": {"Wrong code."}}, "field errors")
}

func TestValidationErrorsAreEmptyWhenTheBodyCarriesNone(t *testing.T) {
	// The API's own 422s carry field errors; a body without them must not pass for them
	err := errorFor(t, jsonResponse(422, object{"message": "The request could not be processed.", "trace_id": traceID}))

	validation := errorAs[*calisero.ValidationError](t, err)
	equal(t, len(validation.ValidationErrors), 0, "field errors")
	equal(t, validation.Details["trace_id"], any(traceID), "the body is in the details")
}

func TestValidationErrorsTurnASingleMessageIntoAListAndDropWhatIsNotText(t *testing.T) {
	err := errorFor(t, jsonResponse(422, object{
		"message": "Invalid.",
		"errors": object{
			"phone":  "Invalid phone.",
			"code":   []any{"Wrong.", 42, nil},
			"body":   42,
			"sender": object{"nested": true},
		},
	}))

	deepEqual(t, errorAs[*calisero.ValidationError](t, err).ValidationErrors, map[string][]string{
		"phone": {"Invalid phone."},
		"code":  {"Wrong."},
	}, "field errors")
}

func TestARefusalOfTheRequestRateLimit(t *testing.T) {
	// Lowercase names, as HTTP/2 sends them
	err := errorFor(t, jsonResponse(429, object{"message": "Too Many Attempts.", "trace_id": traceID},
		"retry-after", "17",
		"x-ratelimit-limit", "240",
		"x-ratelimit-remaining", "0",
		"x-ratelimit-reset", "1790000000",
		"x-trace-id", traceID,
	))

	isNot[*calisero.DailyLimitExceededError](t, err)

	limited := errorAs[*calisero.RateLimitedError](t, err)
	equal(t, limited.Message, "Too Many Attempts.", "message")
	equal(t, limited.StatusCode, 429, "status")
	equal(t, limited.TraceID, traceID, "trace ID")
	equal(t, limited.RetryAfter, 17*time.Second, "retry after")
	equal(t, limited.RateLimitLimit, 240, "rate limit")
	equal(t, limited.RateLimitRemaining, 0, "rate limit remaining")
	equal(t, limited.RateLimitReset, time.Unix(1790000000, 0).UTC(), "rate limit reset")
}

func TestARefusalOfTheDailySendingLimit(t *testing.T) {
	// Mixed-case names, as HTTP/1.1 sends them
	err := errorFor(t, jsonResponse(429, dailyLimitBody,
		"Retry-After", "21600",
		"X-Daily-Limit", "1000",
		"X-Daily-Remaining", "0",
		"X-RateLimit-Limit", "240",
		"X-RateLimit-Remaining", "238",
		"X-Trace-Id", traceID,
	))

	exceeded := errorAs[*calisero.DailyLimitExceededError](t, err)
	equal(t, exceeded.Message, dailyLimitBody["message"].(string), "message")
	equal(t, exceeded.StatusCode, 429, "status")
	equal(t, exceeded.TraceID, traceID, "trace ID")
	equal(t, exceeded.DailyLimit, 1000, "daily limit")
	equal(t, exceeded.DailyRemaining, 0, "daily remaining")
	equal(t, exceeded.ResetsAt.Equal(time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)), true, "resets at")
	equal(t, exceeded.ResetsAt.Format(time.RFC3339), "2026-10-01T00:00:00+03:00", "resets at, as sent")
	equal(t, exceeded.RetryAfter, 6*time.Hour, "retry after")
	equal(t, exceeded.RateLimitLimit, 240, "rate limit")
	equal(t, exceeded.RateLimitRemaining, 238, "rate limit remaining")
	equal(t, exceeded.RateLimitReset.IsZero(), true, "rate limit reset")
	equal(t, exceeded.Details["code"], any(calisero.DailyLimitExceededCode), "code")

	// Code written for the request rate limit keeps catching it
	limited := errorAs[*calisero.RateLimitedError](t, err)
	equal(t, limited.RetryAfter, 6*time.Hour, "retry after, as a RateLimitedError")
}

func TestADailyLimitRefusalFallsBackToTheHeaders(t *testing.T) {
	err := errorFor(t, jsonResponse(429, object{"message": "Daily limit reached.", "code": "daily_limit_exceeded"},
		"X-Daily-Limit", "500",
		"X-Daily-Remaining", "0",
	))

	exceeded := errorAs[*calisero.DailyLimitExceededError](t, err)
	equal(t, exceeded.DailyLimit, 500, "daily limit")
	equal(t, exceeded.DailyRemaining, 0, "daily remaining")
	equal(t, exceeded.ResetsAt.IsZero(), true, "resets at")
	equal(t, exceeded.RetryAfter, time.Duration(0), "retry after")
}

func TestRateLimitHeadersThatAreNotWholeSecondsAreIgnored(t *testing.T) {
	err := errorFor(t, jsonResponse(429, object{"message": "Too Many Attempts."},
		"Retry-After", "Wed, 21 Oct 2026 07:28:00 GMT",
		"X-RateLimit-Limit", "-1",
	))

	limited := errorAs[*calisero.RateLimitedError](t, err)
	equal(t, limited.RetryAfter, time.Duration(0), "retry after")
	equal(t, limited.RateLimitLimit, 0, "rate limit")
}

func TestARequestWithoutAnAnswerIsATransportError(t *testing.T) {
	cause := errors.New("connection reset by peer")
	err := errorFor(t, cause)

	transport := errorAs[*calisero.TransportError](t, err)
	equal(t, transport.Message, "HTTP request failed: connection reset by peer", "message")
	equal(t, transport.StatusCode, 0, "status")
	equal(t, transport.TraceID, "", "trace ID")
	equal(t, errors.Is(err, cause), true, "the cause is kept")
	equal(t, err.Error(), "calisero: HTTP request failed: connection reset by peer", "Error()")
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("unexpected EOF while reading the body")
}

func TestABodyThatCannotBeReadIsATransportError(t *testing.T) {
	answer := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(failingReader{})}
	err := errorFor(t, answer)

	equal(t, errorAs[*calisero.TransportError](t, err).Message, "HTTP request failed: unexpected EOF while reading the body", "message")
}

func TestAnAnswerWithoutABodyIsRead(t *testing.T) {
	client, _ := clientAnswering(t, &http.Response{StatusCode: http.StatusNoContent, Header: http.Header{}})

	noError(t, client.Messages.Delete(context.Background(), messageID))
}

func TestACancelledContextIsATransportError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := calisero.NewClient("test-api-key", calisero.WithBaseURL("http://127.0.0.1:9/api/v1"))
	noError(t, err)

	_, err = client.Messages.List(ctx, nil)

	isA[*calisero.TransportError](t, err)
	equal(t, errors.Is(err, context.Canceled), true, "the context's error is kept")
}

func TestANilContextIsRefused(t *testing.T) {
	client, stub := clientAnswering(t)

	//nolint:staticcheck // A nil context is what the test is about
	_, err := client.Messages.Get(nil, messageID)

	if err == nil || !strings.HasPrefix(err.Error(), "calisero: build the request: ") {
		t.Fatalf("got %v, want the request refused", err)
	}

	equal(t, stub.requestCount(), 0, "requests sent")
}
