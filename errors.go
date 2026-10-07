package calisero

import (
	"strconv"
	"strings"
	"time"
)

// DailyLimitExceededCode is the code of the error body that tells a refusal of the
// account's daily sending limit from a refusal of the request rate limit.
const DailyLimitExceededCode = "daily_limit_exceeded"

// APIError is an error the Calisero API answered with, or the error of a request
// that got no answer (a *TransportError). Every error a call returns once its
// request was sent is one, wrapped in the type that matches the answer's status:
// test for them with errors.As, the specific types first.
type APIError struct {
	// Message is the API's own description of the error, e.g. "Resource not
	// found!"; "HTTP error 404" when the answer carried none.
	Message string

	// StatusCode is the HTTP status of the answer; 0 when the request got no answer.
	StatusCode int

	// TraceID is the request's trace ID, from the X-Trace-Id header or the error
	// body's trace_id: quote it to Calisero support, or look the request up in the
	// dashboard under Developers → Debug. Empty when the answer carried none.
	TraceID string

	// Details is the decoded JSON error body; nil when the answer had none.
	Details map[string]any

	// Err is the error that caused this one: the network error of a
	// *TransportError, the decoding error of an answer that could not be read.
	Err error
}

// Error describes the error with its status and trace ID, e.g. "calisero:
// Resource not found! (status 404, trace ID 9b80eef1-49d4-4502-85a8-febb68cc11a7)".
func (e *APIError) Error() string {
	var description strings.Builder

	description.WriteString("calisero: ")
	description.WriteString(e.Message)

	switch {
	case e.StatusCode != 0 && e.TraceID != "":
		description.WriteString(" (status " + strconv.Itoa(e.StatusCode) + ", trace ID " + e.TraceID + ")")
	case e.StatusCode != 0:
		description.WriteString(" (status " + strconv.Itoa(e.StatusCode) + ")")
	case e.TraceID != "":
		description.WriteString(" (trace ID " + e.TraceID + ")")
	}

	return description.String()
}

// Unwrap returns the error that caused this one, if any.
func (e *APIError) Unwrap() error {
	return e.Err
}

// RequestID is the request's trace ID, the same as TraceID.
func (e *APIError) RequestID() string {
	return e.TraceID
}

// UnauthorizedError is returned when authentication fails (401): the API key is
// missing, invalid or revoked, or the request comes from an IP address the key
// does not allow.
type UnauthorizedError struct {
	APIError
}

// Unwrap returns the embedded *APIError, so errors.As finds it.
func (e *UnauthorizedError) Unwrap() error {
	return &e.APIError
}

// ForbiddenError is returned when access is forbidden (403), e.g. deleting a
// message already sent, or reading another account's resource.
type ForbiddenError struct {
	APIError
}

// Unwrap returns the embedded *APIError, so errors.As finds it.
func (e *ForbiddenError) Unwrap() error {
	return &e.APIError
}

// NotFoundError is returned when a resource is not found (404).
type NotFoundError struct {
	APIError
}

// Unwrap returns the embedded *APIError, so errors.As finds it.
func (e *NotFoundError) Unwrap() error {
	return &e.APIError
}

// ValidationError is returned for validation errors (422): the request's data was
// refused, e.g. an invalid or opted-out recipient, or a wrong verification code.
type ValidationError struct {
	APIError

	// ValidationErrors are the field errors, field name → messages, e.g.
	// {"recipient": ["The recipient field is required."]}; empty when the answer
	// carried none.
	ValidationErrors map[string][]string
}

// Unwrap returns the embedded *APIError, so errors.As finds it.
func (e *ValidationError) Unwrap() error {
	return &e.APIError
}

// RateLimitedError is returned when the request rate limit is exceeded (429):
// each API user may make 240 requests a minute. A refusal of the account's daily
// sending limit is a *DailyLimitExceededError, which wraps this type, so test for
// that one first.
type RateLimitedError struct {
	APIError

	// RetryAfter is how long to wait before sending again (the Retry-After
	// header); 0 when the answer did not say it in seconds.
	RetryAfter time.Duration

	// RateLimitLimit is how many requests the API user may make a minute (the
	// X-RateLimit-Limit header); 0 when the answer did not say.
	RateLimitLimit int

	// RateLimitRemaining is how many requests the API user may still make this
	// minute (the X-RateLimit-Remaining header): 0 when the request rate limit
	// refused the request.
	RateLimitRemaining int

	// RateLimitReset is when requests are accepted again (the X-RateLimit-Reset
	// header, sent only when the request rate limit refused the request); the zero
	// time when the answer did not say.
	RateLimitReset time.Time
}

// Unwrap returns the embedded *APIError, so errors.As finds it.
func (e *RateLimitedError) Unwrap() error {
	return &e.APIError
}

// DailyLimitExceededError is returned when the account reached its daily sending
// limit (429 with the code daily_limit_exceeded): nothing was sent and nothing
// was billed.
//
// The limit resets at midnight, Romania time (ResetsAt); test messages never
// count towards it. It wraps a *RateLimitedError, so code that tests for that
// type keeps catching this refusal too.
type DailyLimitExceededError struct {
	RateLimitedError

	// DailyLimit is how many messages the account can send in a day.
	DailyLimit int

	// DailyRemaining is how many messages the account can still send today: 0 once
	// the limit is reached.
	DailyRemaining int

	// ResetsAt is the next midnight, Romania time, when the account can send
	// again, e.g. 2026-10-01T00:00:00+03:00; the zero time when the answer did not
	// say.
	ResetsAt time.Time
}

// Unwrap returns the embedded *RateLimitedError, so errors.As finds it.
func (e *DailyLimitExceededError) Unwrap() error {
	return &e.RateLimitedError
}

// ServerError is returned for server errors (500, 502, 503, 504).
type ServerError struct {
	APIError
}

// Unwrap returns the embedded *APIError, so errors.As finds it.
func (e *ServerError) Unwrap() error {
	return &e.APIError
}

// TransportError is returned when the request got no answer: a network error, a
// timeout or a cancelled context. Its StatusCode is 0 and the original error is
// its Err: errors.Is(err, context.DeadlineExceeded) tells a timeout of the
// context.
type TransportError struct {
	APIError
}

// Unwrap returns the embedded *APIError, so errors.As and errors.Is find it and
// the original error.
func (e *TransportError) Unwrap() error {
	return &e.APIError
}

// InvalidWebhookPayloadError is returned when a delivery status callback is not
// the payload Calisero sends: not JSON, not an object, too large, or a required
// field missing or malformed.
type InvalidWebhookPayloadError struct {
	// Message says what is wrong with the payload, e.g. `invalid delivery webhook
	// payload: "messageId" must be a non-empty string, got nothing`.
	Message string

	// Err is the error that caused this one, e.g. the JSON syntax error.
	Err error
}

// Error describes what is wrong with the payload.
func (e *InvalidWebhookPayloadError) Error() string {
	return "calisero: " + e.Message
}

// Unwrap returns the error that caused this one, if any.
func (e *InvalidWebhookPayloadError) Unwrap() error {
	return e.Err
}
