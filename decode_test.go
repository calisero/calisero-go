package calisero_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

// readMessage is the message the client reads from data, or its error.
func readMessage(t *testing.T, data object) (calisero.Message, error) {
	t.Helper()

	client, _ := clientAnswering(t, jsonResponse(200, object{"data": data}))

	response, err := client.Messages.Get(context.Background(), messageID)
	if err != nil {
		return calisero.Message{}, err
	}

	return response.Data, nil
}

// readAccount is the account the client reads from data, or its error.
func readAccount(t *testing.T, data object) (calisero.Account, error) {
	t.Helper()

	client, _ := clientAnswering(t, jsonResponse(200, object{"data": data}))

	response, err := client.Accounts.Get(context.Background(), accountID)
	if err != nil {
		return calisero.Account{}, err
	}

	return response.Data, nil
}

// unexpectedMessage is the message of the error the client returns for data.
func unexpectedMessage(t *testing.T, err error) string {
	t.Helper()

	return errorAs[*calisero.APIError](t, err).Message
}

func TestAMessageWithEveryField(t *testing.T) {
	message, err := readMessage(t, messageData(object{
		"status":         "delivered",
		"sent_at":        "2025-02-06T10:18:45.000000Z",
		"delivered_at":   "2025-02-06T10:18:49.000000Z",
		"shortened_urls": []any{shortenedLinkData(nil)},
	}))
	noError(t, err)

	deepEqual(t, message, calisero.Message{
		ID:          messageID,
		Recipient:   phone,
		Body:        "Hello from Calisero!",
		Parts:       1,
		CreatedAt:   time.Date(2025, 2, 6, 10, 18, 43, 0, time.UTC),
		ScheduledAt: ptr(time.Date(2025, 2, 6, 10, 18, 43, 0, time.UTC)),
		SentAt:      ptr(time.Date(2025, 2, 6, 10, 18, 45, 0, time.UTC)),
		DeliveredAt: ptr(time.Date(2025, 2, 6, 10, 18, 49, 0, time.UTC)),
		CallbackURL: "https://yourapp.com/webhooks/calisero",
		Status:      calisero.MessageStatusDelivered,
		Sender:      "CALISERO",
		ShortenedURLs: []calisero.ShortenedLink{{
			ID:            "019adfbb-40a1-71ee-bcb5-8d551b8cfdae",
			OriginalLink:  "https://yourstore.com/orders/12345/tracking",
			ShortenedLink: "https://calisero.ro/s/ghJKPV",
			ClickCount:    3,
			LastClick:     ptr(time.Date(2025, 12, 2, 16, 1, 12, 0, time.UTC)),
			CreatedAt:     time.Date(2025, 12, 2, 15, 43, 2, 0, time.UTC),
		}},
	}, "message")
}

func TestAMinimalMessage(t *testing.T) {
	message, err := readMessage(t, object{
		"id":         messageID,
		"recipient":  phone,
		"body":       "Hello!",
		"parts":      1,
		"created_at": "2025-02-06T10:18:43.000000Z",
		"status":     "scheduled",
	})
	noError(t, err)

	equal(t, message.ScheduledAt == nil && message.SentAt == nil && message.DeliveredAt == nil, true, "no dates")
	equal(t, message.CallbackURL, "", "callback URL")
	equal(t, message.Sender, "", "default sender")
	equal(t, message.ShortenedURLs != nil && len(message.ShortenedURLs) == 0, true, "no shortened links")
}

func TestAMessageOfAFutureStatusIsRead(t *testing.T) {
	message, err := readMessage(t, messageData(object{"status": "expired"}))
	noError(t, err)

	equal(t, message.Status, calisero.MessageStatus("expired"), "status")
}

func TestAMalformedMessageIsRefusedNamingTheField(t *testing.T) {
	cases := map[string]struct {
		data    object
		message string
	}{
		"no id":           {messageData(object{"id": absent}), `invalid message: "id" must be a string, got nothing`},
		"parts as text":   {messageData(object{"parts": "1"}), `invalid message: "parts" must be an integer, got a string`},
		"parts not whole": {messageData(object{"parts": 1.5}), `invalid message: "parts" must be an integer, got a number`},
		"a bad date":      {messageData(object{"created_at": "yesterday"}), `invalid message: "created_at" must be a date-time, got a string`},
		"no date":         {messageData(object{"created_at": nil}), `invalid message: "created_at" must be a date-time, got null`},
		"a status list":   {messageData(object{"status": []any{"sent"}}), `invalid message: "status" must be a string, got an array`},
		"a bad link": {
			messageData(object{"shortened_urls": []any{shortenedLinkData(nil), shortenedLinkData(object{"shortened_link": nil})}}),
			`invalid message.shortened_urls[1]: "shortened_link" must be a string, got null`,
		},
		"a link not an object": {
			messageData(object{"shortened_urls": []any{"https://calisero.ro/s/ghJKPV"}}),
			`invalid message.shortened_urls[0]: expected an object, got a string`,
		},
	}

	for name, tc := range cases {
		_, err := readMessage(t, tc.data)

		equal(t, unexpectedMessage(t, err), "Unexpected API response: "+tc.message, name)
	}
}

func TestOptionalFieldsOfTheWrongTypeReadAsEmpty(t *testing.T) {
	message, err := readMessage(t, messageData(object{
		"sender":         42,
		"callback_url":   false,
		"sent_at":        "not a date",
		"delivered_at":   1700000000,
		"shortened_urls": "none",
	}))
	noError(t, err)

	equal(t, message.Sender, "", "sender")
	equal(t, message.CallbackURL, "", "callback URL")
	equal(t, message.SentAt == nil, true, "sent at")
	equal(t, message.DeliveredAt == nil, true, "delivered at")
	equal(t, len(message.ShortenedURLs), 0, "shortened links")
}

func TestAShortenedLinkWithoutClicks(t *testing.T) {
	message, err := readMessage(t, messageData(object{"shortened_urls": []any{shortenedLinkData(object{"click_count": absent, "last_click": nil})}}))
	noError(t, err)

	link := message.ShortenedURLs[0]
	equal(t, link.ClickCount, 0, "clicks")
	equal(t, link.LastClick == nil, true, "last click")
}

func TestAMinimalAccount(t *testing.T) {
	account, err := readAccount(t, accountData(object{
		"description":     nil,
		"fiscal_code":     nil,
		"registry_number": nil,
		"iban":            nil,
		"city":            nil,
		"state":           nil,
		"country":         nil,
		"address":         nil,
		"postal_code":     nil,
		"email":           nil,
		"phone":           nil,
		"contact_person":  nil,
		"daily_limit":     nil,
		"daily_remaining": nil,
		"sent_today":      absent,
	}))
	noError(t, err)

	equal(t, account.Description+account.FiscalCode+account.RegistryNumber+account.IBAN, "", "company details")
	equal(t, account.City+account.State+account.Country+account.Address, "", "address")
	equal(t, account.Email+account.Phone+account.ContactPerson, "", "contact")
	equal(t, account.PostalCode == nil, true, "postal code")
	equal(t, account.DailyLimit == nil && account.DailyRemaining == nil, true, "no daily limit")
	equal(t, account.SentToday, 0, "sent today")
}

func TestAnAccountCreditMayComeAsADecimalString(t *testing.T) {
	account, err := readAccount(t, accountData(object{"credit": "100.50"}))
	noError(t, err)

	equal(t, account.Credit, 100.5, "credit")
}

func TestAnAccountWithoutACreditIsRefused(t *testing.T) {
	_, err := readAccount(t, accountData(object{"credit": "a lot"}))

	equal(t, unexpectedMessage(t, err), `Unexpected API response: invalid account: "credit" must be a number, got a string`, "message")
}

func TestAnAccountPostalCodeThatIsNotANumberIsLeftOut(t *testing.T) {
	account, err := readAccount(t, accountData(object{"postal_code": "010011"}))
	noError(t, err)

	equal(t, account.PostalCode == nil, true, "postal code")
}

func TestVerifications(t *testing.T) {
	cases := map[string]struct {
		data  object
		check func(calisero.Verification)
	}{
		"unverified": {verificationData(nil), func(v calisero.Verification) {
			equal(t, v.Status, calisero.VerificationStatusUnverified, "status")
			equal(t, v.VerifiedAt == nil, true, "verified at")
		}},
		"verified": {verificationData(object{"status": "verified", "verified_at": "2025-11-08T10:10:18.000000Z", "attempts": 1}), func(v calisero.Verification) {
			equal(t, v.Status, calisero.VerificationStatusVerified, "status")
			equal(t, v.VerifiedAt.Equal(time.Date(2025, 11, 8, 10, 10, 18, 0, time.UTC)), true, "verified at")
		}},
		"expired": {verificationData(object{"expired": true, "attempts": 5}), func(v calisero.Verification) {
			equal(t, v.Expired, true, "expired")
			equal(t, v.Attempts, 5, "attempts")
		}},
		"with a template": {verificationData(object{"brand": nil, "template": "Code: {code}"}), func(v calisero.Verification) {
			equal(t, v.Brand, "", "brand")
			equal(t, v.Template, "Code: {code}", "template")
		}},
	}

	for name, tc := range cases {
		client, _ := clientAnswering(t, jsonResponse(200, object{"data": tc.data}))

		response, err := client.Verifications.Get(context.Background(), verificationID)
		noError(t, err)

		t.Run(name, func(*testing.T) { tc.check(response.Data) })
	}
}

func TestAVerificationWithoutTheExpiredFlagIsRefused(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(200, object{"data": verificationData(object{"expired": "no"})}))

	_, err := client.Verifications.Get(context.Background(), verificationID)

	equal(t, unexpectedMessage(t, err), `Unexpected API response: invalid verification: "expired" must be a boolean, got a string`, "message")
}

func TestAnOptOutWithoutAPhoneIsRefused(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(200, object{"data": optOutData(object{"phone": absent})}))

	_, err := client.OptOuts.Get(context.Background(), optOutID)

	equal(t, unexpectedMessage(t, err), `Unexpected API response: invalid opt-out: "phone" must be a string, got nothing`, "message")
}

func TestUnmarshalJSONRefusesWhatIsNotJSON(t *testing.T) {
	// encoding/json checks the syntax before it calls UnmarshalJSON; a direct call does not
	for _, resource := range []json.Unmarshaler{&calisero.Message{}, &calisero.Account{}, &calisero.OptOut{}, &calisero.Verification{}, &calisero.ShortenedLink{}} {
		if err := resource.UnmarshalJSON([]byte(`{"id":`)); err == nil {
			t.Errorf("%T read truncated JSON", resource)
		}
	}
}

func TestResponseMetaIgnoresHeadersThatAreNotCounts(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(201, object{"data": messageData(nil)},
		"x-trace-id", " "+traceID+" ",
		"x-ratelimit-limit", "lots",
		"x-ratelimit-remaining", "-3",
		"x-daily-limit", "1000.5",
		"x-daily-remaining", "",
	))

	response, err := client.Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: phone, Body: "Hello!"})
	noError(t, err)

	deepEqual(t, response.ResponseMeta, calisero.ResponseMeta{TraceID: traceID}, "response meta")
}

func TestResourcesAreReadWithJSONUnmarshal(t *testing.T) {
	var message calisero.Message
	noError(t, json.Unmarshal(mustJSON(t, messageData(object{"shortened_urls": []any{shortenedLinkData(nil)}})), &message))
	equal(t, message.ID, messageID, "message")
	equal(t, message.ShortenedURLs[0].ClickCount, 3, "shortened link")

	var account calisero.Account
	noError(t, json.Unmarshal(mustJSON(t, accountData(object{"credit": "12.5"})), &account))
	equal(t, account.Credit, 12.5, "account")

	var optOut calisero.OptOut
	noError(t, json.Unmarshal(mustJSON(t, optOutData(nil)), &optOut))
	equal(t, optOut.ID, optOutID, "opt-out")

	var verification calisero.Verification
	noError(t, json.Unmarshal(mustJSON(t, verificationData(nil)), &verification))
	equal(t, verification.ID, verificationID, "verification")

	var link calisero.ShortenedLink
	noError(t, json.Unmarshal(mustJSON(t, shortenedLinkData(nil)), &link))
	equal(t, link.ShortenedLink, "https://calisero.ro/s/ghJKPV", "shortened link")
}

func TestJSONUnmarshalRefusesAMalformedResource(t *testing.T) {
	var message calisero.Message
	err := json.Unmarshal(mustJSON(t, messageData(object{"id": absent})), &message)

	if err == nil || err.Error() != `invalid message: "id" must be a string, got nothing` {
		t.Fatalf("got %v", err)
	}

	var account calisero.Account
	if err := json.Unmarshal([]byte(`"an account"`), &account); err == nil {
		t.Fatal("a string was read as an account")
	}
}

func TestJSONNullLeavesAResourceAsItIs(t *testing.T) {
	var holder struct {
		Message *calisero.Message `json:"message"`
		OptOut  calisero.OptOut   `json:"opt_out"`
	}

	noError(t, json.Unmarshal([]byte(`{"message": null, "opt_out": null}`), &holder))

	equal(t, holder.Message == nil, true, "message")
	equal(t, holder.OptOut.ID, "", "opt-out")
}

func TestResourcesSurviveAJSONRoundTrip(t *testing.T) {
	message, err := readMessage(t, messageData(object{"shortened_urls": []any{shortenedLinkData(nil)}}))
	noError(t, err)

	var decoded calisero.Message
	noError(t, json.Unmarshal(mustJSON(t, message), &decoded))
	deepEqual(t, decoded, message, "message")

	account, err := readAccount(t, accountData(nil))
	noError(t, err)

	var decodedAccount calisero.Account
	noError(t, json.Unmarshal(mustJSON(t, account), &decodedAccount))
	deepEqual(t, decodedAccount, account, "account")
}

func TestResourcesAreWrittenWithTheAPIFieldNames(t *testing.T) {
	message, err := readMessage(t, messageData(nil))
	noError(t, err)

	var written object
	noError(t, json.Unmarshal(mustJSON(t, message), &written))

	for key := range messageData(nil) {
		if _, ok := written[key]; !ok {
			t.Errorf("the field %q is not written", key)
		}
	}

	equal(t, written["sent_at"], nil, "a date the message has not, as null")
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	noError(t, err)

	return data
}
