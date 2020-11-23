package module

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testCases := []struct {
		name          string
		input         io.Reader
		expected      *Spec
		expectedError bool
	}{
		{
			name: "valid spec",
			input: strings.NewReader(`
             metadata {
               name      = "s3"
               namespace = "tier"
               version   = "1.0.0"
               provider  = "aws"
             }
			`),
			expected: &Spec{
				Metadata{
					Name:      "s3",
					Namespace: "tier",
					Version:   "1.0.0",
					Provider:  "aws",
				},
			},
		},
		{
			name:          "empty spec",
			input:         strings.NewReader(``),
			expectedError: true,
		},
		{
			name:          "invalid spec",
			input:         strings.NewReader(`foo: bar`),
			expectedError: true,
		},
		{
			name: "missing fields",
			input: strings.NewReader(`
			metadata { name = "s3" }
			`),
			expected:      nil,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			spec, err := Parse(tc.input)
			if tc.expectedError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}

			if tc.expected != nil {
				assert.Equal(tc.expected, spec)
			}
		})
	}
}
