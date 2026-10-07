package calisero

import "context"

// AuthProvider provides the bearer token the API authenticates requests with:
// the API key.
//
// Token is called before every request, with the request's context, so it may
// read the key from a secret manager and rotate it without a restart. An error it
// returns fails the request before anything is sent.
type AuthProvider interface {
	Token(ctx context.Context) (string, error)
}

// AuthProviderFunc adapts a function to an AuthProvider.
type AuthProviderFunc func(ctx context.Context) (string, error)

// Token returns f(ctx).
func (f AuthProviderFunc) Token(ctx context.Context) (string, error) {
	return f(ctx)
}

// staticToken authenticates with a fixed API key.
type staticToken string

func (t staticToken) Token(context.Context) (string, error) {
	return string(t), nil
}
