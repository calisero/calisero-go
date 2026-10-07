package calisero_test

import (
	"context"
	"errors"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func TestAPIErrorDescribesItselfWithItsStatusAndTraceID(t *testing.T) {
	cases := []struct {
		err  calisero.APIError
		want string
	}{
		{calisero.APIError{Message: "Resource not found!", StatusCode: 404, TraceID: traceID}, "calisero: Resource not found! (status 404, trace ID " + traceID + ")"},
		{calisero.APIError{Message: "HTTP error 502", StatusCode: 502}, "calisero: HTTP error 502 (status 502)"},
		{calisero.APIError{Message: "Refused", TraceID: traceID}, "calisero: Refused (trace ID " + traceID + ")"},
		{calisero.APIError{Message: "HTTP request failed: timeout"}, "calisero: HTTP request failed: timeout"},
	}

	for _, tc := range cases {
		equal(t, tc.err.Error(), tc.want, "Error()")
	}
}

func TestAPIErrorRequestIDIsTheTraceID(t *testing.T) {
	err := &calisero.APIError{TraceID: traceID}

	equal(t, err.RequestID(), traceID, "request ID")
}

func TestEveryErrorTypeIsAnAPIError(t *testing.T) {
	base := calisero.APIError{Message: "Refused", StatusCode: 400, TraceID: traceID}
	errs := []error{
		&calisero.UnauthorizedError{APIError: base},
		&calisero.ForbiddenError{APIError: base},
		&calisero.NotFoundError{APIError: base},
		&calisero.ValidationError{APIError: base},
		&calisero.RateLimitedError{APIError: base},
		&calisero.DailyLimitExceededError{RateLimitedError: calisero.RateLimitedError{APIError: base}},
		&calisero.ServerError{APIError: base},
		&calisero.TransportError{APIError: base},
	}

	for _, err := range errs {
		apiError := errorAs[*calisero.APIError](t, err)
		equal(t, apiError.Message, "Refused", "message")
		equal(t, err.Error(), "calisero: Refused (status 400, trace ID "+traceID+")", "Error()")
	}
}

func TestADailyLimitRefusalIsARateLimitRefusal(t *testing.T) {
	err := error(&calisero.DailyLimitExceededError{
		RateLimitedError: calisero.RateLimitedError{APIError: calisero.APIError{Message: "Daily limit reached."}, RetryAfter: time.Minute},
		DailyLimit:       1000,
	})

	equal(t, errorAs[*calisero.RateLimitedError](t, err).RetryAfter, time.Minute, "retry after")
	equal(t, errorAs[*calisero.DailyLimitExceededError](t, err).DailyLimit, 1000, "daily limit")
	equal(t, errorAs[*calisero.APIError](t, err).Message, "Daily limit reached.", "message")
}

func TestARateLimitRefusalIsNotADailyLimitRefusal(t *testing.T) {
	err := error(&calisero.RateLimitedError{APIError: calisero.APIError{Message: "Too Many Attempts."}})

	isNot[*calisero.DailyLimitExceededError](t, err)
}

func TestATransportErrorKeepsItsCause(t *testing.T) {
	err := error(&calisero.TransportError{APIError: calisero.APIError{Message: "HTTP request failed: context deadline exceeded", Err: context.DeadlineExceeded}})

	equal(t, errors.Is(err, context.DeadlineExceeded), true, "errors.Is finds the cause")
	equal(t, errors.Unwrap(errors.Unwrap(err)), context.DeadlineExceeded, "the chain")
}

func TestErrorsAreFoundWrapped(t *testing.T) {
	err := errors.Join(errors.New("sending the order confirmation"), &calisero.NotFoundError{APIError: calisero.APIError{Message: "Resource not found!", StatusCode: 404}})

	equal(t, errorAs[*calisero.NotFoundError](t, err).StatusCode, 404, "status")
	equal(t, errorAs[*calisero.APIError](t, err).Message, "Resource not found!", "message")
}

func TestInvalidWebhookPayloadErrorDescribesThePayload(t *testing.T) {
	cause := errors.New("unexpected end of JSON input")
	err := error(&calisero.InvalidWebhookPayloadError{Message: "invalid delivery webhook payload: unexpected end of JSON input", Err: cause})

	equal(t, err.Error(), "calisero: invalid delivery webhook payload: unexpected end of JSON input", "Error()")
	equal(t, errors.Is(err, cause), true, "the cause is kept")
	isNot[*calisero.APIError](t, err)
}
