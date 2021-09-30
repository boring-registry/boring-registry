package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testCases := []struct {
		name          string
		path          string
		expected      *Provider
		expectedError bool
	}{
		{
			name:          "valid path",
			path:          "namespace=tier/name=s3/version=1.0.0/os=darwin/arch=amd64",
			expectedError: false,
			expected: &Provider{
				Name:      "s3",
				Namespace: "tier",
				Version:   "1.0.0",
				OS:        "darwin",
				Arch:      "amd64",
			},
		},
		{
			name:          "partial path",
			path:          "namespace=tier/name=foo/version=1.0.0/os=darwin",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			path, err := Parse(tc.path)
			if tc.expectedError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}

			if tc.expected != nil {
				assert.Equal(*tc.expected, path)
			}
		})
	}
}
