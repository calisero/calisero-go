package calisero_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

// The client checked against the API's OpenAPI document (resources/openApi/api-v1.json,
// the one published at https://docs.calisero.ro/open-api): every operation is
// implemented and reads the documented example answers, every documented field is
// read or sent, and every documented error maps to its error type.

type openAPISchema struct {
	Ref        string                   `json:"$ref"`
	Type       string                   `json:"type"`
	Properties map[string]openAPISchema `json:"properties"`
	Required   []string                 `json:"required"`
}

type openAPIMediaType struct {
	Schema   *openAPISchema  `json:"schema"`
	Example  json.RawMessage `json:"example"`
	Examples map[string]struct {
		Value json.RawMessage `json:"value"`
	} `json:"examples"`
}

type openAPIResponse struct {
	Ref     string                      `json:"$ref"`
	Content map[string]openAPIMediaType `json:"content"`
}

type openAPIOperation struct {
	OperationID string `json:"operationId"`
	RequestBody *struct {
		Content map[string]openAPIMediaType `json:"content"`
	} `json:"requestBody"`
	Responses map[string]openAPIResponse `json:"responses"`
	// The requests Calisero sends because of the call, the delivery webhook among them:
	// by callback name, then runtime expression, then method
	Callbacks map[string]map[string]map[string]openAPIOperation `json:"callbacks"`
}

type openAPIDocument struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths      map[string]map[string]openAPIOperation `json:"paths"`
	Components struct {
		Schemas   map[string]openAPISchema   `json:"schemas"`
		Responses map[string]openAPIResponse `json:"responses"`
	} `json:"components"`
}

func loadOpenAPI(t *testing.T) *openAPIDocument {
	t.Helper()

	data, err := os.ReadFile("resources/openApi/api-v1.json")
	noError(t, err)

	var document openAPIDocument
	noError(t, json.Unmarshal(data, &document))

	return &document
}

func (d *openAPIDocument) response(response openAPIResponse) openAPIResponse {
	if response.Ref == "" {
		return response
	}

	return d.Components.Responses[strings.TrimPrefix(response.Ref, "#/components/responses/")]
}

func (d *openAPIDocument) schema(schema openAPISchema) openAPISchema {
	if schema.Ref == "" {
		return schema
	}

	return d.Components.Schemas[strings.TrimPrefix(schema.Ref, "#/components/schemas/")]
}

// examples are the documented examples of an answer.
func (d *openAPIDocument) examples(response openAPIResponse) []json.RawMessage {
	media, ok := d.response(response).Content["application/json"]
	if !ok {
		return nil
	}

	if len(media.Example) > 0 {
		return []json.RawMessage{media.Example}
	}

	examples := make([]json.RawMessage, 0, len(media.Examples))

	for _, name := range sortedKeys(media.Examples) {
		examples = append(examples, media.Examples[name].Value)
	}

	return examples
}

// successExamples are the examples of a successful answer; for one documented by
// its schema alone, those of the other answers with the same schema.
func (d *openAPIDocument) successExamples(response openAPIResponse) []json.RawMessage {
	own := d.examples(response)
	media := d.response(response).Content["application/json"]

	if len(own) > 0 || media.Schema == nil || media.Schema.Ref == "" {
		return own
	}

	var examples []json.RawMessage

	for _, operation := range d.operations() {
		for _, other := range operation.Responses {
			if otherMedia := d.response(other).Content["application/json"]; otherMedia.Schema != nil && otherMedia.Schema.Ref == media.Schema.Ref {
				examples = append(examples, d.examples(other)...)
			}
		}
	}

	return examples
}

// documentedOperation is an operation of the API, with its method and path.
type documentedOperation struct {
	openAPIOperation

	method string
	path   string
}

// operations are the operations of the API, by path and method. The delivery webhook, which
// Calisero sends rather than answers, is a callback of POST /messages, not one of them.
func (d *openAPIDocument) operations() []documentedOperation {
	var operations []documentedOperation

	for _, path := range sortedKeys(d.Paths) {
		for _, method := range sortedKeys(d.Paths[path]) {
			operations = append(operations, documentedOperation{d.Paths[path][method], strings.ToUpper(method), path})
		}
	}

	return operations
}

// success is the status and the examples of an operation's successful answer.
func (d *openAPIDocument) success(operation documentedOperation) (int, []json.RawMessage) {
	for _, status := range sortedKeys(operation.Responses) {
		if strings.HasPrefix(status, "2") {
			var code int
			_, _ = fmt.Sscan(status, &code)

			return code, d.successExamples(operation.Responses[status])
		}
	}

	return 0, nil
}

// documentedData is the data of the first documented example of an answer.
func (d *openAPIDocument) documentedData(t *testing.T, path, method, status string) json.RawMessage {
	t.Helper()

	examples := d.examples(d.Paths[path][method].Responses[status])
	if len(examples) == 0 {
		t.Fatalf("no example documented for the %s answer of %s %s", status, strings.ToUpper(method), path)
	}

	var answer struct {
		Data json.RawMessage `json:"data"`
	}
	noError(t, json.Unmarshal(examples[0], &answer))

	return answer.Data
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))

	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

const contractID = "019a62f1-66b7-7387-a64f-2742c12a2860"

// contractCalls is how the client calls each operation of the API, by operationId.
var contractCalls = map[string]struct {
	call  func(ctx context.Context, client *calisero.Client) error
	query string
}{
	"getAccount": {call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.Accounts.Get(ctx, contractID)
		return err
	}},
	"createMessage": {call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.Messages.Create(ctx, &calisero.CreateMessageRequest{
			Recipient:   phone,
			Body:        "Test message",
			VisibleBody: "Test ******",
			Validity:    72,
			ScheduleAt:  time.Date(2026, 11, 16, 11, 21, 0, 0, time.UTC),
			CallbackURL: "https://yoursite.com/your-callback-url",
			Sender:      "CALISERO",
			ShortenURLs: true,
		})
		return err
	}},
	"listMessages": {query: "page=2", call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.Messages.List(ctx, &calisero.ListOptions{Page: 2})
		return err
	}},
	"getMessage": {call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.Messages.Get(ctx, contractID)
		return err
	}},
	"deleteMessage": {call: func(ctx context.Context, client *calisero.Client) error {
		return client.Messages.Delete(ctx, contractID)
	}},
	"listOptOuts": {query: "page=2", call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.OptOuts.List(ctx, &calisero.ListOptions{Page: 2})
		return err
	}},
	"createOptOut": {call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.OptOuts.Create(ctx, &calisero.CreateOptOutRequest{Phone: phone, Reason: "Unsubscribed"})
		return err
	}},
	"getOptOut": {call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.OptOuts.Get(ctx, contractID)
		return err
	}},
	"updateOptOut": {call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.OptOuts.Update(ctx, contractID, &calisero.UpdateOptOutRequest{Phone: phone, Reason: "Unsubscribed"})
		return err
	}},
	"deleteOptOut": {call: func(ctx context.Context, client *calisero.Client) error {
		return client.OptOuts.Delete(ctx, contractID)
	}},
	"listVerifications": {query: "page=2&status=verified", call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.Verifications.List(ctx, &calisero.VerificationListOptions{Page: 2, Status: calisero.VerificationStatusVerified})
		return err
	}},
	"createVerification": {call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.Verifications.Create(ctx, &calisero.CreateVerificationRequest{Phone: phone, Brand: "Calisero", Template: "Code: {code}", ExpiresIn: 5})
		return err
	}},
	"getVerification": {call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.Verifications.Get(ctx, contractID)
		return err
	}},
	"validateVerificationPost": {call: func(ctx context.Context, client *calisero.Client) error {
		_, err := client.Verifications.Validate(ctx, &calisero.VerificationCheckRequest{Phone: phone, Code: "SBMH0f"})
		return err
	}},
}

func TestContractTheOpenAPIDocumentIsTheVersionThisLibraryWasBuiltFor(t *testing.T) {
	document := loadOpenAPI(t)

	equal(t, strings.HasPrefix(document.Info.Version, "1.0.16 "), true, "version "+document.Info.Version)
	equal(t, document.Servers[0].URL, calisero.DefaultBaseURL, "server")
}

func TestContractEveryDocumentedOperationIsImplementedAndNoOther(t *testing.T) {
	var documented []string

	for _, operation := range loadOpenAPI(t).operations() {
		documented = append(documented, operation.OperationID)
	}

	sort.Strings(documented)
	deepEqual(t, documented, sortedKeys(contractCalls), "operations")
}

func TestContractOperations(t *testing.T) {
	document := loadOpenAPI(t)

	for _, operation := range document.operations() {
		sdk := contractCalls[operation.OperationID]
		status, examples := document.success(operation)

		t.Run(operation.method+" "+operation.path, func(t *testing.T) {
			if len(examples) == 0 {
				t.Fatal("no example documented for the successful answer")
			}

			t.Run("is called with the documented method, path and query", func(t *testing.T) {
				client, stub := clientAnswering(t, rawResponse(status, string(examples[0])))

				noError(t, sdk.call(context.Background(), client))

				request := stub.lastRequest(t)
				equal(t, request.Method, operation.method, "method")
				equal(t, request.URL.Path, "/api/v1"+replaceParameters(operation.path, contractID), "path")
				equal(t, request.URL.RawQuery, sdk.query, "query")
			})

			t.Run("reads the documented answer", func(t *testing.T) {
				for _, example := range examples {
					client, _ := clientAnswering(t, rawResponse(status, string(example)))

					noError(t, sdk.call(context.Background(), client))
				}
			})

			if operation.RequestBody == nil {
				return
			}

			t.Run("sends only documented fields, the required ones included", func(t *testing.T) {
				schema := document.schema(*operation.RequestBody.Content["application/json"].Schema)
				client, stub := clientAnswering(t, rawResponse(status, string(examples[0])))

				noError(t, sdk.call(context.Background(), client))

				sent := stub.lastRequest(t).JSON(t)

				for field := range sent {
					if _, ok := schema.Properties[field]; !ok {
						t.Errorf("the field %q is not documented", field)
					}
				}

				for _, field := range schema.Required {
					if _, ok := sent[field]; !ok {
						t.Errorf("the required field %q is not sent", field)
					}
				}
			})
		})
	}
}

// replaceParameters is a path with its parameters, e.g. {messageId}, replaced by id.
func replaceParameters(path, id string) string {
	for {
		start := strings.Index(path, "{")
		end := strings.Index(path, "}")

		if start < 0 || end < start {
			return path
		}

		path = path[:start] + id + path[end+1:]
	}
}

// jsonFieldNames are the names a type's fields are written under in JSON.
func jsonFieldNames(value any) map[string]bool {
	names := map[string]bool{}
	kind := reflect.TypeOf(value)

	for index := 0; index < kind.NumField(); index++ {
		if name, _, _ := strings.Cut(kind.Field(index).Tag.Get("json"), ","); name != "" && name != "-" {
			names[name] = true
		}
	}

	return names
}

func TestContractEveryDocumentedFieldOfTheResourcesIsRead(t *testing.T) {
	document := loadOpenAPI(t)

	var createdMessage struct {
		ShortenedURLs []json.RawMessage `json:"shortened_urls"`
	}
	noError(t, json.Unmarshal(document.documentedData(t, "/messages", "post", "201"), &createdMessage))

	cases := []struct {
		schema   string
		example  json.RawMessage
		resource any
	}{
		{"Message", document.documentedData(t, "/messages", "post", "201"), &calisero.Message{}},
		{"ShortenedLink", createdMessage.ShortenedURLs[0], &calisero.ShortenedLink{}},
		{"OptOut", document.documentedData(t, "/opt-outs", "post", "201"), &calisero.OptOut{}},
		{"Account", document.documentedData(t, "/accounts/{accountId}", "get", "200"), &calisero.Account{}},
		{"Verification", document.documentedData(t, "/verifications", "post", "201"), &calisero.Verification{}},
	}

	for _, tc := range cases {
		t.Run(tc.schema, func(t *testing.T) {
			// The documented example is read...
			noError(t, json.Unmarshal(tc.example, tc.resource))

			// ...and every documented field has a field of the resource, written back
			names := jsonFieldNames(reflect.ValueOf(tc.resource).Elem().Interface())

			var written object
			noError(t, json.Unmarshal(mustJSON(t, tc.resource), &written))

			for _, property := range sortedKeys(document.Components.Schemas[tc.schema].Properties) {
				if !names[property] {
					t.Errorf("%s.%s is not read", tc.schema, property)
				}

				if _, ok := written[property]; !ok {
					t.Errorf("%s.%s is not written back", tc.schema, property)
				}
			}
		})
	}
}

func TestContractEveryDocumentedFieldOfThePaginationIsRead(t *testing.T) {
	document := loadOpenAPI(t)
	schema := document.Components.Schemas["PaginatedMessages"]

	cases := map[string]map[string]bool{
		"links": jsonFieldNames(calisero.PaginationLinks{}),
		"meta":  jsonFieldNames(calisero.PaginationMeta{}),
	}

	for part, names := range cases {
		for property := range schema.Properties[part].Properties {
			if !names[property] {
				t.Errorf("%s.%s is not read", part, property)
			}
		}
	}
}

func TestContractTheDocumentedDeliveryWebhookIsReadEveryFieldOfIt(t *testing.T) {
	document := loadOpenAPI(t)
	webhook := document.Paths["/messages"]["post"].Callbacks["deliveryStatus"]["{$request.body#/callback_url}"]["post"]
	example := webhook.RequestBody.Content["application/json"].Example

	message, err := calisero.ParseDeliveryWebhook(example)
	noError(t, err)

	var documented, written object
	noError(t, json.Unmarshal(example, &documented))
	noError(t, json.Unmarshal(mustJSON(t, message), &written))

	for _, property := range sortedKeys(document.Components.Schemas["DeliveryWebhookMessage"].Properties) {
		if !sameJSONValue(written[property], documented[property]) {
			t.Errorf("%s: read %#v, documented %#v", property, written[property], documented[property])
		}
	}
}

// sameJSONValue reports whether two decoded JSON values are the same, two date-times
// being the same instant whatever their format.
func sameJSONValue(left, right any) bool {
	leftText, leftIsText := left.(string)
	rightText, rightIsText := right.(string)

	if leftIsText && rightIsText {
		leftTime, leftErr := time.Parse(time.RFC3339Nano, leftText)
		rightTime, rightErr := time.Parse(time.RFC3339Nano, rightText)

		if leftErr == nil && rightErr == nil {
			return leftTime.Equal(rightTime)
		}
	}

	return reflect.DeepEqual(left, right)
}

func TestContractTheClientCanSendEveryDocumentedRequestField(t *testing.T) {
	document := loadOpenAPI(t)

	cases := map[string]func(context.Context, *calisero.Client) error{
		"CreateMessageRequest":      contractCalls["createMessage"].call,
		"CreateOptOutRequest":       contractCalls["createOptOut"].call,
		"UpdateOptOutRequest":       contractCalls["updateOptOut"].call,
		"CreateVerificationRequest": contractCalls["createVerification"].call,
		"VerificationCheckRequest":  contractCalls["validateVerificationPost"].call,
	}

	for name, call := range cases {
		client, stub := clientAnswering(t, errors.New("only the request matters"))

		_ = call(context.Background(), client)

		deepEqual(t, sortedKeys(stub.lastRequest(t).JSON(t)), sortedKeys(document.Components.Schemas[name].Properties), name)
	}
}

func TestContractEveryDocumentedErrorMapsToItsErrorType(t *testing.T) {
	document := loadOpenAPI(t)

	errorTypes := map[string]func(error) bool{
		"401": isError[*calisero.UnauthorizedError],
		"403": isError[*calisero.ForbiddenError],
		"404": isError[*calisero.NotFoundError],
		"405": func(err error) bool {
			_, plain := err.(*calisero.APIError) //nolint:errorlint // A plain *APIError, none of its wrappers

			return plain
		},
		"422": isError[*calisero.ValidationError],
		"429": isError[*calisero.DailyLimitExceededError],
		"500": isError[*calisero.ServerError],
	}

	documented := map[string]bool{}

	for _, operation := range document.operations() {
		for _, status := range sortedKeys(operation.Responses) {
			if strings.HasPrefix(status, "2") {
				continue
			}

			documented[status] = true

			for index, example := range document.examples(operation.Responses[status]) {
				t.Run(fmt.Sprintf("%s %s #%d", operation.OperationID, status, index+1), func(t *testing.T) {
					var code int
					_, _ = fmt.Sscan(status, &code)

					var body struct {
						Message string `json:"message"`
					}
					noError(t, json.Unmarshal(example, &body))

					client, _ := clientAnswering(t, rawResponse(code, string(example)))
					err := contractCalls[operation.OperationID].call(context.Background(), client)

					if isKnown, ok := errorTypes[status]; !ok || !isKnown(err) {
						t.Fatalf("got %T (%v)", err, err)
					}

					apiError := errorAs[*calisero.APIError](t, err)
					equal(t, apiError.StatusCode, code, "status")
					equal(t, apiError.Message, body.Message, "message")
				})
			}
		}
	}

	deepEqual(t, sortedKeys(documented), sortedKeys(errorTypes), "documented statuses")
}

func isError[E error](err error) bool {
	var target E

	return errors.As(err, &target)
}
