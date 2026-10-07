package calisero

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

// DeliveryStatus is a status a delivery status callback reports; other values may
// come with future API versions.
type DeliveryStatus string

// The statuses a delivery status callback reports.
const (
	DeliveryStatusSent        DeliveryStatus = "sent"
	DeliveryStatusDelivered   DeliveryStatus = "delivered"
	DeliveryStatusUndelivered DeliveryStatus = "undelivered"
)

// DeliveryWebhookMessage is the payload Calisero posts to a message's callback
// URL when its status changes: sent, delivered or undelivered.
//
// It reaches your endpoint from outside your application, so it is validated:
// ParseDeliveryWebhook and UnmarshalJSON refuse a malformed payload with an
// *InvalidWebhookPayloadError.
type DeliveryWebhookMessage struct {
	MessageID string `json:"messageId"`

	// Sender is the sender ID the message went out under; empty for the default sender.
	Sender string `json:"sender"`

	// Recipient is the recipient in E.164 format.
	Recipient string `json:"recipient"`

	// Status is the delivery status: sent, delivered or undelivered.
	Status DeliveryStatus `json:"status"`

	// ScheduleAt is when the message was scheduled to go out, if it was scheduled.
	ScheduleAt *time.Time `json:"scheduleAt"`

	// SentAt is when the message was handed to the network (statuses sent and
	// delivered).
	SentAt *time.Time `json:"sentAt"`

	// DeliveredAt is when the handset confirmed delivery (status delivered only).
	DeliveredAt *time.Time `json:"deliveredAt"`

	// Price is the price charged for the message's parts.
	Price float64 `json:"price"`

	// RemainingBalance is the account's balance after billing, if available.
	RemainingBalance *float64 `json:"remainingBalance"`

	// DailyLimit is the account's daily sending limit when the callback was sent;
	// nil when no daily limit applies.
	DailyLimit *int `json:"dailyLimit"`

	// DailyRemaining is how many messages the account could still send today when
	// the callback was sent; nil when no daily limit applies.
	DailyRemaining *int `json:"dailyRemaining"`

	// SentToday is the number of real messages the account created today, Romania
	// time, when the callback was sent: OTP codes included, test messages left
	// out, one per message whatever its parts. Nil in payloads that predate the
	// daily limit.
	SentToday *int `json:"sentToday"`
}

// ParseDeliveryWebhook reads the raw body of a delivery status callback.
//
// It returns an *InvalidWebhookPayloadError when the body is not a JSON object or
// a required field is missing or malformed, which includes the [] Calisero sends
// for a message cancelled in the meantime.
func ParseDeliveryWebhook(body []byte) (*DeliveryWebhookMessage, error) {
	value, err := decodeJSON(body)
	if err != nil {
		return nil, &InvalidWebhookPayloadError{Message: "invalid delivery webhook payload: " + err.Error(), Err: err}
	}

	message, err := readDeliveryWebhookMessage(value)
	if err != nil {
		return nil, &InvalidWebhookPayloadError{Message: err.Error(), Err: err}
	}

	return &message, nil
}

// UnmarshalJSON reads a delivery status callback the way ParseDeliveryWebhook
// does, e.g. when a web framework binds the request's body: a malformed payload,
// the JSON null included, is an *InvalidWebhookPayloadError.
func (m *DeliveryWebhookMessage) UnmarshalJSON(data []byte) error {
	message, err := ParseDeliveryWebhook(data)
	if err != nil {
		return err
	}

	*m = *message

	return nil
}

func readDeliveryWebhookMessage(value any) (DeliveryWebhookMessage, error) {
	r := newReader(value, "delivery webhook payload")
	message := DeliveryWebhookMessage{
		MessageID:        r.nonEmpty("messageId"),
		Sender:           r.optStr("sender"),
		Recipient:        r.nonEmpty("recipient"),
		Status:           DeliveryStatus(r.nonEmpty("status")),
		ScheduleAt:       r.optTimestamp("scheduleAt"),
		SentAt:           r.optTimestamp("sentAt"),
		DeliveredAt:      r.optTimestamp("deliveredAt"),
		Price:            r.number("price"),
		RemainingBalance: r.optNumber("remainingBalance"),
		DailyLimit:       r.optInt("dailyLimit"),
		DailyRemaining:   r.optInt("dailyRemaining"),
		SentToday:        r.optInt("sentToday"),
	}

	if r.err != nil {
		return DeliveryWebhookMessage{}, r.err
	}

	return message, nil
}

// DefaultWebhookMaxBodySize is the largest callback body a DeliveryWebhookHandler
// accepts when its MaxBodySize is not set: 64 KiB.
const DefaultWebhookMaxBodySize = 64 << 10

// ErrInvalidWebhookToken is reported to DeliveryWebhookHandler.OnError for a
// callback whose URL lacks the handler's token.
var ErrInvalidWebhookToken = errors.New("calisero: invalid webhook token: the request URL lacks the expected token")

// DeliveryWebhookHandler answers the delivery status callbacks Calisero posts to
// a message's callback URL: an http.Handler for net/http and the routers built on
// it. It answers:
//
//   - 200 {"received":true} once OnMessage handled the update;
//   - 400 for a payload that is not a delivery status update, 413 for one larger
//     than MaxBodySize;
//   - 401 when Token is set and the request URL lacks it;
//   - 405 for anything but POST;
//   - 500 when OnMessage returns an error.
//
// Calisero retries only a callback that got no answer within 2 seconds, at most 5
// attempts in total, never one answered with an error.
type DeliveryWebhookHandler struct {
	// OnMessage is called with every delivery status update, before the answer,
	// which Calisero waits 2 seconds for: record the status or queue a job, then
	// return. An error it returns is answered with 500. Required.
	OnMessage func(r *http.Request, message *DeliveryWebhookMessage) error

	// Token is a secret the callback URL carries as its token query parameter,
	// e.g. https://yourapp.com/webhooks/calisero?token=…: a request without it is
	// answered with 401, the comparison taking constant time. Calisero does not
	// sign its callbacks, so this is how to accept only those of the URLs you gave
	// it. No token is checked when it is empty.
	Token string

	// OnError is called when a request is refused or OnMessage fails, e.g. to log it.
	OnError func(r *http.Request, err error)

	// MaxBodySize is the largest body accepted, in bytes: DefaultWebhookMaxBodySize
	// when it is not positive.
	MaxBodySize int64
}

// ServeHTTP answers a delivery status callback.
func (h *DeliveryWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		answer(w, http.StatusMethodNotAllowed, `{"error":"Method not allowed"}`)

		return
	}

	if h.Token != "" && !hasToken(r, h.Token) {
		h.report(r, ErrInvalidWebhookToken)
		answer(w, http.StatusUnauthorized, `{"error":"Invalid webhook token"}`)

		return
	}

	if h.OnMessage == nil {
		h.report(r, errors.New("calisero: the DeliveryWebhookHandler has no OnMessage function"))
		answer(w, http.StatusInternalServerError, `{"error":"Webhook handler failed"}`)

		return
	}

	message, err := h.read(w, r)
	if err != nil {
		h.report(r, err)

		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			answer(w, http.StatusRequestEntityTooLarge, `{"error":"Payload too large"}`)
		} else {
			answer(w, http.StatusBadRequest, `{"error":"Invalid payload"}`)
		}

		return
	}

	if err := h.OnMessage(r, message); err != nil {
		h.report(r, err)
		answer(w, http.StatusInternalServerError, `{"error":"Webhook handler failed"}`)

		return
	}

	answer(w, http.StatusOK, `{"received":true}`)
}

// read reads the callback's body, up to MaxBodySize.
func (h *DeliveryWebhookHandler) read(w http.ResponseWriter, r *http.Request) (*DeliveryWebhookMessage, error) {
	if r.Body == nil {
		return ParseDeliveryWebhook(nil)
	}

	limit := h.MaxBodySize
	if limit <= 0 {
		limit = DefaultWebhookMaxBodySize
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		message := "invalid delivery webhook payload: " + err.Error()

		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			message = "invalid delivery webhook payload: larger than " + strconv.FormatInt(limit, 10) + " bytes"
		}

		return nil, &InvalidWebhookPayloadError{Message: message, Err: err}
	}

	return ParseDeliveryWebhook(body)
}

func (h *DeliveryWebhookHandler) report(r *http.Request, err error) {
	if h.OnError != nil {
		h.OnError(r, err)
	}
}

// hasToken reports whether the request URL carries the token: hashes of equal
// length, compared in constant time.
func hasToken(r *http.Request, expected string) bool {
	provided := sha256.Sum256([]byte(r.URL.Query().Get("token")))
	wanted := sha256.Sum256([]byte(expected))

	return subtle.ConstantTimeCompare(provided[:], wanted[:]) == 1
}

// answer writes a JSON answer.
func answer(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
