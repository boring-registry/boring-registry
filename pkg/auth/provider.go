package auth

import "context"

type Provider interface {
	Verify(ctx context.Context, token string) error
}
