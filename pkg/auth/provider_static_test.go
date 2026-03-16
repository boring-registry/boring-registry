package auth

import (
	"context"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"

	"github.com/stretchr/testify/assert"
)

func TestNewStaticProvider(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name           string
		tokens         []string
		expectedTokens []string
	}{
		{
			name:           "no comma-separated tokens",
			tokens:         []string{"example123", "example321"},
			expectedTokens: []string{"example123", "example321"},
		},
		{
			name:           "comma-separated tokens",
			tokens:         []string{"example123", "first,second"},
			expectedTokens: []string{"example123", "first", "second"},
		},
		{
			name:           "incorrectly passed comma-separated tokens",
			tokens:         []string{"example123", "first,"},
			expectedTokens: []string{"example123", "first"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewStaticProvider(tc.tokens...).(*StaticProvider)
			assert.ElementsMatch(t, tc.expectedTokens, p.tokens)
		})
	}
}

func TestStaticProvider_Verify(t *testing.T) {
	t.Parallel()

	type args struct {
		token string
	}
	tests := []struct {
		name      string
		tokens    []string
		args      args
		wantError bool
	}{
		{
			name:      "valid token",
			tokens:    []string{"foo", "bar"},
			args:      args{token: "foo"},
			wantError: false,
		},
		{
			name:      "invalid token",
			tokens:    []string{"foo", "bar"},
			args:      args{token: "invalid"},
			wantError: true,
		},
		{
			name:      "empty token list",
			tokens:    []string{},
			args:      args{token: "foo"},
			wantError: true,
		},
		{
			name:      "token with comma-separated input",
			tokens:    []string{"foo,bar"},
			args:      args{token: "bar"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewStaticProvider(tt.tokens...).(*StaticProvider)
			err := p.Verify(context.Background(), tt.args.token)
			if tt.wantError {
				assert.ErrorIs(t, err, core.ErrInvalidToken)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
