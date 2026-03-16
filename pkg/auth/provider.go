package auth

import "context"

type Provider interface {
	Verify(ctx context.Context, token string) error

	// String returns the name of the provider implementation
	String() string
}
