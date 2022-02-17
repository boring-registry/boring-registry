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
	"golang.org/x/sync/errgroup"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
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

type proxyRegistry struct {
	next   Service // serve most requests via this service
	logger log.Logger

	// upstreamEndpoints uses the hostname as key to re-use clients to the upstream registries.
	// The base URL is set, but the path should be set in the httptransport.EncodeRequestFunc
	upstreamRegistries map[string]endpoint.Endpoint
}

// ListProviderVersions returns the available versions fetched from the upstream registry, as well as from the pull-through cache
func (p *proxyRegistry) ListProviderVersions(ctx context.Context, provider core.Provider) (*ProviderVersions, error) {
	// TODO(oliviermichaelis): the errgroup might not be right, as we want to return both errors in case upstream is down and cache does not contain the provider
	g, _ := errgroup.WithContext(ctx)

	// Get providers from the upstream registry if it is reachable
	upstreamVersions := &ProviderVersions{}
	g.Go(func() error {
		versions, err := p.getUpstreamProviders(ctx, provider)
		if err != nil {
			return err
		}

		// Convert the response to the desired data format
		upstreamVersions = &ProviderVersions{Versions: make(map[string]EmptyObject)}
		for _, version := range versions {
			upstreamVersions.Versions[version.Version] = EmptyObject{}
		}
		return nil
	})

	// Get provider versions from the pull-through cache
	cachedVersions := &ProviderVersions{Versions: make(map[string]EmptyObject)}
	// TODO(oliviermichaelis): check for concurrency problems
	g.Go(func() (err error) {
		providerVersions, err := p.next.ListProviderVersions(ctx, provider)
		if err != nil {
			return err
		}

		// We can only assign cachedVersions once we know that err is non-nil. Otherwise the map is not initialized
		cachedVersions = providerVersions
		return nil
	})

	if err := g.Wait(); err != nil {
		var opError *net.OpError
		var errProviderNotMirrored *storage.ErrProviderNotMirrored
		// Check for net.OpError, as that is an indication for network errors. There is likely a better solution to the problem
		if errors.As(err, &opError) {
			fmt.Println(fmt.Errorf("couldn't reach upstream registry: %v", err)) // TODO(oliviermichaelis): use proper logging
		} else if errors.As(err, &errProviderNotMirrored) {
			fmt.Println(err.Error())
		}
	}

	// Merge both maps together
	for k, v := range upstreamVersions.Versions {
		cachedVersions.Versions[k] = v
	}

	return cachedVersions, nil
}

func (p *proxyRegistry) ListProviderInstallation(ctx context.Context, provider core.Provider) (*Archives, error) {
	// Get archives from the cache
	eg, groupCtx := errgroup.WithContext(ctx)
	results := make(chan *Archives, 2)

	eg.Go(func() error {
		var errProviderNotMirrored *storage.ErrProviderNotMirrored
		res, err := p.next.ListProviderInstallation(groupCtx, provider)
		if errors.As(err, &errProviderNotMirrored) {
			// return from the goroutine without propagating the error, as we've hit an expected error
			_ = level.Info(p.logger).Log(
				"op", "ListProviderInstallation",
				"message", "provider not cached",
				"hostname", provider.Hostname,
				"namespace", provider.Namespace,
				"name", provider.Name,
				"version", provider.Version,
				"err", err,
			)
			return nil
		} else if err != nil {
			// return as we've hit an unforeseen error
			return err
		}
		results <- res
		return nil
	})

	eg.Go(func() error {
		versions, err := p.getUpstreamProviders(groupCtx, provider)
		var opError *net.OpError
		if errors.As(err, &opError) || os.IsTimeout(err) {
			// The error is handled gracefully, as we expect the upstream registry to be down.
			// Therefore we just log the error, but don't return it
			p.logUpstreamError("ListProviderInstallation", provider, err)
			return nil
		} else if err != nil {
			return err
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
					key := fmt.Sprintf("%s_%s", platform.OS, platform.Arch)
					upstreamArchives.Archives[key] = Archive{
						Url: p.ArchiveFileName(),
						// Computing the hash is unfortunately quite complex
						// https://www.terraform.io/language/files/dependency-lock#new-provider-package-checksums
						Hashes: nil,
					}
				}
			}
		}

		results <- upstreamArchives
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results were returned")
	}

	// Warning, this is potentially overwriting locally cached archives. In case a version was deleted from the upstream, we can potentially not serve it locally anymore
	// This could be solved with a more complex merge
	mergedArchive := make(map[string]Archive)
	for len(results) > 0 {
		a := <-results
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
	// Check if there is already an endpoint.Endpoint for the upstream registry, namespace and name
	upstreamUrl, err := url.Parse(fmt.Sprintf("https://%s/v1/providers/%s/%s/versions", provider.Hostname, provider.Namespace, provider.Name))
	if err != nil {
		return nil, err
	}

	// Creating a custom http client with timeout to the upstream registry
	c := http.DefaultClient
	c.Timeout = upstreamTimeout
	clientOption := httptransport.SetClient(c)

	clientEndpoint := httptransport.NewClient(http.MethodGet, upstreamUrl, encodeRequest, decodeUpstreamListProviderVersionsResponse, clientOption).Endpoint()

	response, err := clientEndpoint(ctx, listVersionsRequest{}) // TODO(oliviermichaelis): The object is just a placeholder for now, as we don't have a payload
	if err != nil {
		return nil, err
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

		clientEndpoint = httptransport.NewClient(http.MethodGet, baseURL, encodeUpstreamArchiveDownloadRequest, decodeUpstreamArchiveDownloadResponse).Endpoint()
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

	// TODO(oliviermichaelis): timeout value depends on the server WriteTimeout
	begin := time.Now()
	client := http.Client{Timeout: 30 * time.Second} // need to override default timeout, as the timeout will close the io.ReadCloser from the response body

	binaryResponse, err := client.Get(resp.DownloadURL)
	if err != nil {
		return nil, err
	}

	shasumResponse, err := client.Get(resp.ShasumsURL)
	if err != nil {
		return nil, err
	}

	shasumSignatureResponse, err := client.Get(resp.ShasumsSignatureURL)
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

// TODO(oliviermichaelis): change parameters to use core.Provider
func (p *proxyRegistry) logUpstreamError(op string, provider core.Provider, err error) {
	_ = level.Info(p.logger).Log(
		"op", op,
		"message", "couldn't reach upstream registry",
		"hostname", provider.Hostname,
		"namespace", provider.Namespace,
		"name", provider.Name,
		"version", provider.Version,
		"err", err,
	)
}

func ProxyingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &proxyRegistry{
			next:               next,
			logger:             logger,
			upstreamRegistries: make(map[string]endpoint.Endpoint),
		}
	}
}
