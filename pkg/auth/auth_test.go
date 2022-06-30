package auth_test

import (
	"context"
	"testing"

	"github.com/TierMobility/boring-registry/pkg/auth"
	"github.com/TierMobility/boring-registry/pkg/auth/providers/static"
	"github.com/go-kit/kit/auth/jwt"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	var (
		assert = assert.New(t)
	)

	testCases := []struct {
		name        string
		ctx         context.Context
		token       string
		expectError bool
	}{
		{
			name:        "valid request",
			ctx:         context.WithValue(context.Background(), jwt.JWTTokenContextKey, "foo"),
			token:       "foo",
			expectError: false,
		},
		{
			name:        "invalid request",
			ctx:         context.WithValue(context.Background(), jwt.JWTTokenContextKey, "foo"),
			token:       "bar",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := auth.Middleware(static.New(tc.token))(nopEndpoint)(tc.ctx, nil)
			switch tc.expectError {
			case true:
				assert.Error(err)
			case false:
				assert.NoError(err)
			}
		})
	}
}

func nopEndpoint(ctx context.Context, request interface{}) (interface{}, error) {
	return true, nil
}
