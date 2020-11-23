package module

import (
	"context"
	"testing"

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
		secrets     []string
		expectError bool
	}{
		{
			name:        "valid request",
			ctx:         context.WithValue(context.Background(), headerAuthorization, "Bearer foo"),
			secrets:     []string{"foo"},
			expectError: false,
		},
		{
			name:        "invalid request",
			ctx:         context.WithValue(context.Background(), headerAuthorization, "Bearer foo"),
			secrets:     []string{"bar"},
			expectError: true,
		},
		{
			name:        "valid request with multiple keys",
			ctx:         context.WithValue(context.Background(), headerAuthorization, "Bearer foo"),
			secrets:     []string{"foo", "bar"},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := AuthMiddleware(tc.secrets...)(nopEndpoint)(tc.ctx, nil)
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
