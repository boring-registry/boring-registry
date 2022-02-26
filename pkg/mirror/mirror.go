package mirror

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/TierMobility/boring-registry/pkg/storage"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// upstreamArchiveResult is a helper used to bundle multiple return values in a single struct
type upstreamArchiveResult struct {
	providerBinary  *[]byte
	shasum          *[]byte
	shasumSignature *[]byte
}

type componentAlias string

const (
	upstreamComponent = componentAlias("upstream")
	mirrorComponent   = componentAlias("mirror")
)

type ErrLookup struct {
	err       error
	component componentAlias
}

type proxyRegistry struct {
	next   Service // serve most requests via this service
	logger log.Logger

	// upstreamRegistries uses the hostname as key to re-use clients to the upstream registries.
	// The base URL is set, but the path should be set in the httptransport.EncodeRequestFunc
	upstreamRegistries map[string]endpoint.Endpoint
	defaultClient      *http.Client

	// The downloadClient has a much higher timeout in comparison with the defaultClient.
	// Downloading larger provider archives from an upstream server will most likely exceed the default timeout.
	downloadClient *http.Client
}

// ListProviderVersions returns the available versions fetched from the upstream registry, as well as from the pull-through cache
func (p *proxyRegistry) ListProviderVersions(ctx context.Context, provider core.Provider) (*ProviderVersions, error) {
	// Create a WaitGroup in order to wait until cache and upstream lookup finish
	var wg sync.WaitGroup
	wg.Add(2)
	errCh := make(chan ErrLookup, 2)

	// Get providers from the upstream registry if it is reachable
	upstreamVersions := &ProviderVersions{}
	go func() {
		defer wg.Done()

		versions, err := p.getUpstreamProviders(ctx, provider)
		if err != nil {
			errCh <- ErrLookup{
				err:       err,
				component: upstreamComponent,
			}
			return
		}

		// Convert the response to the desired data format
		upstreamVersions = &ProviderVersions{Versions: make(map[string]EmptyObject)}
		for _, version := range versions {
			upstreamVersions.Versions[version.Version] = EmptyObject{}
		}
	}()

	// Get provider versions from the pull-through cache
	cachedVersions := &ProviderVersions{Versions: make(map[string]EmptyObject)}
	go func() {
		defer wg.Done()

		providerVersions, err := p.next.ListProviderVersions(ctx, provider)
		if err != nil {
			errCh <- ErrLookup{
				err:       err,
				component: mirrorComponent,
			}
			return
		}

		// We can only assign cachedVersions once we know that err is non-nil. Otherwise, the map is not initialized
		cachedVersions = providerVersions
	}()

	wg.Wait()
	close(errCh) // Closing the channel so that a corresponding range on the channel doesn't block

	if len(errCh) >= 2 {
		err := &ErrMirrorFailed{
			provider,
			[]error{},
		}

		for e := range errCh {
			err.errors = append(err.errors, e.err)
		}

		return nil, err
	}

	// TODO(oliviermichaelis): add fs.Patherror handling to directory storage.
	if err := p.handleErrors("ListProviderVersions", provider, errCh); err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}

	// Merge both maps together
	for k, v := range upstreamVersions.Versions {
		cachedVersions.Versions[k] = v
	}

	return cachedVersions, nil
}

func (p *proxyRegistry) ListProviderInstallation(ctx context.Context, provider core.Provider) (*Archives, error) {
	// Create a WaitGroup in order to wait until cache and upstream lookup finish
	var wg sync.WaitGroup
	wg.Add(2)
	errCh := make(chan ErrLookup, 2)
	results := make(chan *Archives, 2)

	go func() {
		defer wg.Done()
		res, err := p.next.ListProviderInstallation(ctx, provider)
		if err != nil {
			errCh <- ErrLookup{
				err:       err,
				component: mirrorComponent,
			}
			return
		}
		results <- res
	}()

	go func() {
		defer wg.Done()
		versions, err := p.getUpstreamProviders(ctx, provider)
		if err != nil {
			errCh <- ErrLookup{
				err:       err,
				component: upstreamComponent,
			}
			return
		}

		upstreamArchives := &Archives{
			Archives: make(map[string]Archive),
		}
		for _, v := range versions {
			if v.Version == provider.Version {
				for _, platform := range v.Platforms {
					p := core.Provider{
						Namespace: provider.Namespace,
						Name:      provider.Name,
						Version:   provider.Version,
						OS:        platform.OS,
						Arch:      platform.Arch,
					}

					providerFileName, err := p.ArchiveFileName()
					if err != nil {
						errCh <- ErrLookup{
							err:       err,
							component: upstreamComponent,
						}
						return
					}

					key := fmt.Sprintf("%s_%s", platform.OS, platform.Arch)
					upstreamArchives.Archives[key] = Archive{
						Url: providerFileName,
						// Computing the hash is unfortunately quite complex
						// https://www.terraform.io/language/files/dependency-lock#new-provider-package-checksums
						Hashes: nil,
					}
				}
			}
		}

		results <- upstreamArchives
	}()

	wg.Wait()
	close(errCh)
	close(results)

	// Two errors indicate that both mirror and upstream failed. We can safely return an error in that case
	if len(errCh) >= 2 {
		err := &ErrMirrorFailed{
			provider,
			[]error{},
		}

		for e := range errCh {
			err.errors = append(err.errors, e.err)
		}

		return nil, err
	}

	if len(errCh) > 0 {
		if err := p.handleErrors("ListProviderInstallation", provider, errCh); err != nil {
			return nil, fmt.Errorf("unexpected error: %w", err)
		}
	}

	// Warning, this is potentially overwriting locally cached archives. In case a version was deleted from the upstream, we can potentially not serve it locally anymore
	// This could be solved with a more complex merge
	mergedArchive := make(map[string]Archive)
	for a := range results {
		for k, v := range a.Archives {
			mergedArchive[k] = v
		}
	}

	return &Archives{Archives: mergedArchive}, nil
}

func (p *proxyRegistry) RetrieveProviderArchive(ctx context.Context, provider core.Provider) (io.Reader, error) {
	// retrieve the provider from the local cache if available
	reader, err := p.next.RetrieveProviderArchive(ctx, provider)
	var errProviderNotMirrored *storage.ErrProviderNotMirrored
	if err != nil {
		if !errors.As(err, &errProviderNotMirrored) { // only return on unexpected errors
			return nil, err
		}
	} else {
		return reader, nil
	}

	// download the provider from the upstream registry, as it's not mirrored yet
	upstreamResult, err := p.upstreamProviderArchive(ctx, provider)
	if err != nil {
		return nil, err
	}

	// store the downloaded provider concurrently in the storage backend
	go func() {
		err := p.MirrorProvider(ctx, provider,
			bytes.NewReader(*upstreamResult.providerBinary),
			bytes.NewReader(*upstreamResult.shasum),
			bytes.NewReader(*upstreamResult.shasumSignature),
		)
		if err != nil {
			_ = level.Error(p.logger).Log(
				"message", "failed to store provider",
				"hostname", provider.Hostname,
				"namespace", provider.Namespace,
				"name", provider.Name,
				"version", provider.Version,
				"err", err,
			)
		}
	}()

	return bytes.NewReader(*upstreamResult.providerBinary), nil
}

func (p *proxyRegistry) MirrorProvider(ctx context.Context, provider core.Provider, binary, shasum, shasumSignature io.Reader) error {
	return p.next.MirrorProvider(ctx, provider, binary, shasum, shasumSignature)
}

func (p *proxyRegistry) getUpstreamProviders(ctx context.Context, provider core.Provider) ([]listResponseVersion, error) {
	upstreamUrl, err := url.Parse(fmt.Sprintf("https://%s/v1/providers/%s/%s/versions", provider.Hostname, provider.Namespace, provider.Name))
	if err != nil {
		return nil, err
	}

	clientOption := httptransport.SetClient(p.defaultClient)
	clientEndpoint := httptransport.NewClient(http.MethodGet, upstreamUrl, encodeRequest, decodeUpstreamListProviderVersionsResponse, clientOption).Endpoint()

	response, err := clientEndpoint(ctx, nil) // The request is empty, as we don't have a request body
	if err != nil {
		return nil, &storage.ErrProviderNotMirrored{
			Provider: provider,
			Err:      err,
		}
	}

	resp, ok := response.(listResponse)
	if !ok {
		return nil, fmt.Errorf("failed type assertion for %v", response)
	}
	return resp.Versions, nil
}

func (p *proxyRegistry) upstreamProviderArchive(ctx context.Context, provider core.Provider) (*upstreamArchiveResult, error) {
	clientEndpoint, ok := p.upstreamRegistries[provider.Hostname]
	if !ok {
		baseURL, err := url.Parse(fmt.Sprintf("https://%s", provider.Hostname))
		if err != nil {
			return nil, err
		}

		clientOption := httptransport.SetClient(p.defaultClient)
		clientEndpoint = httptransport.NewClient(http.MethodGet, baseURL, encodeUpstreamArchiveDownloadRequest, decodeUpstreamArchiveDownloadResponse, clientOption).Endpoint()
		p.upstreamRegistries[provider.Hostname] = clientEndpoint
	}

	request := retrieveProviderArchiveRequest{
		Hostname:     provider.Hostname,
		Namespace:    provider.Namespace,
		Name:         provider.Name,
		Version:      provider.Version,
		OS:           provider.OS,
		Architecture: provider.Arch,
	}
	response, err := clientEndpoint(ctx, request)
	if err != nil {
		return nil, err
	}

	resp, ok := response.(downloadResponse)
	if !ok {
		return nil, fmt.Errorf("failed type assertion for %v", response)
	}

	begin := time.Now()

	binaryResponse, err := p.downloadClient.Get(resp.DownloadURL)
	if err != nil {
		return nil, err
	}

	shasumResponse, err := p.downloadClient.Get(resp.ShasumsURL)
	if err != nil {
		return nil, err
	}

	shasumSignatureResponse, err := p.downloadClient.Get(resp.ShasumsSignatureURL)
	if err != nil {
		return nil, err
	}

	// Go expects us to close the Body once we're done reading from it
	defer func() {
		_ = binaryResponse.Body.Close()
		_ = shasumResponse.Body.Close()
		_ = shasumSignatureResponse.Body.Close()
	}()

	_ = level.Info(p.logger).Log(
		"message", "successfully downloaded upstream provider",
		"hostname", provider.Hostname,
		"namespace", provider.Namespace,
		"name", provider.Name,
		"version", provider.Version,
		"took", time.Since(begin),
	)

	binary, err := io.ReadAll(binaryResponse.Body)
	if err != nil {
		return nil, err
	}

	shasum, err := io.ReadAll(shasumResponse.Body)
	if err != nil {
		return nil, err
	}

	shasumSignature, err := io.ReadAll(shasumSignatureResponse.Body)
	if err != nil {
		return nil, err
	}

	return &upstreamArchiveResult{
		providerBinary:  &binary,
		shasum:          &shasum,
		shasumSignature: &shasumSignature,
	}, nil
}

// handleErrors handles lookup errors from upstream and the mirror. It returns the first unexpected error
func (p *proxyRegistry) handleErrors(op string, provider core.Provider, errCh <-chan ErrLookup) error {
	for e := range errCh {
		var errProviderNotMirrored *storage.ErrProviderNotMirrored
		if e.component == upstreamComponent {
			// Check for net.OpError, as that is an indication for network errors. There is likely a better solution to the problem
			var opError *net.OpError
			if errors.As(e.err, &opError) || errors.As(e.err, &errProviderNotMirrored) || os.IsTimeout(e.err) {
				// The error is handled gracefully, as we expect the upstream registry to be down.
				// Therefore we just log the error, but don't return it
				_ = level.Info(p.logger).Log(
					"op", op,
					"message", "couldn't reach upstream registry",
					"hostname", provider.Hostname,
					"namespace", provider.Namespace,
					"name", provider.Name,
					"version", provider.Version,
					"err", e.err,
				)
			} else {
				return e.err
			}
		} else if e.component == mirrorComponent {
			if errors.As(e.err, &errProviderNotMirrored) {
				_ = level.Info(p.logger).Log(
					"op", op,
					"message", "provider not cached",
					"hostname", provider.Hostname,
					"namespace", provider.Namespace,
					"name", provider.Name,
					"Version", provider.Version,
					"err", e.err,
				)
			} else {
				return e.err
			}
		} else {
			return e.err
		}
	}

	return nil
}

func ProxyingMiddleware(logger log.Logger) Middleware {
	defaultClient := http.DefaultClient
	defaultClient.Timeout = upstreamTimeout
	downloadClient := http.DefaultClient
	downloadClient.Timeout = 30 * time.Second

	return func(next Service) Service {
		return &proxyRegistry{
			next:               next,
			logger:             logger,
			upstreamRegistries: make(map[string]endpoint.Endpoint),
			defaultClient:      defaultClient,
			downloadClient:     downloadClient,
		}
	}
}
