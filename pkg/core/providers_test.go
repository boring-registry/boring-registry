package core

import (
	assertion "github.com/stretchr/testify/assert"
	"testing"
)

func TestProvider_ArchiveFileName(t *testing.T) {
	t.Parallel()
	assert := assertion.New(t)

	testCases := []struct {
		name             string
		provider         Provider
		expectedFileName string
		expectError      bool
	}{
		{
			name: "valid provider",
			provider: Provider{
				Name:    "random",
				Version: "2.0.0",
				OS:      "linux",
				Arch:    "amd64",
			},
			expectedFileName: "terraform-provider-random_2.0.0_linux_amd64.zip",
			expectError:      false,
		},
		{
			name: "missing name",
			provider: Provider{
				Version: "2.0.0",
				OS:      "linux",
				Arch:    "amd64",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fileName, err := tc.provider.ArchiveFileName()
			if tc.expectError {
				assert.Error(err)
			} else {
				assert.Equal(tc.expectedFileName, fileName)
			}
		})
	}
}
