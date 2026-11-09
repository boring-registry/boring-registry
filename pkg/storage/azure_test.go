package storage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

// blockingUDCFetcher counts fetches and blocks each fetch until release is closed
type blockingUDCFetcher struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	ctxErr  chan error
}

func newBlockingUDCFetcher() *blockingUDCFetcher {
	return &blockingUDCFetcher{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		ctxErr:  make(chan error, 1),
	}
}

func (f *blockingUDCFetcher) fetch(ctx context.Context, _ service.KeyInfo, _ *service.GetUserDelegationCredentialOptions) (*service.UserDelegationCredential, error) {
	f.calls.Add(1)
	f.started <- struct{}{}
	<-f.release
	f.ctxErr <- ctx.Err()
	return &service.UserDelegationCredential{}, nil
}

func TestAzureStorage_userDelegationCredential_ConcurrentCallersShareFetch(t *testing.T) {
	assert := assertion.New(t)
	fetcher := newBlockingUDCFetcher()
	s := &AzureStorage{signedURLExpiry: 5 * time.Minute, getUDC: fetcher.fetch}

	const callers = 10
	results := make([]*service.UserDelegationCredential, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			udc, err := s.userDelegationCredential(context.Background())
			assert.NoError(err)
			results[i] = udc
		}()
	}

	<-fetcher.started
	// Give the other callers time to join the in-flight fetch
	time.Sleep(50 * time.Millisecond)
	close(fetcher.release)
	wg.Wait()

	assert.Equal(int32(1), fetcher.calls.Load())
	for _, udc := range results {
		assert.Same(results[0], udc)
	}
}

func TestAzureStorage_userDelegationCredential_CancelledCallerDoesNotCancelFetch(t *testing.T) {
	assert := assertion.New(t)
	fetcher := newBlockingUDCFetcher()
	s := &AzureStorage{signedURLExpiry: 5 * time.Minute, getUDC: fetcher.fetch}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := s.userDelegationCredential(ctx)
		errCh <- err
	}()

	<-fetcher.started
	cancel()
	assert.ErrorIs(<-errCh, context.Canceled)

	// The fetch keeps running after its caller gives up, and fills the cache for later callers
	close(fetcher.release)
	assert.NoError(<-fetcher.ctxErr)
	assert.Eventually(func() bool {
		udc, _ := s.cachedUDC(time.Now().UTC())
		return udc != nil
	}, time.Second, 10*time.Millisecond)

	_, err := s.userDelegationCredential(context.Background())
	assert.NoError(err)
	assert.Equal(int32(1), fetcher.calls.Load())
}

func TestAzureStorage_userDelegationCredential_RefreshesInBackgroundAfterRefreshThreshold(t *testing.T) {
	assert := assertion.New(t)
	fetcher := newBlockingUDCFetcher()
	old := &service.UserDelegationCredential{}
	s := &AzureStorage{
		signedURLExpiry: 5 * time.Minute,
		getUDC:          fetcher.fetch,
		udc:             old,
		udcExpiry:       time.Now().Add(time.Hour),
		udcRefreshAt:    time.Now().Add(-time.Minute),
	}

	// Callers keep getting the cached credential without waiting while a single refresh is in flight
	for range 5 {
		udc, err := s.userDelegationCredential(context.Background())
		assert.NoError(err)
		assert.Same(old, udc)
	}

	<-fetcher.started
	close(fetcher.release)
	assert.Eventually(func() bool {
		udc, refresh := s.cachedUDC(time.Now().UTC())
		return udc != old && !refresh
	}, time.Second, 10*time.Millisecond)
	assert.Equal(int32(1), fetcher.calls.Load())
}

func TestAzureStorage_userDelegationCredential_FailedBackgroundRefreshKeepsCachedCredential(t *testing.T) {
	assert := assertion.New(t)
	fetcher := &fakeUDCFetcher{err: errors.New("boom")}
	old := &service.UserDelegationCredential{}
	s := &AzureStorage{
		signedURLExpiry: 5 * time.Minute,
		getUDC:          fetcher.fetch,
		udc:             old,
		udcExpiry:       time.Now().Add(time.Hour),
		udcRefreshAt:    time.Now().Add(-time.Minute),
	}

	udc, err := s.userDelegationCredential(context.Background())
	assert.NoError(err)
	assert.Same(old, udc)

	// Joining the in-flight refresh waits for it to finish
	<-s.udcGroup.DoChan("udc", func() (any, error) { return nil, nil })
	cached, refresh := s.cachedUDC(time.Now().UTC())
	assert.Same(old, cached)
	assert.True(refresh)
}

func TestAzureStorage_userDelegationCredential_RefreshThresholdIsHalfwayThroughUsableLifetime(t *testing.T) {
	assert := assertion.New(t)
	fetcher := &fakeUDCFetcher{}
	s := &AzureStorage{signedURLExpiry: 5 * time.Minute, getUDC: fetcher.fetch}

	now := time.Now().UTC()
	_, err := s.userDelegationCredential(context.Background())
	assert.NoError(err)

	usable := azureUDCMinValidity - azureUDCClockSkew
	assert.WithinDuration(now.Add(usable/2), s.udcRefreshAt, time.Second)
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
			name:            "validity is clamped below the Azure maximum of 7 days by the clock skew",
			signedURLExpiry: 200 * time.Hour,
			wantValidity:    azureUDCMaxValidity - azureUDCClockSkew,
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
	tests := []struct {
		name            string
		signedURLExpiry time.Duration
	}{
		{
			name:            "above the Azure maximum of 7 days",
			signedURLExpiry: azureUDCMaxValidity + time.Hour,
		},
		{
			name:            "too long for the key to stay usable for the minimum validity",
			signedURLExpiry: azureUDCMaxValidity - azureUDCClockSkew - azureUDCMinValidity + time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAzureStorage("example", "container", WithAzureStorageSignedUrlExpiry(tc.signedURLExpiry))
			assertion.New(t).ErrorContains(err, "exceeds the Azure user delegation key maximum")
		})
	}
}
