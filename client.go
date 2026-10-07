package calisero

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the production API.
const DefaultBaseURL = "https://rest.calisero.ro/api/v1"

// DefaultTimeout is how long the default HTTP client waits for each answer,
// headers and body included.
const DefaultTimeout = 30 * time.Second

// HTTPDoer sends an HTTP request and returns the answer, whatever its status:
// *http.Client implements it.
//
// It returns an error only when the request got no answer (a network error, a
// timeout, a cancelled context): the client turns that error into a
// *TransportError.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is the Calisero SMS API client. Create it with NewClient, once, and
// share it: a Client is safe for concurrent use by multiple goroutines.
//
// It never retries a request: the API takes no idempotency key, so a message
// retried after a timeout or a server error could be sent twice.
type Client struct {
	// Messages sends and manages SMS messages.
	Messages *MessageService

	// OptOuts manages the phone numbers that opted out of receiving messages.
	OptOuts *OptOutService

	// Accounts reads the account: credit, status and daily sending limit.
	Accounts *AccountService

	// Verifications verifies phone numbers with one-time codes.
	Verifications *VerificationService

	baseURL    string
	userAgent  string
	auth       AuthProvider
	httpClient HTTPDoer
}

// Option configures a Client: pass them to NewClient.
type Option func(*settings) error

// settings is what the options of NewClient set.
type settings struct {
	baseURL    string
	timeout    time.Duration
	httpClient HTTPDoer
	appProduct string
	auth       AuthProvider
}

// WithBaseURL sends the requests to another base URL than DefaultBaseURL, e.g. a
// local server in tests. It must be an absolute http or https URL, without a
// query string.
func WithBaseURL(baseURL string) Option {
	return func(s *settings) error {
		trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
		parsed, err := url.Parse(trimmed)

		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
			parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("calisero: the base URL must be an absolute http or https URL, got %q", baseURL)
		}

		s.baseURL = trimmed

		return nil
	}
}

// WithTimeout sets how long the default HTTP client waits for each answer,
// headers and body included: DefaultTimeout otherwise. It is ignored with
// WithHTTPClient, whose client keeps its own timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(s *settings) error {
		if timeout <= 0 {
			return fmt.Errorf("calisero: the timeout must be positive, got %s", timeout)
		}

		s.timeout = timeout

		return nil
	}
}

// WithHTTPClient sends the requests through httpClient instead of the default
// HTTP client: an *http.Client with a proxy, TLS settings or instrumentation of
// your own, or a stub in tests.
//
// It is used as configured, its timeout and its redirect policy included,
// whereas the default client waits DefaultTimeout and follows no redirect.
func WithHTTPClient(httpClient HTTPDoer) Option {
	return func(s *settings) error {
		if standard, ok := httpClient.(*http.Client); httpClient == nil || (ok && standard == nil) {
			return errors.New("calisero: the HTTP client must not be nil")
		}

		s.httpClient = httpClient

		return nil
	}
}

// WithAppInfo names your application in the User-Agent header, after the
// library: WithAppInfo("MyShop", "2.1.0") appends "MyShop/2.1.0", so the requests
// of each of your applications can be told apart. Characters a header does not
// allow (spaces, diacritics…) become "-"; the version may be empty.
func WithAppInfo(name, version string) Option {
	return func(s *settings) error {
		product, err := appProduct(name, version)
		if err != nil {
			return err
		}

		s.appProduct = product

		return nil
	}
}

// WithAuthProvider authenticates the requests with the token of provider instead
// of an API key, e.g. to read the key from a secret manager and rotate it without
// a restart. NewClient then ignores its API key, which may be empty.
func WithAuthProvider(provider AuthProvider) Option {
	return func(s *settings) error {
		if provider == nil {
			return errors.New("calisero: the auth provider must not be nil")
		}

		s.auth = provider

		return nil
	}
}

// NewClient creates a client that authenticates with apiKey, a key of the
// dashboard's API Keys section, or with the provider of WithAuthProvider.
//
// It returns an error when the API key is empty and no provider is given, or
// when an option is invalid: a misconfigured environment fails at startup rather
// than on the first message.
func NewClient(apiKey string, options ...Option) (*Client, error) {
	s := settings{baseURL: DefaultBaseURL, timeout: DefaultTimeout}

	for _, option := range options {
		if option == nil {
			continue
		}

		if err := option(&s); err != nil {
			return nil, err
		}
	}

	if s.auth == nil {
		if strings.TrimSpace(apiKey) == "" {
			return nil, errors.New("calisero: the API key must not be empty")
		}

		s.auth = staticToken(apiKey)
	}

	if s.httpClient == nil {
		s.httpClient = &http.Client{
			Timeout: s.timeout,
			// A redirect is answered as an error rather than followed: the API answers
			// with one only when something is wrong, and the API key stays on its host
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	client := &Client{
		baseURL:    s.baseURL,
		userAgent:  buildUserAgent(s.appProduct),
		auth:       s.auth,
		httpClient: s.httpClient,
	}
	client.Messages = &MessageService{client: client}
	client.OptOuts = &OptOutService{client: client}
	client.Accounts = &AccountService{client: client}
	client.Verifications = &VerificationService{client: client}

	return client, nil
}

// BaseURL is the base URL every request path is appended to, without a trailing
// slash: DefaultBaseURL unless WithBaseURL gave another one.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// UserAgent is the User-Agent header of every request: the library and its
// version, the Go release and the platform, then your application, e.g.
// "Calisero-SMS-Go/1.0.0 (Go 1.27.1; linux amd64) MyShop/2.1.0".
func (c *Client) UserAgent() string {
	return c.userAgent
}
