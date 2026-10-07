package calisero_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	calisero "github.com/calisero/calisero-go"
)

func TestNewClientHasTheServices(t *testing.T) {
	client, err := calisero.NewClient("test-api-key")
	noError(t, err)

	if client.Messages == nil || client.OptOuts == nil || client.Accounts == nil || client.Verifications == nil {
		t.Fatalf("a service is missing: %+v", client)
	}

	equal(t, client.BaseURL(), calisero.DefaultBaseURL, "base URL")
	equal(t, calisero.DefaultBaseURL, "https://rest.calisero.ro/api/v1", "default base URL")
	equal(t, calisero.DefaultTimeout, 30*time.Second, "default timeout")
}

func TestNewClientsAreIndependent(t *testing.T) {
	first, err := calisero.NewClient("first-key")
	noError(t, err)
	second, err := calisero.NewClient("second-key")
	noError(t, err)

	if first.Messages == second.Messages || first.Verifications == second.Verifications {
		t.Fatal("two clients share a service")
	}
}

func TestNewClientRefusesAnEmptyAPIKey(t *testing.T) {
	for _, apiKey := range []string{"", "   ", "\t\n"} {
		client, err := calisero.NewClient(apiKey)

		if err == nil || client != nil {
			t.Fatalf("NewClient(%q) = %v, %v: want an error", apiKey, client, err)
		}

		equal(t, err.Error(), "calisero: the API key must not be empty", "error")
	}
}

func TestNewClientAuthenticatesWithTheAPIKey(t *testing.T) {
	client, stub := clientAnswering(t, jsonResponse(200, object{"data": messageData(nil)}))

	_, err := client.Messages.Get(context.Background(), messageID)
	noError(t, err)

	equal(t, stub.lastRequest(t).Header.Get("Authorization"), "Bearer test-api-key", "Authorization")
}

func TestWithAuthProviderIsAskedBeforeEachRequest(t *testing.T) {
	var calls atomic.Int32

	provider := calisero.AuthProviderFunc(func(ctx context.Context) (string, error) {
		if ctx == nil {
			t.Error("the auth provider got no context")
		}

		return fmt.Sprintf("rotated-key-%d", calls.Add(1)), nil
	})
	stub := &stubDoer{answers: []any{
		jsonResponse(200, object{"data": messageData(nil)}),
		jsonResponse(200, object{"data": messageData(nil)}),
	}}

	client, err := calisero.NewClient("", calisero.WithAuthProvider(provider), calisero.WithHTTPClient(stub))
	noError(t, err)

	for range 2 {
		_, err = client.Messages.Get(context.Background(), messageID)
		noError(t, err)
	}

	equal(t, stub.requests[0].Header.Get("Authorization"), "Bearer rotated-key-1", "first Authorization")
	equal(t, stub.requests[1].Header.Get("Authorization"), "Bearer rotated-key-2", "second Authorization")
}

func TestWithAuthProviderErrorFailsTheRequestBeforeSendingIt(t *testing.T) {
	cause := errors.New("secret manager unavailable")
	stub := &stubDoer{}

	client, err := calisero.NewClient("", calisero.WithHTTPClient(stub), calisero.WithAuthProvider(
		calisero.AuthProviderFunc(func(context.Context) (string, error) { return "", cause }),
	))
	noError(t, err)

	_, err = client.Messages.Get(context.Background(), messageID)

	if !errors.Is(err, cause) {
		t.Fatalf("got %v, want an error wrapping the provider's", err)
	}

	equal(t, err.Error(), "calisero: get the API key from the auth provider: secret manager unavailable", "error")
	equal(t, stub.requestCount(), 0, "requests sent")
}

func TestWithAuthProviderRefusesAnEmptyToken(t *testing.T) {
	stub := &stubDoer{}

	client, err := calisero.NewClient("", calisero.WithHTTPClient(stub), calisero.WithAuthProvider(
		calisero.AuthProviderFunc(func(context.Context) (string, error) { return " ", nil }),
	))
	noError(t, err)

	_, err = client.Messages.List(context.Background(), nil)

	if err == nil || err.Error() != "calisero: the auth provider returned an empty API key" {
		t.Fatalf("got %v, want the empty key refused", err)
	}

	equal(t, stub.requestCount(), 0, "requests sent")
}

func TestWithAuthProviderRefusesNil(t *testing.T) {
	_, err := calisero.NewClient("test-api-key", calisero.WithAuthProvider(nil))

	if err == nil {
		t.Fatal("a nil auth provider was accepted")
	}
}

func TestWithBaseURL(t *testing.T) {
	stub := &stubDoer{answers: []any{jsonResponse(200, pageOf(nil, "messages", 1, false))}}

	client, err := calisero.NewClient("test-api-key", calisero.WithHTTPClient(stub), calisero.WithBaseURL(" http://127.0.0.1:8080/api/v1/ "))
	noError(t, err)

	equal(t, client.BaseURL(), "http://127.0.0.1:8080/api/v1", "base URL")

	_, err = client.Messages.List(context.Background(), nil)
	noError(t, err)

	equal(t, stub.lastRequest(t).URL.String(), "http://127.0.0.1:8080/api/v1/messages", "request URL")
}

func TestWithBaseURLRefusesWhatIsNotAnAbsoluteHTTPURL(t *testing.T) {
	for _, baseURL := range []string{
		"",
		"rest.calisero.ro/api/v1",
		"/api/v1",
		"ftp://rest.calisero.ro/api/v1",
		"https://",
		"https://rest.calisero.ro/api/v1?version=1",
		"https://rest.calisero.ro/api/v1#messages",
		"://rest.calisero.ro",
	} {
		_, err := calisero.NewClient("test-api-key", calisero.WithBaseURL(baseURL))

		if err == nil || !strings.HasPrefix(err.Error(), "calisero: the base URL must be an absolute http or https URL") {
			t.Errorf("WithBaseURL(%q): got %v, want the URL refused", baseURL, err)
		}
	}
}

func TestWithHTTPClientRefusesNil(t *testing.T) {
	var nilClient *http.Client

	for _, httpClient := range []calisero.HTTPDoer{nil, nilClient} {
		_, err := calisero.NewClient("test-api-key", calisero.WithHTTPClient(httpClient))

		if err == nil || err.Error() != "calisero: the HTTP client must not be nil" {
			t.Errorf("WithHTTPClient(%#v): got %v, want it refused", httpClient, err)
		}
	}
}

func TestWithTimeoutRefusesWhatIsNotPositive(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		_, err := calisero.NewClient("test-api-key", calisero.WithTimeout(timeout))

		if err == nil || !strings.HasPrefix(err.Error(), "calisero: the timeout must be positive") {
			t.Errorf("WithTimeout(%s): got %v, want it refused", timeout, err)
		}
	}
}

func TestNewClientSkipsNilOptions(t *testing.T) {
	client, err := calisero.NewClient("test-api-key", nil, calisero.WithTimeout(time.Second), nil)
	noError(t, err)

	equal(t, client.BaseURL(), calisero.DefaultBaseURL, "base URL")
}

func TestClientUserAgentNamesTheLibraryAndTheRuntime(t *testing.T) {
	client, err := calisero.NewClient("test-api-key")
	noError(t, err)

	pattern := regexp.MustCompile(`^Calisero-SMS-Go/\d+\.\d+\.\d+ \(Go [0-9A-Za-z.+_-]+; [a-z0-9]+ [a-z0-9]+\)$`)

	if !pattern.MatchString(client.UserAgent()) {
		t.Fatalf("User-Agent %q does not match %s", client.UserAgent(), pattern)
	}

	equal(t, strings.HasPrefix(client.UserAgent(), calisero.UserAgent+" (Go "), true, "starts with the product token")
	equal(t, strings.HasSuffix(client.UserAgent(), "; "+runtime.GOOS+" "+runtime.GOARCH+")"), true, "ends with the platform")
	equal(t, calisero.UserAgent, "Calisero-SMS-Go/"+calisero.Version, "product token")
}

func TestWithAppInfoAppendsYourApplication(t *testing.T) {
	cases := []struct {
		name, version, product string
	}{
		{"MyShop", "2.1.0", "MyShop/2.1.0"},
		{"My Shop", "2.1 beta", "My-Shop/2.1-beta"},
		{"Magazinul Ăla", "", "Magazinul-la"},
		{" shop\r\nX-Injected: 1 ", "1.0 (build 7)", "shop-X-Injected-1/1.0-build-7"},
		{"MyShop", "   ", "MyShop"},
	}

	for _, tc := range cases {
		client, err := calisero.NewClient("test-api-key", calisero.WithAppInfo(tc.name, tc.version))
		noError(t, err)

		if !strings.HasSuffix(client.UserAgent(), ") "+tc.product) {
			t.Errorf("WithAppInfo(%q, %q): User-Agent %q, want it to end with %q", tc.name, tc.version, client.UserAgent(), tc.product)
		}
	}
}

func TestWithAppInfoRefusesANameWithoutUsableCharacters(t *testing.T) {
	for _, name := range []string{"", "   ", "ăîșț", "()"} {
		_, err := calisero.NewClient("test-api-key", calisero.WithAppInfo(name, "1.0"))

		if err == nil || err.Error() != "calisero: the application name must hold letters, digits or -._ characters" {
			t.Errorf("WithAppInfo(%q): got %v, want it refused", name, err)
		}
	}
}

func TestTheUserAgentIsSentWithEveryRequest(t *testing.T) {
	stub := &stubDoer{answers: []any{jsonResponse(200, pageOf(nil, "opt-outs", 1, false))}}

	client, err := calisero.NewClient("test-api-key", calisero.WithHTTPClient(stub), calisero.WithAppInfo("MyShop", "2.1.0"))
	noError(t, err)

	_, err = client.OptOuts.List(context.Background(), nil)
	noError(t, err)

	equal(t, stub.lastRequest(t).Header.Get("User-Agent"), client.UserAgent(), "User-Agent")
}
