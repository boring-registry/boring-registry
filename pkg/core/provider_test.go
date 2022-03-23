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

func TestProvider_ShasumFileName(t *testing.T) {
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
			},
			expectedFileName: "terraform-provider-random_2.0.0_SHA256SUMS",
			expectError:      false,
		},
		{
			name: "missing name",
			provider: Provider{
				Version: "2.0.0",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fileName, err := tc.provider.ShasumFileName()
			if tc.expectError {
				assert.Error(err)
			} else {
				assert.Equal(tc.expectedFileName, fileName)
			}
		})
	}
}

func TestProvider_ShasumSignatureFileName(t *testing.T) {
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
			},
			expectedFileName: "terraform-provider-random_2.0.0_SHA256SUMS.sig",
			expectError:      false,
		},
		{
			name: "missing version",
			provider: Provider{
				Name: "random",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fileName, err := tc.provider.ShasumSignatureFileName()
			if tc.expectError {
				assert.Error(err)
			} else {
				assert.Equal(tc.expectedFileName, fileName)
			}
		})
	}
}

func TestNewProviderFromArchive(t *testing.T) {
	t.Parallel()
	assert := assertion.New(t)

	testCases := []struct {
		name             string
		fileName         string
		expectedProvider Provider
		expectError      bool
	}{
		{
			name:     "valid filename",
			fileName: "terraform-provider-random_2.0.0_linux_amd64.zip",
			expectedProvider: Provider{
				Name:     "random",
				Version:  "2.0.0",
				OS:       "linux",
				Arch:     "amd64",
				Filename: "terraform-provider-random_2.0.0_linux_amd64.zip",
			},
			expectError: false,
		},
		{
			name:        "invalid filename",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			provider, err := NewProviderFromArchive(tc.fileName)
			if tc.expectError {
				assert.Error(err)
			} else {
				assert.Equal(tc.fileName, provider.Filename)
				assert.Equal(tc.expectedProvider.Name, provider.Name)
				assert.Equal(tc.expectedProvider.Version, provider.Version)
				assert.Equal(tc.expectedProvider.OS, provider.OS)
				assert.Equal(tc.expectedProvider.Arch, provider.Arch)
			}
		})
	}
}
