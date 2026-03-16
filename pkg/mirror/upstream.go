package mirror

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/discovery"
)

type upstreamProvider interface {
	listProviderVersions(ctx context.Context, provider *core.Provider) (*core.ProviderVersions, error)
	getProvider(ctx context.Context, provider *core.Provider) (*core.Provider, error)
	shaSums(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error)
}

type upstreamProviderRegistry struct {
	client                 *http.Client
	remoteServiceDiscovery discovery.ServiceDiscoveryResolver
}

func (u *upstreamProviderRegistry) listProviderVersions(ctx context.Context, provider *core.Provider) (*core.ProviderVersions, error) {
	discovered, err := u.remoteServiceDiscovery.Resolve(ctx, provider.Hostname)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s%s/%s/versions", discovered.ProvidersV1, provider.Namespace, provider.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL(discovered.URL.Host, path), nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		// we ignore the error here, as there is not much we can do about it at this point
		_ = resp.Body.Close()
	}()

	return decodeUpstreamListProviderVersionsResponse(resp)
}

func (u *upstreamProviderRegistry) getProvider(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
	discovered, err := u.remoteServiceDiscovery.Resolve(ctx, provider.Hostname)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s%s/%s/%s/download/%s/%s", discovered.ProvidersV1, provider.Namespace, provider.Name, provider.Version, provider.OS, provider.Arch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL(discovered.URL.Host, path), nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		// we ignore the error here, as there is not much we can do about it at this point
		_ = resp.Body.Close()
	}()

	decoded, err := decodeUpstreamProviderResponse(resp)
	if err != nil {
		return nil, err
	}

	// Merge provider into decoded to fully populate the struct
	decoded.Hostname = provider.Hostname
	decoded.Namespace = provider.Namespace
	decoded.Name = provider.Name
	decoded.Version = provider.Version

	return decoded, err
}

func (u *upstreamProviderRegistry) shaSums(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.SHASumsURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	sha256Sums, err := core.NewSha256Sums(provider.ShasumFileName(), resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SHA256SUM: %w", err)
	}
	return sha256Sums, nil
}

func newUpstreamProviderRegistry(remoteServiceDiscovery discovery.ServiceDiscoveryResolver) *upstreamProviderRegistry {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 100
	return &upstreamProviderRegistry{
		client: &http.Client{
			Transport: transport,
		},
		remoteServiceDiscovery: remoteServiceDiscovery,
	}
}

func upstreamURL(hostname, path string) string {
	upstreamUrl := url.URL{
		Scheme: "https",
		Host:   hostname,
		Path:   path,
	}
	return upstreamUrl.String()
}
