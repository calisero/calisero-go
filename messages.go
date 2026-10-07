package calisero

import (
	"context"
	"time"
)

// MessageStatus is the status of a message; other values may come with future API
// versions.
type MessageStatus string

// The statuses of a message. Only a scheduled message can be cancelled.
const (
	MessageStatusScheduled   MessageStatus = "scheduled"
	MessageStatusPending     MessageStatus = "pending"
	MessageStatusQueued      MessageStatus = "queued"
	MessageStatusSent        MessageStatus = "sent"
	MessageStatusDelivered   MessageStatus = "delivered"
	MessageStatusUndelivered MessageStatus = "undelivered"
	MessageStatusFailed      MessageStatus = "failed"
	MessageStatusCanceled    MessageStatus = "canceled"
)

// Message is an SMS message.
type Message struct {
	ID string `json:"id"`

	// Recipient is the recipient's phone number.
	Recipient string `json:"recipient"`

	// Body is the text sent, or the VisibleBody it was sent with.
	Body string `json:"body"`

	// Parts is how many SMS parts the body takes.
	Parts int `json:"parts"`

	CreatedAt time.Time `json:"created_at"`

	// ScheduledAt is when the message is or was due to be sent.
	ScheduledAt *time.Time `json:"scheduled_at"`

	// SentAt is when the message was handed to the network; nil until it is.
	SentAt *time.Time `json:"sent_at"`

	// DeliveredAt is when the handset confirmed delivery; nil until it does.
	DeliveredAt *time.Time `json:"delivered_at"`

	// CallbackURL is where Calisero posts the message's delivery status updates;
	// empty when it has none.
	CallbackURL string `json:"callback_url"`

	Status MessageStatus `json:"status"`

	// Sender is the sender ID; empty for the default sender.
	Sender string `json:"sender"`

	// ShortenedURLs are the links of the body Calisero shortened (asked with
	// ShortenURLs), with their click statistics; empty when none were.
	ShortenedURLs []ShortenedLink `json:"shortened_urls"`
}

// UnmarshalJSON reads a message as the API sends it, the way the client does: a
// required field missing or malformed is an error naming it.
func (m *Message) UnmarshalJSON(data []byte) error {
	return unmarshalResource(data, m, "message", readMessage)
}

func readMessage(value any, context string) (Message, error) {
	r := newReader(value, context)
	message := Message{
		ID:          r.str("id"),
		Recipient:   r.str("recipient"),
		Body:        r.str("body"),
		Parts:       r.integer("parts"),
		CreatedAt:   r.timestamp("created_at"),
		ScheduledAt: r.optTimestamp("scheduled_at"),
		SentAt:      r.optTimestamp("sent_at"),
		DeliveredAt: r.optTimestamp("delivered_at"),
		CallbackURL: r.optStr("callback_url"),
		Status:      MessageStatus(r.str("status")),
		Sender:      r.optStr("sender"),
	}
	message.ShortenedURLs = readList(r, r.optArray("shortened_urls"), context+".shortened_urls", readShortenedLink)

	if r.err != nil {
		return Message{}, r.err
	}

	return message, nil
}

// ShortenedLink is a link of a message body shortened by Calisero, with its click
// statistics.
type ShortenedLink struct {
	ID string `json:"id"`

	// OriginalLink is the URL as it was written in the message body.
	OriginalLink string `json:"original_link"`

	// ShortenedLink is the short URL that replaced it in the SMS sent.
	ShortenedLink string `json:"shortened_link"`

	// ClickCount is how many times the short URL was opened.
	ClickCount int `json:"click_count"`

	// LastClick is when the short URL was last opened; nil if it never was.
	LastClick *time.Time `json:"last_click"`

	CreatedAt time.Time `json:"created_at"`
}

// UnmarshalJSON reads a shortened link as the API sends it, the way the client
// does: a required field missing or malformed is an error naming it.
func (l *ShortenedLink) UnmarshalJSON(data []byte) error {
	return unmarshalResource(data, l, "shortened link", readShortenedLink)
}

func readShortenedLink(value any, context string) (ShortenedLink, error) {
	r := newReader(value, context)
	link := ShortenedLink{
		ID:            r.str("id"),
		OriginalLink:  r.str("original_link"),
		ShortenedLink: r.str("shortened_link"),
		LastClick:     r.optTimestamp("last_click"),
		CreatedAt:     r.timestamp("created_at"),
	}

	if clicks := r.optInt("click_count"); clicks != nil {
		link.ClickCount = *clicks
	}

	if r.err != nil {
		return ShortenedLink{}, r.err
	}

	return link, nil
}

// CreateMessageRequest is a message to send. The optional fields left at their
// zero value are not sent.
type CreateMessageRequest struct {
	// Recipient is the recipient's mobile number in international E.164 format,
	// e.g. +40742123456. Required.
	Recipient string

	// Body is the text sent, at most 1600 characters. Required.
	Body string

	// VisibleBody is what the API shows instead of Body in its answers, its lists
	// and the dashboard, e.g. the text with a code masked: "Your code is ******".
	VisibleBody string

	// Validity is the message's validity period, in hours.
	Validity int

	// ScheduleAt is when to send the message, converted to the Romania time the
	// API reads; the zero time sends it at once, as does a time already past
	// today. From today to 5 years ahead.
	ScheduleAt time.Time

	// CallbackURL is where Calisero posts the message's delivery status updates.
	CallbackURL string

	// Sender is one of the account's approved sender IDs, exactly as approved; the
	// default sender when empty.
	Sender string

	// ShortenURLs replaces the http:// and https:// links of the body with short
	// ones, whose clicks are counted. The body is sent as written otherwise.
	ShortenURLs bool
}

// createMessagePayload is the body of POST /messages.
type createMessagePayload struct {
	Recipient   string `json:"recipient"`
	Body        string `json:"body"`
	VisibleBody string `json:"visible_body,omitempty"`
	Validity    int    `json:"validity,omitempty"`
	ScheduleAt  string `json:"schedule_at,omitempty"`
	CallbackURL string `json:"callback_url,omitempty"`
	Sender      string `json:"sender,omitempty"`
	ShortenURLs bool   `json:"shorten_urls,omitempty"`
}

func (r *CreateMessageRequest) payload() createMessagePayload {
	payload := createMessagePayload{
		Recipient:   r.Recipient,
		Body:        r.Body,
		VisibleBody: r.VisibleBody,
		Validity:    r.Validity,
		CallbackURL: r.CallbackURL,
		Sender:      r.Sender,
		ShortenURLs: r.ShortenURLs,
	}

	if !r.ScheduleAt.IsZero() {
		payload.ScheduleAt = formatScheduleAt(r.ScheduleAt)
	}

	return payload
}

// MessageService sends and manages SMS messages: reach it as Client.Messages.
type MessageService struct {
	client *Client
}

// Create sends a message, now or at its ScheduleAt.
//
// Its ResponseMeta holds what is left of the account's daily sending limit after
// this message. Once the limit is reached it returns a *DailyLimitExceededError;
// a refused recipient (invalid, or opted out) a *ValidationError.
func (s *MessageService) Create(ctx context.Context, request *CreateMessageRequest) (*Response[Message], error) {
	if request == nil {
		return nil, errNilRequest("Messages.Create")
	}

	res, err := s.client.post(ctx, "/messages", request.payload())
	if err != nil {
		return nil, err
	}

	return single(res, readMessage, "message")
}

// Get retrieves a message by its ID.
func (s *MessageService) Get(ctx context.Context, messageID string) (*Response[Message], error) {
	id, err := pathID(messageID, "message ID")
	if err != nil {
		return nil, err
	}

	res, err := s.client.get(ctx, "/messages/"+id, nil)
	if err != nil {
		return nil, err
	}

	return single(res, readMessage, "message")
}

// List lists the messages, newest first, 50 a page; options may be nil.
func (s *MessageService) List(ctx context.Context, options *ListOptions) (*Page[Message], error) {
	var pageNumber int

	if options != nil {
		pageNumber = options.Page
	}

	query, err := pageQuery(pageNumber)
	if err != nil {
		return nil, err
	}

	res, err := s.client.get(ctx, "/messages", query)
	if err != nil {
		return nil, err
	}

	return page(res, readMessage)
}

// Delete cancels a scheduled message. A message already queued, sent or
// delivered cannot be: Delete then returns a *ForbiddenError.
func (s *MessageService) Delete(ctx context.Context, messageID string) error {
	id, err := pathID(messageID, "message ID")
	if err != nil {
		return err
	}

	_, err = s.client.delete(ctx, "/messages/"+id)

	return err
}
