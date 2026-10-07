package calisero_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	calisero "github.com/calisero/calisero-go"
)

// API payloads as the Calisero API sends them (field names, types and formats of
// its resources), shared by the tests.
const (
	traceID        = "9b80eef1-49d4-4502-85a8-febb68cc11a7"
	messageID      = "9e2574e8-3615-4090-9b5a-0fc812079da8"
	optOutID       = "019999b5-5687-71cd-ae81-baf6eea9d86f"
	accountID      = "9d0e0bd3-c805-4dd0-bafb-07c5ded0a8e1"
	verificationID = "019a62f1-66b7-7387-a64f-2742c12a2860"
	phone          = "+40742***350"
)

type object = map[string]any

// absent removes a field from a fixture.
var absent = &struct{ absent bool }{true}

// with is base with the overrides applied: absent removes a field.
func with(base, overrides object) object {
	result := object{}

	for key, value := range base {
		result[key] = value
	}

	for key, value := range overrides {
		if value == absent {
			delete(result, key)
		} else {
			result[key] = value
		}
	}

	return result
}

func shortenedLinkData(overrides object) object {
	return with(object{
		"id":             "019adfbb-40a1-71ee-bcb5-8d551b8cfdae",
		"original_link":  "https://yourstore.com/orders/12345/tracking",
		"shortened_link": "https://calisero.ro/s/ghJKPV",
		"click_count":    3,
		"last_click":     "2025-12-02T16:01:12.000000Z",
		"created_at":     "2025-12-02T15:43:02.000000Z",
	}, overrides)
}

func messageData(overrides object) object {
	return with(object{
		"id":             messageID,
		"recipient":      phone,
		"body":           "Hello from Calisero!",
		"parts":          1,
		"created_at":     "2025-02-06T10:18:43.000000Z",
		"scheduled_at":   "2025-02-06T10:18:43.000000Z",
		"sent_at":        nil,
		"delivered_at":   nil,
		"callback_url":   "https://yourapp.com/webhooks/calisero",
		"status":         "scheduled",
		"sender":         "CALISERO",
		"shortened_urls": []any{},
	}, overrides)
}

func optOutData(overrides object) object {
	return with(object{
		"id":         optOutID,
		"phone":      phone,
		"reason":     "Customer asked to stop receiving messages",
		"created_at": "2025-09-19T12:01:15.000000Z",
		"updated_at": "2025-09-19T12:01:15.000000Z",
	}, overrides)
}

func accountData(overrides object) object {
	return with(object{
		"id":              accountID,
		"code":            "ABC1234",
		"name":            "My Company",
		"description":     "Company description",
		"fiscal_code":     "RO12345678",
		"registry_number": "J40/123456/2023",
		"iban":            "RO49AAAA1B31007593840000",
		"city":            "Bucharest",
		"state":           "Bucharest",
		"country":         "RO",
		"address":         "Main Street 123",
		"postal_code":     123456,
		"email":           "contact@company.com",
		"phone":           "+40721***456",
		"contact_person":  "John Doe",
		"credit":          100.5,
		"status":          "active",
		"sandbox":         false,
		"daily_limit":     1000,
		"daily_remaining": 873,
		"sent_today":      127,
		"created_at":      "2024-09-20T12:48:55.000000Z",
	}, overrides)
}

func verificationData(overrides object) object {
	return with(object{
		"id":          verificationID,
		"phone":       phone,
		"brand":       "Calisero",
		"status":      "unverified",
		"template":    nil,
		"created_at":  "2025-11-08T10:09:38.000000Z",
		"expires_at":  "2025-11-08T10:12:38.000000Z",
		"verified_at": nil,
		"attempts":    0,
		"expired":     false,
	}, overrides)
}

func webhookPayload(overrides object) object {
	return with(object{
		"price":            0.0378,
		"sender":           "CALISERO",
		"sentAt":           "2025-09-19T12:01:15.000000Z",
		"status":           "delivered",
		"messageId":        "019961db-14a7-7348-963f-5a7a789a969f",
		"recipient":        phone,
		"scheduleAt":       "2025-09-19T12:02:51.000000Z",
		"deliveredAt":      "2025-09-19T12:02:53.000000Z",
		"remainingBalance": 999.392,
		"dailyLimit":       1000,
		"dailyRemaining":   588,
		"sentToday":        412,
	}, overrides)
}

// pageOf is a page of a list as the API sends it (simple pagination: no total, no
// last page).
func pageOf(items []any, resource string, page int, hasNext bool) object {
	if items == nil {
		items = []any{}
	}

	path := "https://rest.calisero.ro/api/v1/" + resource
	links := object{"first": path + "?page=1", "last": nil, "prev": nil, "next": nil}
	meta := object{"current_page": page, "current_page_url": fmt.Sprintf("%s?page=%d", path, page), "from": nil, "path": path, "per_page": 50, "to": nil}

	if page > 1 {
		links["prev"] = fmt.Sprintf("%s?page=%d", path, page-1)
	}

	if hasNext {
		links["next"] = fmt.Sprintf("%s?page=%d", path, page+1)
	}

	if len(items) > 0 {
		from := (page-1)*50 + 1
		meta["from"] = from
		meta["to"] = from + len(items) - 1
	}

	return object{"data": items, "links": links, "meta": meta}
}

// dailyLimitBody is the API's refusal of the daily sending limit.
var dailyLimitBody = object{
	"message":         "This account can send at most 1,000 messages a day. The limit resets at midnight, Romania time (2026-10-01T00:00:00+03:00). Contact us to raise it.",
	"code":            "daily_limit_exceeded",
	"daily_limit":     1000,
	"daily_remaining": 0,
	"resets_at":       "2026-10-01T00:00:00+03:00",
	"trace_id":        traceID,
}

// recordedRequest is a request the stub received, its body read.
type recordedRequest struct {
	Method string
	URL    *url.URL
	Header http.Header
	Body   string
}

// JSON is the decoded JSON body of the request.
func (r *recordedRequest) JSON(t *testing.T) object {
	t.Helper()

	var body object
	if err := json.Unmarshal([]byte(r.Body), &body); err != nil {
		t.Fatalf("the request body is not a JSON object: %v (%q)", err, r.Body)
	}

	return body
}

// stubDoer is an HTTPDoer that records its requests and gives the queued answers
// in turn: an *http.Response, or the error of a request that got none.
type stubDoer struct {
	mu       sync.Mutex
	requests []*recordedRequest
	answers  []any
}

func (s *stubDoer) Do(request *http.Request) (*http.Response, error) {
	body := ""

	if request.Body != nil {
		data, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}

		body = string(data)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, &recordedRequest{Method: request.Method, URL: request.URL, Header: request.Header.Clone(), Body: body})

	if len(s.answers) == 0 {
		return nil, fmt.Errorf("stubDoer: no answer queued for %s %s", request.Method, request.URL)
	}

	answer := s.answers[0]
	s.answers = s.answers[1:]

	switch typed := answer.(type) {
	case error:
		return nil, typed
	case *http.Response:
		typed.Request = request

		return typed, nil
	default:
		return nil, fmt.Errorf("stubDoer: cannot answer with a %T", answer)
	}
}

// lastRequest is the last request the stub received.
func (s *stubDoer) lastRequest(t *testing.T) *recordedRequest {
	t.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.requests) == 0 {
		t.Fatal("stubDoer: no request was sent")
	}

	return s.requests[len(s.requests)-1]
}

// requestCount is how many requests the stub received.
func (s *stubDoer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.requests)
}

// jsonResponse is a JSON answer, as the API sends them; headers are name, value
// pairs, kept with the case they are given in.
func jsonResponse(status int, body any, headers ...string) *http.Response {
	encoded, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}

	return rawResponse(status, string(encoded), append([]string{"Content-Type", "application/json"}, headers...)...)
}

// rawResponse is an answer with the given body; headers are name, value pairs,
// kept with the case they are given in.
func rawResponse(status int, body string, headers ...string) *http.Response {
	header := http.Header{}

	for index := 0; index+1 < len(headers); index += 2 {
		header[headers[index]] = append(header[headers[index]], headers[index+1])
	}

	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// clientAnswering is a client whose requests get the given answers in turn.
func clientAnswering(t *testing.T, answers ...any) (*calisero.Client, *stubDoer) {
	t.Helper()

	stub := &stubDoer{answers: answers}

	client, err := calisero.NewClient("test-api-key", calisero.WithHTTPClient(stub))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	return client, stub
}

func ptr[T any](value T) *T {
	return &value
}

func equal[T comparable](t *testing.T, got, want T, what string) {
	t.Helper()

	if got != want {
		t.Errorf("%s: got %#v, want %#v", what, got, want)
	}
}

func deepEqual(t *testing.T, got, want any, what string) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n got  %#v\n want %#v", what, got, want)
	}
}

func noError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// errorAs is err as the error type E, failing the test when it is not one.
func errorAs[E error](t *testing.T, err error) E {
	t.Helper()

	var target E
	if !errors.As(err, &target) {
		t.Fatalf("got the error %T (%v), want a %T", err, err, target)
	}

	return target
}

// isA fails the test when err is not an E.
func isA[E error](t *testing.T, err error) {
	t.Helper()

	var target E
	if !errors.As(err, &target) {
		t.Errorf("got the error %T (%v), want a %T", err, err, target)
	}
}

// isNot fails the test when err is an E.
func isNot[E error](t *testing.T, err error) {
	t.Helper()

	var target E
	if errors.As(err, &target) {
		t.Errorf("the error %T (%v) is a %T, it should not be", err, err, target)
	}
}

func intValue(value *int) any {
	if value == nil {
		return nil
	}

	return *value
}
