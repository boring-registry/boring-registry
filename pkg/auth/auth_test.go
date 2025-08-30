package auth

import (
	"context"
	"testing"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	type providerCase struct {
		name      string
		providers []Provider
		ctx       context.Context
		wantError bool
	}

	testCases := []providerCase{
		{
			name:      "token matches single provider",
			providers: []Provider{NewStaticProvider("foo")},
			ctx:       context.WithValue(context.Background(), jwt.JWTContextKey, "foo"),
			wantError: false,
		},
		{
			name:      "token matches first provider",
			providers: []Provider{NewStaticProvider("foo"), NewStaticProvider("bar")},
			ctx:       context.WithValue(context.Background(), jwt.JWTContextKey, "foo"),
			wantError: false,
		},
		{
			name:      "token matches second provider",
			providers: []Provider{NewStaticProvider("foo"), NewStaticProvider("bar")},
			ctx:       context.WithValue(context.Background(), jwt.JWTContextKey, "bar"),
			wantError: false,
		},
		{
			name:      "token matches none",
			providers: []Provider{NewStaticProvider("foo"), NewStaticProvider("bar")},
			ctx:       context.WithValue(context.Background(), jwt.JWTContextKey, "baz"),
			wantError: true,
		},
		{
			name:      "no token in context",
			providers: []Provider{NewStaticProvider("foo"), NewStaticProvider("bar")},
			ctx:       context.Background(),
			wantError: true,
		},
		{
			name:      "token key is set but empty",
			providers: []Provider{NewStaticProvider("foo")},
			ctx:       context.WithValue(context.Background(), jwt.JWTContextKey, ""),
			wantError: true,
		},
		{
			name:      "no providers, should skip auth",
			providers: []Provider{},
			ctx:       context.WithValue(context.Background(), jwt.JWTContextKey, "anything"),
			wantError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Middleware(tc.providers...)(nopEndpoint)(tc.ctx, nil)
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func nopEndpoint(ctx context.Context, request any) (any, error) {
	return true, nil
}
