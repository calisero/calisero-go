package calisero

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// result is a successful answer: its status, its headers and its decoded JSON body.
type result struct {
	status int
	header http.Header

	// body is the decoded JSON body; nil when the answer had none.
	body any
}

// unexpected is the error of a successful answer that is not what the API
// documents: an *APIError carrying its status, trace ID and body.
func (res *result) unexpected(err error) error {
	details, _ := res.body.(map[string]any)

	return &APIError{
		Message:    "Unexpected API response: " + err.Error(),
		StatusCode: res.status,
		TraceID:    headerString(res.header, "X-Trace-Id"),
		Details:    details,
		Err:        err,
	}
}

func (c *Client) get(ctx context.Context, path string, query url.Values) (*result, error) {
	return c.send(ctx, http.MethodGet, path, query, nil)
}

func (c *Client) post(ctx context.Context, path string, payload any) (*result, error) {
	return c.send(ctx, http.MethodPost, path, nil, payload)
}

func (c *Client) put(ctx context.Context, path string, payload any) (*result, error) {
	return c.send(ctx, http.MethodPut, path, nil, payload)
}

func (c *Client) delete(ctx context.Context, path string) (*result, error) {
	return c.send(ctx, http.MethodDelete, path, nil, nil)
}

// send sends an authenticated JSON request and reads the answer: a successful one
// as a result, an error one as the matching *APIError type.
func (c *Client) send(ctx context.Context, method, path string, query url.Values, payload any) (*result, error) {
	var body io.Reader

	if payload != nil {
		encoded, err := encodeJSON(payload)
		if err != nil {
			return nil, fmt.Errorf("calisero: encode the request body: %w", err)
		}

		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.endpoint(path, query), body)
	if err != nil {
		return nil, fmt.Errorf("calisero: build the request: %w", err)
	}

	token, err := c.auth.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("calisero: get the API key from the auth provider: %w", err)
	}

	if strings.TrimSpace(token) == "" {
		return nil, errors.New("calisero: the auth provider returned an empty API key")
	}

	// Without it the API answers a missing or invalid key with a redirect, not a 401
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	// Names the library, its version and the runtime, so the platform can tell its requests apart
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, newTransportError(err)
	}

	if response.Body == nil {
		return handleResponse(response.StatusCode, response.Header, nil)
	}

	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, newTransportError(err)
	}

	return handleResponse(response.StatusCode, response.Header, data)
}

// endpoint is the URL of a request: the base URL, the path and the query string.
func (c *Client) endpoint(path string, query url.Values) string {
	endpoint := c.baseURL + "/" + strings.TrimLeft(path, "/")

	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	return endpoint
}

// encodeJSON is the JSON of a request body, characters such as & and < written
// as they are rather than escaped.
func encodeJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer

	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// newTransportError is the error of a request that got no complete answer.
func newTransportError(err error) error {
	return &TransportError{APIError{Message: "HTTP request failed: " + err.Error(), Err: err}}
}

// handleResponse reads an answer: the decoded body of a successful one, or the
// error of a failed one.
func handleResponse(status int, header http.Header, data []byte) (*result, error) {
	if status >= 200 && status < 300 {
		res := &result{status: status, header: header}

		if len(bytes.TrimSpace(data)) == 0 {
			return res, nil
		}

		body, err := decodeJSON(data)
		if err != nil {
			return nil, &APIError{
				Message:    "Failed to decode JSON response: " + err.Error(),
				StatusCode: status,
				TraceID:    traceID(header, nil),
				Err:        err,
			}
		}

		res.body = body

		return res, nil
	}

	details := decodeErrorBody(data)
	message := errorMessage(details)

	if message == "" {
		message = "HTTP error " + strconv.Itoa(status)
	}

	base := APIError{Message: message, StatusCode: status, TraceID: traceID(header, details), Details: details}

	switch status {
	case http.StatusUnauthorized:
		return nil, &UnauthorizedError{base}
	case http.StatusForbidden:
		return nil, &ForbiddenError{base}
	case http.StatusNotFound:
		return nil, &NotFoundError{base}
	case http.StatusUnprocessableEntity:
		return nil, &ValidationError{APIError: base, ValidationErrors: validationErrors(details)}
	case http.StatusTooManyRequests:
		return nil, rateLimitError(header, base)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return nil, &ServerError{base}
	default:
		return nil, &base
	}
}

// decodeErrorBody is the JSON object of an error answer; nil when it has none,
// e.g. an error page from a proxy.
func decodeErrorBody(data []byte) map[string]any {
	value, err := decodeJSON(data)
	if err != nil {
		return nil
	}

	details, _ := value.(map[string]any)

	return details
}

// errorMessage is the error's description: the API sends it as message, and
// error.message is read too; empty when the body has neither.
func errorMessage(details map[string]any) string {
	if message, ok := details["message"].(string); ok && message != "" {
		return message
	}

	if inner, ok := details["error"].(map[string]any); ok {
		if message, ok := inner["message"].(string); ok && message != "" {
			return message
		}
	}

	return ""
}

// traceID is the request's trace ID: the X-Trace-Id header the API sends with
// every answer, else the trace_id of the error body, else an X-Request-ID header.
func traceID(header http.Header, details map[string]any) string {
	if id := headerString(header, "X-Trace-Id"); id != "" {
		return id
	}

	if id, ok := details["trace_id"].(string); ok && id != "" {
		return id
	}

	return headerString(header, "X-Request-ID")
}

// validationErrors are the field errors of a 422 body (errors, or
// validation_errors), each as a list of messages; empty when it carries none.
func validationErrors(details map[string]any) map[string][]string {
	fields, ok := details["errors"].(map[string]any)
	if !ok {
		fields, _ = details["validation_errors"].(map[string]any)
	}

	result := make(map[string][]string, len(fields))

	for field, messages := range fields {
		switch typed := messages.(type) {
		case string:
			result[field] = []string{typed}
		case []any:
			list := make([]string, 0, len(typed))

			for _, message := range typed {
				if text, ok := message.(string); ok {
					list = append(list, text)
				}
			}

			result[field] = list
		}
	}

	return result
}

// rateLimitError is the error of a 429, which comes from one of two limits, told
// apart by the body's code: the account's daily sending limit
// (daily_limit_exceeded) or the request rate limit.
func rateLimitError(header http.Header, base APIError) error {
	limited := RateLimitedError{APIError: base}

	if seconds := headerInt(header, "Retry-After"); seconds != nil {
		limited.RetryAfter = time.Duration(*seconds) * time.Second
	}

	if limit := headerInt(header, "X-RateLimit-Limit"); limit != nil {
		limited.RateLimitLimit = *limit
	}

	if remaining := headerInt(header, "X-RateLimit-Remaining"); remaining != nil {
		limited.RateLimitRemaining = *remaining
	}

	if reset := headerInt(header, "X-RateLimit-Reset"); reset != nil {
		limited.RateLimitReset = time.Unix(int64(*reset), 0).UTC()
	}

	if code, _ := base.Details["code"].(string); code != DailyLimitExceededCode {
		return &limited
	}

	exceeded := &DailyLimitExceededError{RateLimitedError: limited}

	if limit := intOrHeader(base.Details, "daily_limit", header, "X-Daily-Limit"); limit != nil {
		exceeded.DailyLimit = *limit
	}

	if remaining := intOrHeader(base.Details, "daily_remaining", header, "X-Daily-Remaining"); remaining != nil {
		exceeded.DailyRemaining = *remaining
	}

	if resetsAt, ok := base.Details["resets_at"].(string); ok {
		exceeded.ResetsAt, _ = parseTime(resetsAt)
	}

	return exceeded
}

// intOrHeader is the integer of a body field, else of a header; nil when neither
// holds one.
func intOrHeader(details map[string]any, key string, header http.Header, name string) *int {
	if value, ok := toInt(details[key]); ok {
		return &value
	}

	return headerInt(header, name)
}

// digits matches a header value that is a non-negative integer.
var digits = regexp.MustCompile(`^[0-9]+$`)

// headerString is the first value of a header, trimmed; empty when the header is
// absent. Names match whatever their case, as HTTP defines them.
func headerString(header http.Header, name string) string {
	if values := header.Values(name); len(values) > 0 {
		return strings.TrimSpace(values[0])
	}

	// A header map built by hand may hold names in a case other than the canonical one
	for key, values := range header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return strings.TrimSpace(values[0])
		}
	}

	return ""
}

// headerInt is the value of a header as a non-negative integer; nil when it is
// absent or not one.
func headerInt(header http.Header, name string) *int {
	value := headerString(header, name)

	if !digits.MatchString(value) {
		return nil
	}

	number, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}

	return &number
}

// readResponseMeta reads what the headers of an answer report.
func readResponseMeta(header http.Header) ResponseMeta {
	return ResponseMeta{
		TraceID:            headerString(header, "X-Trace-Id"),
		RateLimitLimit:     headerInt(header, "X-RateLimit-Limit"),
		RateLimitRemaining: headerInt(header, "X-RateLimit-Remaining"),
		DailyLimit:         headerInt(header, "X-Daily-Limit"),
		DailyRemaining:     headerInt(header, "X-Daily-Remaining"),
	}
}
