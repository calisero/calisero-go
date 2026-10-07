package calisero_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func TestParseDeliveryWebhookReadsAFullPayload(t *testing.T) {
	message, err := calisero.ParseDeliveryWebhook(mustJSON(t, webhookPayload(nil)))
	noError(t, err)

	deepEqual(t, *message, calisero.DeliveryWebhookMessage{
		MessageID:        "019961db-14a7-7348-963f-5a7a789a969f",
		Sender:           "CALISERO",
		Recipient:        phone,
		Status:           calisero.DeliveryStatusDelivered,
		ScheduleAt:       ptr(time.Date(2025, 9, 19, 12, 2, 51, 0, time.UTC)),
		SentAt:           ptr(time.Date(2025, 9, 19, 12, 1, 15, 0, time.UTC)),
		DeliveredAt:      ptr(time.Date(2025, 9, 19, 12, 2, 53, 0, time.UTC)),
		Price:            0.0378,
		RemainingBalance: ptr(999.392),
		DailyLimit:       ptr(1000),
		DailyRemaining:   ptr(588),
		SentToday:        ptr(412),
	}, "message")
}

func TestParseDeliveryWebhookReadsAMinimalPayload(t *testing.T) {
	message, err := calisero.ParseDeliveryWebhook([]byte(`{"messageId":"019961db-14a7-7348-963f-5a7a789a969f","recipient":"+40742***350","status":"sent","price":0.0378}`))
	noError(t, err)

	equal(t, message.Status, calisero.DeliveryStatusSent, "status")
	equal(t, message.Sender, "", "default sender")
	equal(t, message.ScheduleAt == nil && message.SentAt == nil && message.DeliveredAt == nil, true, "no dates")
	equal(t, message.RemainingBalance == nil, true, "remaining balance")
}

func TestParseDeliveryWebhookReadsAPayloadWithoutTheDailyLimit(t *testing.T) {
	message, err := calisero.ParseDeliveryWebhook(mustJSON(t, webhookPayload(object{
		"dailyLimit":     nil,
		"dailyRemaining": nil,
		"sentToday":      absent,
	})))
	noError(t, err)

	equal(t, message.DailyLimit == nil && message.DailyRemaining == nil && message.SentToday == nil, true, "no daily limit")
}

func TestParseDeliveryWebhookReadsAmountsSentAsStrings(t *testing.T) {
	message, err := calisero.ParseDeliveryWebhook(mustJSON(t, webhookPayload(object{"price": "0.0378", "remainingBalance": "999.392"})))
	noError(t, err)

	equal(t, message.Price, 0.0378, "price")
	equal(t, *message.RemainingBalance, 999.392, "remaining balance")
}

func TestParseDeliveryWebhookIgnoresOptionalFieldsOfTheWrongType(t *testing.T) {
	message, err := calisero.ParseDeliveryWebhook(mustJSON(t, webhookPayload(object{
		"sender":           42,
		"sentAt":           "soon",
		"remainingBalance": "a lot",
		"dailyLimit":       "1000",
	})))
	noError(t, err)

	equal(t, message.Sender, "", "sender")
	equal(t, message.SentAt == nil, true, "sent at")
	equal(t, message.RemainingBalance == nil, true, "remaining balance")
	equal(t, message.DailyLimit == nil, true, "daily limit")
}

func TestParseDeliveryWebhookRefusesWhatIsNotADeliveryStatusUpdate(t *testing.T) {
	cases := map[string]struct {
		body    string
		message string
	}{
		"not JSON":               {"messageId=1", "invalid delivery webhook payload: invalid character 'm' looking for beginning of value"},
		"truncated JSON":         {`{"messageId":`, "invalid delivery webhook payload: unexpected end of JSON input"},
		"nothing":                {"", "invalid delivery webhook payload: unexpected end of JSON input"},
		"a cancelled message":    {"[]", "invalid delivery webhook payload: expected an object, got an array"},
		"null":                   {"null", "invalid delivery webhook payload: expected an object, got null"},
		"a string":               {`"delivered"`, "invalid delivery webhook payload: expected an object, got a string"},
		"no message ID":          {string(mustJSON(t, webhookPayload(object{"messageId": absent}))), `invalid delivery webhook payload: "messageId" must be a non-empty string, got nothing`},
		"an empty message ID":    {string(mustJSON(t, webhookPayload(object{"messageId": ""}))), `invalid delivery webhook payload: "messageId" must be a non-empty string, got a string`},
		"a numeric message ID":   {string(mustJSON(t, webhookPayload(object{"messageId": 42}))), `invalid delivery webhook payload: "messageId" must be a non-empty string, got a number`},
		"no recipient":           {string(mustJSON(t, webhookPayload(object{"recipient": nil}))), `invalid delivery webhook payload: "recipient" must be a non-empty string, got null`},
		"no status":              {string(mustJSON(t, webhookPayload(object{"status": absent}))), `invalid delivery webhook payload: "status" must be a non-empty string, got nothing`},
		"no price":               {string(mustJSON(t, webhookPayload(object{"price": absent}))), `invalid delivery webhook payload: "price" must be a number, got nothing`},
		"a price that is a word": {string(mustJSON(t, webhookPayload(object{"price": "free"}))), `invalid delivery webhook payload: "price" must be a number, got a string`},
	}

	for name, tc := range cases {
		message, err := calisero.ParseDeliveryWebhook([]byte(tc.body))

		if message != nil {
			t.Errorf("%s: read %+v", name, message)
		}

		invalid := errorAs[*calisero.InvalidWebhookPayloadError](t, err)
		equal(t, invalid.Message, tc.message, name)
		equal(t, err.Error(), "calisero: "+tc.message, name+": Error()")
		equal(t, invalid.Err != nil, true, name+": the cause is kept")
	}
}

func TestDeliveryWebhookMessageIsReadWithJSONUnmarshal(t *testing.T) {
	var message calisero.DeliveryWebhookMessage
	noError(t, json.Unmarshal(mustJSON(t, webhookPayload(object{"price": "0.05"})), &message))

	equal(t, message.Price, 0.05, "price")

	for _, body := range []string{"null", "[]", string(mustJSON(t, webhookPayload(object{"price": absent})))} {
		var refused calisero.DeliveryWebhookMessage

		err := json.Unmarshal([]byte(body), &refused)
		isA[*calisero.InvalidWebhookPayloadError](t, err)
	}
}

func TestDeliveryWebhookMessageSurvivesAJSONRoundTrip(t *testing.T) {
	message, err := calisero.ParseDeliveryWebhook(mustJSON(t, webhookPayload(nil)))
	noError(t, err)

	decoded, err := calisero.ParseDeliveryWebhook(mustJSON(t, message))
	noError(t, err)

	deepEqual(t, decoded, message, "message")
}

// webhookCall is what a DeliveryWebhookHandler answered and what it reported.
type webhookCall struct {
	status   int
	header   http.Header
	body     string
	messages []*calisero.DeliveryWebhookMessage
	requests []*http.Request
	errors   []error
}

// callWebhook sends a request to a handler set up with configure, and records its
// answer, the updates OnMessage got and the errors OnError got.
func callWebhook(t *testing.T, request *http.Request, configure func(*calisero.DeliveryWebhookHandler)) *webhookCall {
	t.Helper()

	call := &webhookCall{}
	handler := &calisero.DeliveryWebhookHandler{
		OnMessage: func(r *http.Request, message *calisero.DeliveryWebhookMessage) error {
			call.messages = append(call.messages, message)
			call.requests = append(call.requests, r)

			return nil
		},
		OnError: func(_ *http.Request, err error) {
			call.errors = append(call.errors, err)
		},
	}

	if configure != nil {
		configure(handler)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	call.status = recorder.Code
	call.header = recorder.Header()
	call.body = recorder.Body.String()

	return call
}

func callbackRequest(target string, body []byte) *http.Request {
	request := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	return request
}

func TestDeliveryWebhookHandlerHandsTheUpdateToOnMessage(t *testing.T) {
	call := callWebhook(t, callbackRequest("/webhooks/calisero?tenant=42", mustJSON(t, webhookPayload(nil))), nil)

	equal(t, call.status, http.StatusOK, "status")
	equal(t, call.body, `{"received":true}`, "body")
	equal(t, call.header.Get("Content-Type"), "application/json", "Content-Type")
	equal(t, len(call.messages), 1, "updates")
	equal(t, call.messages[0].MessageID, "019961db-14a7-7348-963f-5a7a789a969f", "message ID")
	equal(t, call.requests[0].URL.Query().Get("tenant"), "42", "the request reaches OnMessage")
	equal(t, len(call.errors), 0, "errors")
}

func TestDeliveryWebhookHandlerRefusesWhatIsNotAPost(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodHead, http.MethodOptions} {
		call := callWebhook(t, httptest.NewRequest(method, "/webhooks/calisero", nil), nil)

		equal(t, call.status, http.StatusMethodNotAllowed, method+": status")
		equal(t, call.header.Get("Allow"), http.MethodPost, method+": Allow")
		equal(t, len(call.messages), 0, method+": updates")
	}
}

func TestDeliveryWebhookHandlerChecksTheToken(t *testing.T) {
	withToken := func(handler *calisero.DeliveryWebhookHandler) { handler.Token = "s3cr3t" }

	for _, target := range []string{"/webhooks/calisero", "/webhooks/calisero?token=", "/webhooks/calisero?token=wrong", "/webhooks/calisero?token=S3CR3T", "/webhooks/calisero?other=s3cr3t"} {
		call := callWebhook(t, callbackRequest(target, mustJSON(t, webhookPayload(nil))), withToken)

		equal(t, call.status, http.StatusUnauthorized, target+": status")
		equal(t, call.body, `{"error":"Invalid webhook token"}`, target+": body")
		equal(t, len(call.messages), 0, target+": updates")

		if len(call.errors) != 1 || !errors.Is(call.errors[0], calisero.ErrInvalidWebhookToken) {
			t.Errorf("%s: OnError got %v", target, call.errors)
		}
	}

	call := callWebhook(t, callbackRequest("/webhooks/calisero?token=s3cr3t", mustJSON(t, webhookPayload(nil))), withToken)
	equal(t, call.status, http.StatusOK, "the right token")
}

func TestDeliveryWebhookHandlerRefusesAnInvalidPayload(t *testing.T) {
	for _, body := range []string{"not json", "[]", "{}", string(mustJSON(t, webhookPayload(object{"price": nil})))} {
		call := callWebhook(t, callbackRequest("/webhooks/calisero", []byte(body)), nil)

		equal(t, call.status, http.StatusBadRequest, body+": status")
		equal(t, call.body, `{"error":"Invalid payload"}`, body+": body")
		equal(t, len(call.messages), 0, body+": updates")

		if len(call.errors) != 1 {
			t.Fatalf("%s: OnError got %v", body, call.errors)
		}

		isA[*calisero.InvalidWebhookPayloadError](t, call.errors[0])
	}
}

func TestDeliveryWebhookHandlerRefusesAPayloadTooLarge(t *testing.T) {
	call := callWebhook(t, callbackRequest("/webhooks/calisero", mustJSON(t, webhookPayload(nil))), func(handler *calisero.DeliveryWebhookHandler) {
		handler.MaxBodySize = 16
	})

	equal(t, call.status, http.StatusRequestEntityTooLarge, "status")
	equal(t, call.body, `{"error":"Payload too large"}`, "body")
	equal(t, errorAs[*calisero.InvalidWebhookPayloadError](t, call.errors[0]).Message, "invalid delivery webhook payload: larger than 16 bytes", "error")

	var tooLarge *http.MaxBytesError
	equal(t, errors.As(call.errors[0], &tooLarge), true, "the cause is kept")
}

func TestDeliveryWebhookHandlerAcceptsUpTo64KiBByDefault(t *testing.T) {
	payload := webhookPayload(object{"padding": strings.Repeat("x", 60<<10)})
	call := callWebhook(t, callbackRequest("/webhooks/calisero", mustJSON(t, payload)), nil)
	equal(t, call.status, http.StatusOK, "60 KiB")

	payload = webhookPayload(object{"padding": strings.Repeat("x", calisero.DefaultWebhookMaxBodySize)})
	call = callWebhook(t, callbackRequest("/webhooks/calisero", mustJSON(t, payload)), nil)
	equal(t, call.status, http.StatusRequestEntityTooLarge, "more than 64 KiB")
	equal(t, calisero.DefaultWebhookMaxBodySize, 64*1024, "default")
}

func TestDeliveryWebhookHandlerAnswers500WhenOnMessageFails(t *testing.T) {
	failure := errors.New("database unavailable")
	call := callWebhook(t, callbackRequest("/webhooks/calisero", mustJSON(t, webhookPayload(nil))), func(handler *calisero.DeliveryWebhookHandler) {
		handler.OnMessage = func(*http.Request, *calisero.DeliveryWebhookMessage) error { return failure }
	})

	equal(t, call.status, http.StatusInternalServerError, "status")
	equal(t, call.body, `{"error":"Webhook handler failed"}`, "body")

	if len(call.errors) != 1 || !errors.Is(call.errors[0], failure) {
		t.Errorf("OnError got %v", call.errors)
	}
}

func TestDeliveryWebhookHandlerWithoutOnMessage(t *testing.T) {
	call := callWebhook(t, callbackRequest("/webhooks/calisero", mustJSON(t, webhookPayload(nil))), func(handler *calisero.DeliveryWebhookHandler) {
		handler.OnMessage = nil
	})

	equal(t, call.status, http.StatusInternalServerError, "status")

	if len(call.errors) != 1 || call.errors[0].Error() != "calisero: the DeliveryWebhookHandler has no OnMessage function" {
		t.Errorf("OnError got %v", call.errors)
	}
}

func TestDeliveryWebhookHandlerWithoutOnError(t *testing.T) {
	handler := &calisero.DeliveryWebhookHandler{
		OnMessage: func(*http.Request, *calisero.DeliveryWebhookMessage) error { return nil },
		Token:     "s3cr3t",
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, callbackRequest("/webhooks/calisero", []byte("{}")))

	equal(t, recorder.Code, http.StatusUnauthorized, "status")
}

func TestDeliveryWebhookHandlerOfARequestWithoutABody(t *testing.T) {
	request := callbackRequest("/webhooks/calisero", nil)
	request.Body = nil

	call := callWebhook(t, request, nil)

	equal(t, call.status, http.StatusBadRequest, "status")
}

type brokenBody struct{}

func (brokenBody) Read([]byte) (int, error) { return 0, errors.New("connection reset by peer") }
func (brokenBody) Close() error             { return nil }

func TestDeliveryWebhookHandlerOfABodyThatCannotBeRead(t *testing.T) {
	request := callbackRequest("/webhooks/calisero", nil)
	request.Body = brokenBody{}

	call := callWebhook(t, request, nil)

	equal(t, call.status, http.StatusBadRequest, "status")
	equal(t, errorAs[*calisero.InvalidWebhookPayloadError](t, call.errors[0]).Message, "invalid delivery webhook payload: connection reset by peer", "error")
}

func TestDeliveryWebhookHandlerOverHTTP(t *testing.T) {
	var (
		mu       sync.Mutex
		received []*calisero.DeliveryWebhookMessage
	)

	mux := http.NewServeMux()
	mux.Handle("POST /webhooks/calisero", &calisero.DeliveryWebhookHandler{
		Token: "s3cr3t",
		OnMessage: func(_ *http.Request, message *calisero.DeliveryWebhookMessage) error {
			mu.Lock()
			defer mu.Unlock()

			received = append(received, message)

			return nil
		},
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	post := func(target, body string) (int, string) {
		request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+target, strings.NewReader(body))
		noError(t, err)
		request.Header.Set("Content-Type", "application/json")

		response, err := server.Client().Do(request)
		noError(t, err)

		defer response.Body.Close()

		answer, err := io.ReadAll(response.Body)
		noError(t, err)

		return response.StatusCode, string(answer)
	}

	status, body := post("/webhooks/calisero?token=s3cr3t", string(mustJSON(t, webhookPayload(nil))))
	equal(t, status, http.StatusOK, "status")
	equal(t, body, `{"received":true}`, "body")

	status, _ = post("/webhooks/calisero?token=wrong", string(mustJSON(t, webhookPayload(nil))))
	equal(t, status, http.StatusUnauthorized, "a wrong token")

	status, _ = post("/webhooks/calisero?token=s3cr3t", "[]")
	equal(t, status, http.StatusBadRequest, "a cancelled message")

	mu.Lock()
	defer mu.Unlock()

	equal(t, len(received), 1, "updates")
	equal(t, received[0].Status, calisero.DeliveryStatusDelivered, "status")
}
