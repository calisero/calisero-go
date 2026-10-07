package calisero_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func TestVerificationsListWithTheStatusFilter(t *testing.T) {
	items := []any{verificationData(object{"status": "verified", "verified_at": "2025-11-08T10:10:18.000000Z", "attempts": 1})}
	client, stub := clientAnswering(t, jsonResponse(200, pageOf(items, "verifications", 2, true)))

	page, err := client.Verifications.List(context.Background(), &calisero.VerificationListOptions{Page: 2, Status: calisero.VerificationStatusVerified})
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.URL.Path, "/api/v1/verifications", "path")
	equal(t, request.URL.RawQuery, "page=2&status=verified", "query")
	equal(t, page.Data[0].Status, calisero.VerificationStatusVerified, "status")
	equal(t, page.Data[0].Attempts, 1, "attempts")
	equal(t, page.HasNextPage(), true, "has a next page")
}

func TestVerificationsListWithoutOptions(t *testing.T) {
	client, stub := clientAnswering(t,
		jsonResponse(200, pageOf(nil, "verifications", 1, false)),
		jsonResponse(200, pageOf(nil, "verifications", 1, false)),
	)

	_, err := client.Verifications.List(context.Background(), nil)
	noError(t, err)
	equal(t, stub.lastRequest(t).URL.RawQuery, "", "query of nil options")

	_, err = client.Verifications.List(context.Background(), &calisero.VerificationListOptions{Status: calisero.VerificationStatusUnverified})
	noError(t, err)
	equal(t, stub.lastRequest(t).URL.RawQuery, "status=unverified", "query of a status alone")
}

func TestVerificationsListRefusesANegativePage(t *testing.T) {
	client, _ := clientAnswering(t)

	_, err := client.Verifications.List(context.Background(), &calisero.VerificationListOptions{Page: -1})

	equal(t, err.Error(), "calisero: the page must be a positive number, got -1", "error")
}

func TestVerificationsCreateWithABrand(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(201, object{"data": verificationData(nil)},
		"X-Daily-Limit", "1000",
		"X-Daily-Remaining", "871",
	))

	response, err := client.Verifications.Create(context.Background(), &calisero.CreateVerificationRequest{Phone: phone, Brand: "Calisero"})
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.Method, http.MethodPost, "method")
	equal(t, request.URL.Path, "/api/v1/verifications", "path")
	deepEqual(t, request.JSON(t), object{"phone": phone, "brand": "Calisero"}, "body")

	verification := response.Data
	equal(t, verification.ID, verificationID, "ID")
	equal(t, verification.Status, calisero.VerificationStatusUnverified, "status")
	equal(t, verification.Brand, "Calisero", "brand")
	equal(t, verification.Template, "", "template")
	equal(t, verification.ExpiresAt.Equal(time.Date(2025, 11, 8, 10, 12, 38, 0, time.UTC)), true, "expires at")
	equal(t, verification.VerifiedAt == nil, true, "verified at")
	equal(t, verification.Attempts, 0, "attempts")
	equal(t, verification.Expired, false, "expired")

	// The code's SMS counts towards the daily sending limit
	equal(t, intValue(response.ResponseMeta.DailyLimit), any(1000), "daily limit")
	equal(t, intValue(response.ResponseMeta.DailyRemaining), any(871), "daily remaining")
}

func TestVerificationsCreateWithATemplate(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(201, object{"data": verificationData(object{"brand": nil, "template": "Your code: {code}"})}))

	response, err := client.Verifications.Create(context.Background(), &calisero.CreateVerificationRequest{
		Phone:     phone,
		Template:  "Your code: {code}",
		ExpiresIn: 10,
	})
	noError(t, err)

	deepEqual(t, stub.lastRequest(t).JSON(t), object{"phone": phone, "template": "Your code: {code}", "expires_in": float64(10)}, "body")
	equal(t, response.Data.Brand, "", "brand")
	equal(t, response.Data.Template, "Your code: {code}", "template")
}

func TestVerificationsCreateRefusedByTheDailySendingLimit(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(429, dailyLimitBody, "Retry-After", "3600"))

	_, err := client.Verifications.Create(context.Background(), &calisero.CreateVerificationRequest{Phone: phone, Brand: "Calisero"})

	exceeded := errorAs[*calisero.DailyLimitExceededError](t, err)
	equal(t, exceeded.RetryAfter, time.Hour, "retry after")
}

func TestVerificationsGet(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, object{"data": verificationData(object{"expired": true, "attempts": 5})}))

	response, err := client.Verifications.Get(context.Background(), verificationID)
	noError(t, err)

	equal(t, stub.lastRequest(t).URL.Path, "/api/v1/verifications/"+verificationID, "path")
	equal(t, response.Data.Expired, true, "expired")
	equal(t, response.Data.Attempts, 5, "attempts")
}

func TestVerificationsValidate(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, object{"data": verificationData(object{
		"status":      "verified",
		"verified_at": "2025-11-08T10:10:18.000000Z",
		"attempts":    1,
	})}))

	response, err := client.Verifications.Validate(context.Background(), &calisero.VerificationCheckRequest{Phone: phone, Code: "abc123"})
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.Method, http.MethodPut, "method")
	equal(t, request.URL.Path, "/api/v1/verifications/validate", "path")
	deepEqual(t, request.JSON(t), object{"phone": phone, "code": "abc123"}, "body")
	equal(t, response.Data.Status, calisero.VerificationStatusVerified, "status")
	equal(t, response.Data.VerifiedAt.Equal(time.Date(2025, 11, 8, 10, 10, 18, 0, time.UTC)), true, "verified at")
}

func TestVerificationsValidateOfAWrongCode(t *testing.T) {
	for _, message := range []string{
		"The validation code is incorrect.",
		"The validation code has expired.",
		"The maximum number of verification attempts has been exceeded.",
	} {
		client, _ := clientAnswering(t, jsonResponse(422, object{"message": message, "errors": object{"code": []any{message}}}))

		_, err := client.Verifications.Validate(context.Background(), &calisero.VerificationCheckRequest{Phone: phone, Code: "WRONG1"})

		validation := errorAs[*calisero.ValidationError](t, err)
		equal(t, validation.Message, message, "message")
		deepEqual(t, validation.ValidationErrors["code"], []string{message}, "code")
	}
}

func TestVerificationsValidateWithoutAVerification(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(404, object{"message": "Resource not found!"}))

	_, err := client.Verifications.Validate(context.Background(), &calisero.VerificationCheckRequest{Phone: phone, Code: "ABC123"})

	isA[*calisero.NotFoundError](t, err)
}

func TestVerificationsRefuseInvalidArguments(t *testing.T) {
	client, stub := clientAnswering(t)
	ctx := context.Background()

	_, err := client.Verifications.Create(ctx, nil)
	equal(t, err.Error(), "calisero: Verifications.Create needs a request", "Create(nil)")

	_, err = client.Verifications.Validate(ctx, nil)
	equal(t, err.Error(), "calisero: Verifications.Validate needs a request", "Validate(nil)")

	_, err = client.Verifications.Get(ctx, "")
	equal(t, err.Error(), "calisero: the verification ID must not be empty", "Get(\"\")")

	equal(t, stub.requestCount(), 0, "requests sent")
}
