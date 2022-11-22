package auth

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := NewStaticProvider(tc.tokens...).(*StaticProvider)
			assert.ElementsMatch(t, tc.expectedTokens, p.tokens)
		})
	}
}
