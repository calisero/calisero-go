package calisero_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func TestAccountsGetOfATypicalAccount(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, object{"data": accountData(nil)}, "X-Trace-Id", traceID))

	response, err := client.Accounts.Get(context.Background(), accountID)
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.Method, http.MethodGet, "method")
	equal(t, request.URL.Path, "/api/v1/accounts/"+accountID, "path")

	deepEqual(t, response.Data, calisero.Account{
		ID:             accountID,
		Code:           "ABC1234",
		Name:           "My Company",
		Description:    "Company description",
		FiscalCode:     "RO12345678",
		RegistryNumber: "J40/123456/2023",
		IBAN:           "RO49AAAA1B31007593840000",
		City:           "Bucharest",
		State:          "Bucharest",
		Country:        "RO",
		Address:        "Main Street 123",
		PostalCode:     ptr(123456),
		Email:          "contact@company.com",
		Phone:          "+40721***456",
		ContactPerson:  "John Doe",
		Credit:         100.5,
		Status:         calisero.AccountStatusActive,
		Sandbox:        false,
		DailyLimit:     ptr(1000),
		DailyRemaining: ptr(873),
		SentToday:      127,
		CreatedAt:      time.Date(2024, 9, 20, 12, 48, 55, 0, time.UTC),
	}, "account")
	equal(t, response.ResponseMeta.TraceID, traceID, "trace ID")
}

func TestAccountsGetOfASandboxAccountWithoutADailyLimit(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(200, object{"data": accountData(object{
		"sandbox":         true,
		"daily_limit":     nil,
		"daily_remaining": nil,
		"sent_today":      0,
	})}))

	response, err := client.Accounts.Get(context.Background(), accountID)
	noError(t, err)

	equal(t, response.Data.Sandbox, true, "sandbox")
	equal(t, response.Data.DailyLimit == nil, true, "daily limit")
	equal(t, response.Data.DailyRemaining == nil, true, "daily remaining")
	equal(t, response.Data.SentToday, 0, "sent today")
}

func TestAccountsGetOfAnInvalidAPIKey(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(401, object{"message": "Unauthenticated."}))

	_, err := client.Accounts.Get(context.Background(), accountID)

	equal(t, errorAs[*calisero.UnauthorizedError](t, err).Message, "Unauthenticated.", "message")
}

func TestAccountsGetOfAnotherAccount(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(403, object{"message": "You don't have permission to access this resource."}))

	_, err := client.Accounts.Get(context.Background(), accountID)

	equal(t, errorAs[*calisero.ForbiddenError](t, err).StatusCode, 403, "status")
}

func TestAccountsGetRefusesAnEmptyID(t *testing.T) {
	client, stub := clientAnswering(t)

	_, err := client.Accounts.Get(context.Background(), "")

	equal(t, err.Error(), "calisero: the account ID must not be empty", "error")
	equal(t, stub.requestCount(), 0, "requests sent")
}
