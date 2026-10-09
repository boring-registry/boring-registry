package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	assertion "github.com/stretchr/testify/assert"
)

// fakeUDCFetcher counts user delegation credential fetches and records the requested KeyInfo
type fakeUDCFetcher struct {
	calls    int
	lastInfo service.KeyInfo
	err      error
}

func (f *fakeUDCFetcher) fetch(_ context.Context, info service.KeyInfo, _ *service.GetUserDelegationCredentialOptions) (*service.UserDelegationCredential, error) {
	f.calls++
	f.lastInfo = info
	if f.err != nil {
		return nil, f.err
	}
	return &service.UserDelegationCredential{}, nil
}

func parseSASTime(t *testing.T, s *string) time.Time {
	t.Helper()
	parsed, err := time.Parse(sas.TimeFormat, *s)
	if err != nil {
		t.Fatalf("failed to parse SAS time %q: %v", *s, err)
	}
	return parsed
}

func TestAzureStorage_userDelegationCredential_CachesCredential(t *testing.T) {
	assert := assertion.New(t)
	fetcher := &fakeUDCFetcher{}
	s := &AzureStorage{signedURLExpiry: 5 * time.Minute, getUDC: fetcher.fetch}

	first, err := s.userDelegationCredential(context.Background())
	assert.NoError(err)
	for i := 0; i < 4; i++ {
		udc, err := s.userDelegationCredential(context.Background())
		assert.NoError(err)
		assert.Same(first, udc)
	}

	assert.Equal(1, fetcher.calls)
}

func TestAzureStorage_userDelegationCredential_RefreshesNearExpiry(t *testing.T) {
	assert := assertion.New(t)
	fetcher := &fakeUDCFetcher{}
	s := &AzureStorage{signedURLExpiry: 5 * time.Minute, getUDC: fetcher.fetch}

	_, err := s.userDelegationCredential(context.Background())
	assert.NoError(err)

	// The cached key would expire before a newly signed URL does
	s.udcExpiry = time.Now().Add(s.signedURLExpiry)

	_, err = s.userDelegationCredential(context.Background())
	assert.NoError(err)
	assert.Equal(2, fetcher.calls)
}

func TestAzureStorage_userDelegationCredential_DoesNotCacheErrors(t *testing.T) {
	assert := assertion.New(t)
	fetcher := &fakeUDCFetcher{err: errors.New("boom")}
	s := &AzureStorage{signedURLExpiry: 5 * time.Minute, getUDC: fetcher.fetch}

	udc, err := s.userDelegationCredential(context.Background())
	assert.ErrorIs(err, fetcher.err)
	assert.Nil(udc)

	fetcher.err = nil
	udc, err = s.userDelegationCredential(context.Background())
	assert.NoError(err)
	assert.NotNil(udc)
	assert.Equal(2, fetcher.calls)
}

func TestAzureStorage_userDelegationCredential_KeyValidity(t *testing.T) {
	tests := []struct {
		name            string
		signedURLExpiry time.Duration
		wantValidity    time.Duration
	}{
		{
			name:            "default signed URL expiry",
			signedURLExpiry: 5 * time.Minute,
			wantValidity:    5*time.Minute + azureUDCMinValidity,
		},
		{
			name:            "long signed URL expiry extends validity by the minimum validity",
			signedURLExpiry: 10 * time.Hour,
			wantValidity:    10*time.Hour + azureUDCMinValidity,
		},
		{
			name:            "validity is clamped to the Azure maximum of 7 days",
			signedURLExpiry: 200 * time.Hour,
			wantValidity:    azureUDCMaxValidity,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert := assertion.New(t)
			fetcher := &fakeUDCFetcher{}
			s := &AzureStorage{signedURLExpiry: tc.signedURLExpiry, getUDC: fetcher.fetch}

			now := time.Now().UTC()
			_, err := s.userDelegationCredential(context.Background())
			assert.NoError(err)

			// sas.TimeFormat has second precision
			start := parseSASTime(t, fetcher.lastInfo.Start)
			expiry := parseSASTime(t, fetcher.lastInfo.Expiry)
			assert.WithinDuration(now.Add(-azureUDCClockSkew), start, 2*time.Second)
			assert.WithinDuration(now.Add(tc.wantValidity), expiry, 2*time.Second)
		})
	}
}

func TestNewAzuriteStorage_SetsUserDelegationCredentialFetcher(t *testing.T) {
	assert := assertion.New(t)
	client, err := azblob.NewClientWithNoCredential("https://example.blob.core.windows.net/", nil)
	assert.NoError(err)

	s := NewAzuriteStorage(client, nil, "example", "container", "", DefaultModuleArchiveFormat, 5*time.Minute).(*AzureStorage)
	assert.NotNil(s.getUDC)
}

func TestNewAzureStorage_RejectsSignedURLExpiryAboveKeyMaximum(t *testing.T) {
	assert := assertion.New(t)

	_, err := NewAzureStorage("example", "container", WithAzureStorageSignedUrlExpiry(azureUDCMaxValidity+time.Hour))
	assert.ErrorContains(err, "exceeds the Azure user delegation key maximum")
}
