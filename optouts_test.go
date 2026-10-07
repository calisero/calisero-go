package calisero_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func TestOptOutsCreate(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(201, object{"data": optOutData(nil)}, "X-Trace-Id", traceID))

	response, err := client.OptOuts.Create(context.Background(), &calisero.CreateOptOutRequest{Phone: phone, Reason: "Replied STOP"})
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.Method, http.MethodPost, "method")
	equal(t, request.URL.Path, "/api/v1/opt-outs", "path")
	deepEqual(t, request.JSON(t), object{"phone": phone, "reason": "Replied STOP"}, "body")

	optOut := response.Data
	equal(t, optOut.ID, optOutID, "ID")
	equal(t, optOut.Phone, phone, "phone")
	equal(t, optOut.Reason, "Customer asked to stop receiving messages", "reason")
	equal(t, optOut.CreatedAt.Equal(time.Date(2025, 9, 19, 12, 1, 15, 0, time.UTC)), true, "created at")
	equal(t, response.ResponseMeta.TraceID, traceID, "trace ID")
}

func TestOptOutsCreateWithoutAReason(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(201, object{"data": optOutData(object{"reason": nil})}))

	response, err := client.OptOuts.Create(context.Background(), &calisero.CreateOptOutRequest{Phone: phone})
	noError(t, err)

	deepEqual(t, stub.lastRequest(t).JSON(t), object{"phone": phone}, "body")
	equal(t, response.Data.Reason, "", "reason")
}

func TestOptOutsCreateOfANumberAlreadyOptedOut(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(422, object{
		"message": "The phone has already been taken.",
		"errors":  object{"phone": []any{"The phone has already been taken."}},
	}))

	_, err := client.OptOuts.Create(context.Background(), &calisero.CreateOptOutRequest{Phone: phone})

	deepEqual(t, errorAs[*calisero.ValidationError](t, err).ValidationErrors, map[string][]string{"phone": {"The phone has already been taken."}}, "field errors")
}

func TestOptOutsGet(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, object{"data": optOutData(nil)}))

	response, err := client.OptOuts.Get(context.Background(), optOutID)
	noError(t, err)

	equal(t, stub.lastRequest(t).Method, http.MethodGet, "method")
	equal(t, stub.lastRequest(t).URL.Path, "/api/v1/opt-outs/"+optOutID, "path")
	equal(t, response.Data.ID, optOutID, "ID")
}

func TestOptOutsList(t *testing.T) {
	items := []any{optOutData(nil), optOutData(object{"id": "019999b5-0000-71cd-ae81-baf6eea9d86f", "reason": nil})}
	client, stub := clientAnswering(t, jsonResponse(200, pageOf(items, "opt-outs", 2, false)))

	page, err := client.OptOuts.List(context.Background(), &calisero.ListOptions{Page: 2})
	noError(t, err)

	equal(t, stub.lastRequest(t).URL.Path, "/api/v1/opt-outs", "path")
	equal(t, stub.lastRequest(t).URL.RawQuery, "page=2", "query")
	equal(t, len(page.Data), 2, "opt-outs")
	equal(t, page.Data[1].Reason, "", "no reason")
	equal(t, page.Meta.CurrentPage, 2, "current page")
	equal(t, page.HasNextPage(), false, "has a next page")
}

func TestOptOutsListTheFirstPage(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, pageOf(nil, "opt-outs", 1, false)))

	page, err := client.OptOuts.List(context.Background(), nil)
	noError(t, err)

	equal(t, stub.lastRequest(t).URL.RawQuery, "", "query")
	equal(t, len(page.Data), 0, "opt-outs")
}

func TestOptOutsUpdate(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, object{"data": optOutData(object{
		"reason":     "Updated reason",
		"updated_at": "2025-09-20T08:00:00.000000Z",
	})}))

	response, err := client.OptOuts.Update(context.Background(), optOutID, &calisero.UpdateOptOutRequest{Phone: phone, Reason: "Updated reason"})
	noError(t, err)

	request := stub.lastRequest(t)
	equal(t, request.Method, http.MethodPut, "method")
	equal(t, request.URL.Path, "/api/v1/opt-outs/"+optOutID, "path")
	deepEqual(t, request.JSON(t), object{"phone": phone, "reason": "Updated reason"}, "body")
	equal(t, response.Data.Reason, "Updated reason", "reason")
	equal(t, response.Data.UpdatedAt.Equal(time.Date(2025, 9, 20, 8, 0, 0, 0, time.UTC)), true, "updated at")
}

func TestOptOutsDelete(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, []any{}))

	noError(t, client.OptOuts.Delete(context.Background(), optOutID))

	equal(t, stub.lastRequest(t).Method, http.MethodDelete, "method")
	equal(t, stub.lastRequest(t).URL.Path, "/api/v1/opt-outs/"+optOutID, "path")
}

func TestOptOutsOfAnOptOutThatDoesNotExist(t *testing.T) {
	client, _ := clientAnswering(t, jsonResponse(404, object{"message": "Resource not found!"}))

	err := client.OptOuts.Delete(context.Background(), optOutID)

	isA[*calisero.NotFoundError](t, err)
}

func TestOptOutsRefuseInvalidArguments(t *testing.T) {
	client, stub := clientAnswering(t)
	ctx := context.Background()

	_, err := client.OptOuts.Create(ctx, nil)
	equal(t, err.Error(), "calisero: OptOuts.Create needs a request", "Create(nil)")

	_, err = client.OptOuts.Get(ctx, "")
	equal(t, err.Error(), "calisero: the opt-out ID must not be empty", "Get(\"\")")

	_, err = client.OptOuts.Update(ctx, "", &calisero.UpdateOptOutRequest{Phone: phone})
	equal(t, err.Error(), "calisero: the opt-out ID must not be empty", "Update(\"\")")

	_, err = client.OptOuts.Update(ctx, optOutID, nil)
	equal(t, err.Error(), "calisero: OptOuts.Update needs a request", "Update(nil)")

	err = client.OptOuts.Delete(ctx, " ")
	equal(t, err.Error(), "calisero: the opt-out ID must not be empty", "Delete(\" \")")

	_, err = client.OptOuts.List(ctx, &calisero.ListOptions{Page: -2})
	equal(t, err.Error(), "calisero: the page must be a positive number, got -2", "List(-2)")

	equal(t, stub.requestCount(), 0, "requests sent")
}
