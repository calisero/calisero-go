package calisero_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func TestMessagesCreateSendsEveryOption(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(201, object{"data": messageData(object{"status": "scheduled"})}))

	response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{
		Recipient:   phone,
		Body:        "Your code is 123456. Manage alerts: https://yourapp.com/alerts",
		VisibleBody: "Your code is ******. Manage alerts: https://yourapp.com/alerts",
		Validity:    24,
		ScheduleAt:  time.Date(2026, 12, 25, 8, 0, 0, 0, time.UTC),
		CallbackURL: "https://yourapp.com/webhooks/calisero",
		Sender:      "Calisero",
		ShortenURLs: true,
	})
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.Method, http.MethodPost, "method")
	equal(t, request.URL.Path, "/api/v1/messages", "path")
	deepEqual(t, request.JSON(t), object{
		"recipient":    phone,
		"body":         "Your code is 123456. Manage alerts: https://yourapp.com/alerts",
		"visible_body": "Your code is ******. Manage alerts: https://yourapp.com/alerts",
		"validity":     float64(24),
		"schedule_at":  "2026-12-25 10:00:00",
		"callback_url": "https://yourapp.com/webhooks/calisero",
		"sender":       "Calisero",
		"shorten_urls": true,
	}, "body")
	equal(t, response.Data.ID, messageID, "message ID")
	equal(t, response.Data.Status, calisero.MessageStatusScheduled, "status")
}

func TestMessagesCreateSendsTheRecipientAndTheBodyOfAMinimalRequest(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(201, object{"data": messageData(nil)}))

	_, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: phone, Body: "Hello!"})
	noError(t, err)

	deepEqual(t, stub.lastRequest(t).JSON(t), object{"recipient": phone, "body": "Hello!"}, "body")
}

func TestMessagesCreateSendsTheRequiredFieldsEvenEmpty(t *testing.T) {
	// The API says what is missing (422): the client sends what it is given
	client, stub := clientAnswering(t, jsonResponse(422, object{
		"message": "The recipient field is required.",
		"errors":  object{"recipient": []any{"The recipient field is required."}},
	}))

	_, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Body: "Hello!"})

	isA[*calisero.ValidationError](t, err)
	deepEqual(t, stub.lastRequest(t).JSON(t), object{"recipient": "", "body": "Hello!"}, "body")
}

func TestMessagesCreateSchedulesInRomaniaTime(t *testing.T) {
	bucharestless := time.FixedZone("UTC-5", -5*60*60)
	cases := map[string]struct {
		at   time.Time
		want string
	}{
		"summer time, UTC+3":             {time.Date(2026, 7, 15, 9, 30, 0, 0, time.UTC), "2026-07-15 12:30:00"},
		"winter time, UTC+2":             {time.Date(2026, 1, 15, 9, 30, 5, 0, time.UTC), "2026-01-15 11:30:05"},
		"midnight, on the next day":      {time.Date(2026, 3, 1, 22, 0, 0, 0, time.UTC), "2026-03-02 00:00:00"},
		"before the switch to summer":    {time.Date(2026, 3, 29, 0, 59, 59, 0, time.UTC), "2026-03-29 02:59:59"},
		"after the switch to summer":     {time.Date(2026, 3, 29, 1, 0, 0, 0, time.UTC), "2026-03-29 04:00:00"},
		"after the switch back":          {time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC), "2026-10-25 03:00:00"},
		"a time in another zone":         {time.Date(2026, 12, 24, 20, 15, 0, 0, bucharestless), "2026-12-25 03:15:00"},
		"sub-second precision truncated": {time.Date(2026, 5, 4, 10, 0, 0, 999999999, time.UTC), "2026-05-04 13:00:00"},
	}

	for name, tc := range cases {
		client, stub := clientAnswering(t, jsonResponse(201, object{"data": messageData(nil)}))

		_, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: phone, Body: "Hello!", ScheduleAt: tc.at})
		noError(t, err)

		equal(t, stub.lastRequest(t).JSON(t)["schedule_at"], any(tc.want), name)
	}
}

func TestMessagesCreateReadsTheAnswerAndItsHeaders(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(201, object{"data": messageData(object{
		"shortened_urls": []any{shortenedLinkData(nil), shortenedLinkData(object{"click_count": 0, "last_click": nil})},
	})},
		"X-Trace-Id", traceID,
		"X-RateLimit-Limit", "240",
		"X-RateLimit-Remaining", "239",
		"X-Daily-Limit", "1000",
		"X-Daily-Remaining", "872",
	))

	response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: phone, Body: "Hello!", ShortenURLs: true})
	noError(t, err)

	meta := response.ResponseMeta
	equal(t, meta.TraceID, traceID, "trace ID")
	equal(t, intValue(meta.RateLimitLimit), any(240), "rate limit")
	equal(t, intValue(meta.RateLimitRemaining), any(239), "rate limit remaining")
	equal(t, intValue(meta.DailyLimit), any(1000), "daily limit")
	equal(t, intValue(meta.DailyRemaining), any(872), "daily remaining")

	links := response.Data.ShortenedURLs
	equal(t, len(links), 2, "shortened links")
	equal(t, links[0].OriginalLink, "https://yourstore.com/orders/12345/tracking", "original link")
	equal(t, links[0].ShortenedLink, "https://calisero.ro/s/ghJKPV", "shortened link")
	equal(t, links[0].ClickCount, 3, "clicks")
	equal(t, links[0].LastClick.Equal(time.Date(2025, 12, 2, 16, 1, 12, 0, time.UTC)), true, "last click")
	equal(t, links[1].LastClick == nil, true, "never clicked")
}

func TestMessagesCreateWithoutADailyLimit(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(201, object{"data": messageData(nil)}, "X-Trace-Id", traceID))

	response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: phone, Body: "Hello!"})
	noError(t, err)

	equal(t, response.ResponseMeta.DailyLimit == nil, true, "daily limit")
	equal(t, response.ResponseMeta.DailyRemaining == nil, true, "daily remaining")
}

func TestMessagesCreateRefusedByTheDailySendingLimit(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(429, dailyLimitBody, "Retry-After", "21600", "X-Daily-Remaining", "0"))

	_, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: phone, Body: "Hello!"})

	exceeded := errorAs[*calisero.DailyLimitExceededError](t, err)
	equal(t, exceeded.DailyLimit, 1000, "daily limit")
	equal(t, exceeded.DailyRemaining, 0, "daily remaining")
}

func TestMessagesCreateRefusesAnOptedOutRecipient(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(422, object{
		"message": "The recipient has opted out of receiving messages.",
		"errors":  object{"recipient": []any{"The recipient has opted out of receiving messages."}},
	}))

	_, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: phone, Body: "Special offer!"})

	validation := errorAs[*calisero.ValidationError](t, err)
	deepEqual(t, validation.ValidationErrors["recipient"], []string{"The recipient has opted out of receiving messages."}, "recipient")
}

func TestMessagesCreateNeedsARequest(t *testing.T) {
	client, stub := clientAnswering(t)

	_, err := client.Messages.Create(context.Background(), nil)

	if err == nil || err.Error() != "calisero: Messages.Create needs a request" {
		t.Fatalf("got %v", err)
	}

	equal(t, stub.requestCount(), 0, "requests sent")
}

func TestMessagesGet(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, object{"data": messageData(object{
		"status":       "delivered",
		"sent_at":      "2025-02-06T10:18:45.000000Z",
		"delivered_at": "2025-02-06T10:18:49.000000Z",
	})}, "X-Trace-Id", traceID))

	response, err := client.Messages.Get(context.Background(), messageID)
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.Method, http.MethodGet, "method")
	equal(t, request.URL.Path, "/api/v1/messages/"+messageID, "path")
	equal(t, request.URL.RawQuery, "", "query")

	message := response.Data
	equal(t, message.Status, calisero.MessageStatusDelivered, "status")
	equal(t, message.SentAt.Equal(time.Date(2025, 2, 6, 10, 18, 45, 0, time.UTC)), true, "sent at")
	equal(t, message.DeliveredAt.Equal(time.Date(2025, 2, 6, 10, 18, 49, 0, time.UTC)), true, "delivered at")
	equal(t, response.ResponseMeta.TraceID, traceID, "trace ID")
}

func TestMessagesGetEscapesTheID(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, object{"data": messageData(nil)}))

	_, err := client.Messages.Get(context.Background(), "a/b c?d")
	noError(t, err)

	equal(t, stub.lastRequest(t).URL.EscapedPath(), "/api/v1/messages/a%2Fb%20c%3Fd", "path")
}

func TestMessagesRefuseAnEmptyID(t *testing.T) {
	client, stub := clientAnswering(t)

	for _, id := range []string{"", "  "} {
		_, err := client.Messages.Get(context.Background(), id)

		if err == nil || err.Error() != "calisero: the message ID must not be empty" {
			t.Errorf("Get(%q): got %v", id, err)
		}

		if err := client.Messages.Delete(context.Background(), id); err == nil || err.Error() != "calisero: the message ID must not be empty" {
			t.Errorf("Delete(%q): got %v", id, err)
		}
	}

	equal(t, stub.requestCount(), 0, "requests sent")
}

func TestMessagesGetOfAMessageThatDoesNotExist(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(404, object{"message": "Resource not found!", "trace_id": traceID}))

	_, err := client.Messages.Get(context.Background(), "00000000-0000-0000-0000-000000000000")

	equal(t, errorAs[*calisero.NotFoundError](t, err).Message, "Resource not found!", "message")
}

func TestMessagesListTheFirstPage(t *testing.T) {
	items := []any{messageData(nil), messageData(object{"id": "019961db-14a7-7348-963f-5a7a789a969f"})}
	client, stub := clientAnswering(t, jsonResponse(200, pageOf(items, "messages", 1, true), "X-Trace-Id", traceID, "X-RateLimit-Remaining", "238"))

	page, err := client.Messages.List(context.Background(), nil)
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.URL.Path, "/api/v1/messages", "path")
	equal(t, request.URL.RawQuery, "", "query")

	equal(t, len(page.Data), 2, "messages")
	equal(t, page.Data[1].ID, "019961db-14a7-7348-963f-5a7a789a969f", "second message")
	equal(t, page.Meta.CurrentPage, 1, "current page")
	equal(t, page.Meta.PerPage, 50, "per page")
	equal(t, intValue(page.Meta.From), any(1), "from")
	equal(t, intValue(page.Meta.To), any(2), "to")
	equal(t, page.Meta.Path, "https://rest.calisero.ro/api/v1/messages", "path")
	equal(t, page.Links.First, "https://rest.calisero.ro/api/v1/messages?page=1", "first")
	equal(t, page.Links.Last, "", "last")
	equal(t, page.Links.Prev, "", "prev")
	equal(t, page.Links.Next, "https://rest.calisero.ro/api/v1/messages?page=2", "next")
	equal(t, page.HasNextPage(), true, "has a next page")
	equal(t, page.ResponseMeta.TraceID, traceID, "trace ID")
	equal(t, intValue(page.ResponseMeta.RateLimitRemaining), any(238), "rate limit remaining")
}

func TestMessagesListALaterPage(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, pageOf([]any{messageData(nil)}, "messages", 3, false)))

	page, err := client.Messages.List(context.Background(), &calisero.ListOptions{Page: 3})
	noError(t, err)

	equal(t, stub.lastRequest(t).URL.RawQuery, "page=3", "query")
	equal(t, page.Links.Prev, "https://rest.calisero.ro/api/v1/messages?page=2", "prev")
	equal(t, page.HasNextPage(), false, "has a next page")
	equal(t, intValue(page.Meta.From), any(101), "from")
}

func TestMessagesListTheFirstPageWithoutAQuery(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, pageOf(nil, "messages", 1, false)))

	page, err := client.Messages.List(context.Background(), &calisero.ListOptions{Page: 1})
	noError(t, err)

	equal(t, stub.lastRequest(t).URL.RawQuery, "", "query")
	equal(t, page.Data != nil && len(page.Data) == 0, true, "an empty page has an empty list")
	equal(t, page.Meta.From == nil && page.Meta.To == nil, true, "from and to")
}

func TestMessagesListRefusesANegativePage(t *testing.T) {
	client, stub := clientAnswering(t)

	_, err := client.Messages.List(context.Background(), &calisero.ListOptions{Page: -1})

	if err == nil || err.Error() != "calisero: the page must be a positive number, got -1" {
		t.Fatalf("got %v", err)
	}

	equal(t, stub.requestCount(), 0, "requests sent")
}

func TestMessagesListOfAMalformedPage(t *testing.T) {
	cases := map[string]struct {
		body    object
		message string
	}{
		"no data":         {object{"links": object{}, "meta": object{}}, `invalid page: "data" must be an array, got nothing`},
		"a bad message":   {pageOf([]any{messageData(nil), messageData(object{"parts": "1"})}, "messages", 1, false), `invalid page.data[1]: "parts" must be an integer, got a string`},
		"no links":        {object{"data": []any{}, "meta": pageOf(nil, "messages", 1, false)["meta"]}, `invalid page: "links" must be an object, got nothing`},
		"a bad meta":      {with(pageOf(nil, "messages", 1, false), object{"meta": object{"current_page": 1}}), `invalid page.meta: "path" must be a string, got nothing`},
		"not a page":      {object{"data": "messages"}, `invalid page: "data" must be an array, got a string`},
		"links not links": {with(pageOf(nil, "messages", 1, false), object{"links": []any{}}), `invalid page: "links" must be an object, got an array`},
	}

	for name, tc := range cases {
		client, _ := clientAnswering(t, jsonResponse(200, tc.body))

		_, err := client.Messages.List(context.Background(), nil)

		equal(t, errorAs[*calisero.APIError](t, err).Message, "Unexpected API response: "+tc.message, name)
	}
}

func TestMessagesDelete(t *testing.T) {
	// The API answers a cancellation with [], {} or nothing
	for _, answer := range []*http.Response{jsonResponse(200, []any{}), jsonResponse(200, object{}), rawResponse(200, "")} {
		client, stub := clientAnswering(t, answer)

		noError(t, client.Messages.Delete(context.Background(), messageID))

		request := stub.lastRequest(t)
		equal(t, request.Method, http.MethodDelete, "method")
		equal(t, request.URL.Path, "/api/v1/messages/"+messageID, "path")
		equal(t, request.Body, "", "body")
	}
}

func TestMessagesDeleteOfAMessageAlreadySent(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(403, object{
		"message": "You are not authorized to delete this resource or the resource has been queued, sent or delivered",
	}))

	err := client.Messages.Delete(context.Background(), messageID)

	forbidden := errorAs[*calisero.ForbiddenError](t, err)
	equal(t, strings.HasSuffix(forbidden.Message, "queued, sent or delivered"), true, "message")
}

func TestMessagesDeleteWithoutAnAnswer(t *testing.T) {
	client, _ := clientAnswering(t, errors.New("connection refused"))

	err := client.Messages.Delete(context.Background(), messageID)

	isA[*calisero.TransportError](t, err)
}
