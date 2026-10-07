package calisero_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

// The client over real HTTP: its default HTTP client against a local server,
// never the network.

// receivedRequest is a request the local server received, its body read.
type receivedRequest struct {
	method string
	uri    string
	header http.Header
	body   string
}

// localServer is an HTTP server on a free local port that records the requests it
// receives and answers them with handle.
type localServer struct {
	*httptest.Server

	mu       sync.Mutex
	received []receivedRequest
}

func startServer(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *localServer {
	t.Helper()

	server := &localServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		server.mu.Lock()
		server.received = append(server.received, receivedRequest{method: r.Method, uri: r.RequestURI, header: r.Header.Clone(), body: string(body)})
		server.mu.Unlock()

		handle(w, r)
	}))
	t.Cleanup(server.Close)

	return server
}

func (s *localServer) requests() []receivedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]receivedRequest(nil), s.received...)
}

// clientFor is a client of the local server, with the default HTTP client.
func clientFor(t *testing.T, serverURL string, options ...calisero.Option) *calisero.Client {
	t.Helper()

	client, err := calisero.NewClient("integration-key", append([]calisero.Option{calisero.WithBaseURL(serverURL + "/api/v1")}, options...)...)
	noError(t, err)

	return client
}

func sendJSON(w http.ResponseWriter, status int, body any, headers ...string) {
	for index := 0; index+1 < len(headers); index += 2 {
		w.Header().Set(headers[index], headers[index+1])
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func TestOverHTTPAnAuthenticatedJSONRequestAndItsAnswer(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, 201, object{"data": messageData(object{"body": "Bună ziua! Ștefan, țară, încă"})},
			"X-Trace-Id", traceID,
			"X-RateLimit-Limit", "240",
			"X-RateLimit-Remaining", "239",
			"X-Daily-Limit", "1000",
			"X-Daily-Remaining", "999",
		)
	})

	response, err := clientFor(t, server.URL).Messages.Create(context.Background(), &calisero.CreateMessageRequest{
		Recipient: phone,
		Body:      "Bună ziua! Ștefan, țară, încă",
	})
	noError(t, err)

	received := server.requests()[0]
	equal(t, received.method, http.MethodPost, "method")
	equal(t, received.uri, "/api/v1/messages", "URI")
	equal(t, received.header.Get("Accept"), "application/json", "Accept")
	equal(t, received.header.Get("Content-Type"), "application/json", "Content-Type")
	equal(t, received.header.Get("Authorization"), "Bearer integration-key", "Authorization")
	equal(t, received.header.Get("User-Agent"), fmt.Sprintf("Calisero-SMS-Go/%s (Go %s; %s %s)",
		calisero.Version, strings.TrimPrefix(strings.Fields(runtime.Version())[0], "go"), runtime.GOOS, runtime.GOARCH), "User-Agent")
	equal(t, received.body, `{"recipient":"+40742***350","body":"Bună ziua! Ștefan, țară, încă"}`, "body")

	equal(t, response.Data.Body, "Bună ziua! Ștefan, țară, încă", "message body")
	deepEqual(t, response.ResponseMeta, calisero.ResponseMeta{
		TraceID:            traceID,
		RateLimitLimit:     ptr(240),
		RateLimitRemaining: ptr(239),
		DailyLimit:         ptr(1000),
		DailyRemaining:     ptr(999),
	}, "response meta")
}

func TestOverHTTPTheQueryOfAListAndItsPage(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, 200, pageOf([]any{messageData(nil)}, "messages", 2, true))
	})

	page, err := clientFor(t, server.URL).Messages.List(context.Background(), &calisero.ListOptions{Page: 2})
	noError(t, err)

	equal(t, server.requests()[0].uri, "/api/v1/messages?page=2", "URI")
	equal(t, page.Data[0].ID, messageID, "message")
	equal(t, page.HasNextPage(), true, "has a next page")
}

func TestOverHTTPACancellationAnsweredWithAnEmptyArray(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, 200, []any{})
	})

	noError(t, clientFor(t, server.URL).Messages.Delete(context.Background(), messageID))

	received := server.requests()[0]
	equal(t, received.method, http.MethodDelete, "method")
	equal(t, received.uri, "/api/v1/messages/"+messageID, "URI")
	equal(t, received.body, "", "body")
}

func TestOverHTTPADailyLimitRefusal(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, 429, dailyLimitBody, "Retry-After", "21600", "X-Daily-Limit", "1000", "X-Daily-Remaining", "0", "X-Trace-Id", traceID)
	})

	_, err := clientFor(t, server.URL).Messages.Create(context.Background(), &calisero.CreateMessageRequest{Recipient: phone, Body: "Hello"})

	exceeded := errorAs[*calisero.DailyLimitExceededError](t, err)
	equal(t, exceeded.StatusCode, 429, "status")
	equal(t, exceeded.TraceID, traceID, "trace ID")
	equal(t, exceeded.RetryAfter, 6*time.Hour, "retry after")
	equal(t, exceeded.DailyLimit, 1000, "daily limit")
	equal(t, exceeded.DailyRemaining, 0, "daily remaining")
	equal(t, exceeded.ResetsAt.Format(time.RFC3339), "2026-10-01T00:00:00+03:00", "resets at")
}

func TestOverHTTPValidationErrors(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, 422, object{
			"message":  "The recipient field is required.",
			"errors":   object{"recipient": []any{"The recipient field is required."}},
			"trace_id": traceID,
		})
	})

	_, err := clientFor(t, server.URL).Messages.Create(context.Background(), &calisero.CreateMessageRequest{Body: "Hello"})

	validation := errorAs[*calisero.ValidationError](t, err)
	equal(t, validation.TraceID, traceID, "trace ID")
	deepEqual(t, validation.ValidationErrors, map[string][]string{"recipient": {"The recipient field is required."}}, "field errors")
}

func TestOverHTTPAnHTMLErrorPage(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "<html><body>Bad Gateway</body></html>")
	})

	_, err := clientFor(t, server.URL).Accounts.Get(context.Background(), accountID)

	equal(t, errorAs[*calisero.ServerError](t, err).Message, "HTTP error 502", "message")
}

func TestOverHTTPARedirectIsNotFollowed(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/elsewhere") {
			sendJSON(w, 200, object{"data": messageData(nil)})

			return
		}

		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	})

	_, err := clientFor(t, server.URL).Messages.Get(context.Background(), messageID)

	equal(t, errorAs[*calisero.APIError](t, err).StatusCode, http.StatusFound, "status")
	equal(t, len(server.requests()), 1, "requests received")
	equal(t, server.requests()[0].uri, "/api/v1/messages/"+messageID, "URI")
}

func TestOverHTTPASlowAnswerTimesOut(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
			return
		}

		sendJSON(w, 200, object{"data": messageData(nil)})
	})

	startedAt := time.Now()
	_, err := clientFor(t, server.URL, calisero.WithTimeout(100*time.Millisecond)).Messages.Get(context.Background(), messageID)

	transport := errorAs[*calisero.TransportError](t, err)
	equal(t, transport.StatusCode, 0, "status")

	var netError net.Error
	equal(t, errors.As(err, &netError) && netError.Timeout(), true, "a timeout")

	if elapsed := time.Since(startedAt); elapsed > 1500*time.Millisecond {
		t.Errorf("the request took %s", elapsed)
	}
}

func TestOverHTTPTheCallerCancels(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
			return
		}

		sendJSON(w, 200, object{"data": messageData(nil)})
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := clientFor(t, server.URL).Messages.Get(ctx, messageID)

	isA[*calisero.TransportError](t, err)
	equal(t, errors.Is(err, context.DeadlineExceeded), true, "the context's error is kept")
}

func TestOverHTTPARefusedConnection(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, _ *http.Request) { sendJSON(w, 200, object{}) })
	serverURL := server.URL
	server.Close()

	_, err := clientFor(t, serverURL).Messages.List(context.Background(), nil)

	transport := errorAs[*calisero.TransportError](t, err)
	equal(t, transport.Err != nil, true, "the cause is kept")
	equal(t, strings.HasPrefix(transport.Message, "HTTP request failed: "), true, "message")
}

func TestOverHTTPConcurrentRequestsEachGetTheirAnswer(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]

		// Answer in an order unrelated to the order of the requests
		time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)

		sendJSON(w, 200, object{"data": messageData(object{"id": id})}, "X-Trace-Id", "trace-"+id)
	})

	client := clientFor(t, server.URL)
	responses := make([]*calisero.Response[calisero.Message], 20)
	errs := make([]error, 20)

	var wait sync.WaitGroup

	for index := range responses {
		wait.Add(1)

		go func() {
			defer wait.Done()

			responses[index], errs[index] = client.Messages.Get(context.Background(), fmt.Sprintf("msg-%d", index))
		}()
	}

	wait.Wait()

	for index, response := range responses {
		noError(t, errs[index])
		equal(t, response.Data.ID, fmt.Sprintf("msg-%d", index), "message ID")
		equal(t, response.ResponseMeta.TraceID, fmt.Sprintf("trace-msg-%d", index), "trace ID")
	}
}

func TestOverHTTPTheUserAgentNamesYourApplication(t *testing.T) {
	server := startServer(t, func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, 200, pageOf(nil, "opt-outs", 1, false))
	})

	client := clientFor(t, server.URL, calisero.WithAppInfo("My Shop", "2.1.0"))

	_, err := client.OptOuts.List(context.Background(), nil)
	noError(t, err)

	userAgent := server.requests()[0].header.Get("User-Agent")
	equal(t, userAgent, client.UserAgent(), "User-Agent")

	pattern := regexp.MustCompile(`^Calisero-SMS-Go/\d+\.\d+\.\d+ \(Go [^;()]+; [a-z0-9]+ [a-z0-9]+\) My-Shop/2\.1\.0$`)
	equal(t, pattern.MatchString(userAgent), true, "User-Agent "+userAgent)
}
