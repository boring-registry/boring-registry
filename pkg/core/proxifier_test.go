package core

import (
	"context"
	"testing"

	assertion "github.com/stretchr/testify/assert"
)

const (
	prefixProxy     = "/v1/proxy"
	downloadUrlRoot = "https://s3.aws.com/"
	downloadUrlPath = "providers/sfr/siroco/terraform-provider-random_2.0.0_linux_amd64.zip?X-Signature=ABC"
	downloadUrl     = downloadUrlRoot + downloadUrlPath
)

func TestProxifier_GetProxyUrl(t *testing.T) {
	t.Parallel()
	assert := assertion.New(t)

	testCases := []struct {
		name        string
		service     ProxyUrlService
		downloadUrl string
		rootUrl     string
		expectedUrl string
		expectError bool
	}{
		{
			name:        "valid proxy URL",
			service:     NewProxyUrlService(true, prefixProxy),
			downloadUrl: downloadUrl,
			expectedUrl: prefixProxy + "/" + downloadUrlPath,
			expectError: false,
		},
		{
			name:        "invalid download URL",
			service:     NewProxyUrlService(true, prefixProxy),
			downloadUrl: "this is not an URL",
			expectedUrl: "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			url, err := tc.service.GetProxyUrl(ctx, tc.downloadUrl)

			if tc.expectError {
				assert.Equal(tc.expectedUrl, url)
				assert.NotNil(err)
			} else {
				assert.Equal(tc.expectedUrl, url)
			}
		})
	}
}

func TestProxifier_IsEnabled(t *testing.T) {
	t.Parallel()
	assert := assertion.New(t)

	testCases := []struct {
		name    string
		service ProxyUrlService
		expect  bool
	}{
		{
			name:    "proxy is enabled",
			service: NewProxyUrlService(true, prefixProxy),
			expect:  true,
		},
		{
			name:    "proxy is NOT enabled",
			service: NewProxyUrlService(false, prefixProxy),
			expect:  false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			isEnabled := tc.service.IsProxyEnabled(ctx)
			assert.Equal(tc.expect, isEnabled)
		})
	}
}
