package mirror

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/TierMobility/boring-registry/pkg/discovery"
)

// Service implements the Provider Network Mirror Protocol.
// For more information see: https://www.terraform.io/docs/internals/provider-network-mirror-protocol.html
type Service interface {
	// ListProviderVersions determines which versions are currently available for a particular provider
	// https://www.terraform.io/docs/internals/provider-network-mirror-protocol.html#list-available-versions
	ListProviderVersions(ctx context.Context, provider *core.Provider) (*ListProviderVersionsResponse, error)

	// ListProviderInstallation returns download URLs and associated metadata for the distribution packages for a particular version of a provider
	// https://www.terraform.io/docs/internals/provider-network-mirror-protocol.html#list-available-installation-packages
	ListProviderInstallation(ctx context.Context, provider *core.Provider) (*ListProviderInstallationResponse, error)

	// RetrieveProviderArchive returns an io.Reader of a zip archive containing the provider binary for a given provider
	RetrieveProviderArchive(ctx context.Context, provider *core.Provider) (*retrieveProviderArchiveResponse, error)
}

type mirror struct {
	storage Storage
}

func (m *mirror) ListProviderVersions(ctx context.Context, provider *core.Provider) (*ListProviderVersionsResponse, error) {
	providers, err := m.storage.ListMirroredProviders(ctx, provider)
	if err != nil {
		return nil, err
	}

	response := &ListProviderVersionsResponse{
		Versions:     map[string]EmptyObject{},
		mirrorSource: mirrorSource{isMirror: true},
	}
	for _, p := range providers {
		response.Versions[p.Version] = EmptyObject{}
	}
	return response, nil
}

func (m *mirror) ListProviderInstallation(ctx context.Context, provider *core.Provider) (*ListProviderInstallationResponse, error) {
	providers, err := m.storage.ListMirroredProviders(ctx, provider)
	if err != nil {
		return nil, err
	}

	sha256Sums, err := m.storage.MirroredSha256Sum(ctx, providers[0])
	if err != nil {
		return nil, err
	}
	return toListProviderInstallationResponse(providers, sha256Sums)
}

func (m *mirror) RetrieveProviderArchive(ctx context.Context, provider *core.Provider) (*retrieveProviderArchiveResponse, error) {
	mirrored, err := m.storage.GetMirroredProvider(ctx, provider)
	if err != nil {
		return nil, err
	}

	return &retrieveProviderArchiveResponse{
		location:     mirrored.DownloadURL,
		mirrorSource: mirrorSource{isMirror: true},
	}, nil
}

func NewMirror(s Storage) Service {
	return &mirror{
		storage: s,
	}
}

type pullTroughMirror struct {
	upstream upstreamProvider
	mirror   Service
	copier   Copier
}

func (p *pullTroughMirror) ListProviderVersions(ctx context.Context, provider *core.Provider) (*ListProviderVersionsResponse, error) {
	upstreamCtx, cancelUpstreamCtx := context.WithTimeout(ctx, 10*time.Second)
	defer cancelUpstreamCtx()
	providerVersionsResponse, err := p.upstream.listProviderVersions(upstreamCtx, provider)
	if err == nil {
		// The request to the upstream registry was successful, we can transform and return the response
		return toListProviderVersionsResponse(providerVersionsResponse), nil
	}

	var urlError *url.Error
	if isUrlError := errors.As(err, &urlError); !isUrlError {
		// It's not a network-related error
		return nil, err
	}

	// We try to return a response based on the mirror
	return p.mirror.ListProviderVersions(ctx, provider)
}

func (p *pullTroughMirror) ListProviderInstallation(ctx context.Context, provider *core.Provider) (*ListProviderInstallationResponse, error) {
	upstreamCtx, cancelUpstreamCtx := context.WithTimeout(ctx, 10*time.Second)
	defer cancelUpstreamCtx()
	response, err := p.upstream.listProviderVersions(upstreamCtx, provider)
	if err != nil {
		var urlError *url.Error
		if isUrlError := errors.As(err, &urlError); !isUrlError {
			// It's not a network-related error, therefore we abort the attempt
			return nil, err
		}
	}

	if err == nil && versionExists(provider.Version, response) {
		// The request to the upstream registry was successful, we can return the response
		sha256Sums, err := p.upstreamSha256Sums(ctx, provider, response)
		if err != nil {
			return nil, err
		}
		for _, version := range response.Versions {
			if version.Version != provider.Version {
				continue
			}
			return mergePlatforms(provider, version.Platforms, sha256Sums)
		}
	}

	// Try to retrieve the information from the mirror
	return p.mirror.ListProviderInstallation(ctx, provider)
}

func (p *pullTroughMirror) RetrieveProviderArchive(ctx context.Context, provider *core.Provider) (*retrieveProviderArchiveResponse, error) {
	// If it's in the cache, then redirect to storage
	mirrored, err := p.mirror.RetrieveProviderArchive(ctx, provider)
	if err == nil {
		return mirrored, nil
	}
	var providerError *core.ProviderError
	if !errors.As(err, &providerError) {
		// It's not a core.ProviderError, and therefore an error that needs to be passed to the caller
		return nil, err
	}

	// If not, then redirect to upstream download and start the mirror process
	upstream, err := p.upstream.getProvider(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Download the provider from upstream and upload to the mirror
	go p.copier.copy(upstream)

	return &retrieveProviderArchiveResponse{
		location:     upstream.DownloadURL,
		mirrorSource: mirrorSource{isMirror: false},
	}, nil
}

func (p *pullTroughMirror) upstreamSha256Sums(ctx context.Context, provider *core.Provider, versions *core.ProviderVersions) (*core.Sha256Sums, error) {
	if len(versions.Versions) == 0 || len(versions.Versions[0].Platforms) == 0 {
		return nil, errors.New("core.ProviderVersions doesn't contain any platforms")
	}

	// To retrieve the SHA256SUM, we need to construct a core.Provider that has all required fields set to download the provider from upstream.
	// As the SHA256SUM file is the same for all platforms, we iterate over the versions and choose the first platform from the matching version.
	clone := provider.Clone()
	for _, v := range versions.Versions {
		if v.Version == provider.Version {
			clone.OS = v.Platforms[0].OS
			clone.Arch = v.Platforms[0].Arch
			break
		}
	}

	providerUpstream, err := p.upstream.getProvider(ctx, clone)
	if err != nil {
		return nil, err
	}
	return p.upstream.shaSums(ctx, providerUpstream)
}

func NewPullThroughMirror(s Storage, c Copier) Service {
	remoteServiceDiscovery := discovery.NewRemoteServiceDiscovery(http.DefaultClient)
	svc := &pullTroughMirror{
		upstream: newUpstreamProviderRegistry(remoteServiceDiscovery),
		mirror: &mirror{
			storage: s,
		},
		copier: c,
	}

	return svc
}

func mergePlatforms(provider *core.Provider, platforms []core.Platform, sha256Sums *core.Sha256Sums) (*ListProviderInstallationResponse, error) {
	archives := &ListProviderInstallationResponse{
		Archives:     map[string]Archive{},
		mirrorSource: mirrorSource{isMirror: false},
	}

	for _, p := range platforms {
		provider.OS = p.OS
		provider.Arch = p.Arch

		checksum, err := sha256Sums.Checksum(provider.ArchiveFileName())
		if err != nil {
			return nil, err
		}

		key := fmt.Sprintf("%s_%s", p.OS, p.Arch)
		archives.Archives[key] = Archive{
			Url: provider.ArchiveFileName(),
			Hashes: []string{
				// The checksum has to be prefixed with the `zh:` prefix
				// See the documentation for more context:
				// https://developer.hashicorp.com/terraform/language/files/dependency-lock#zh
				fmt.Sprintf("zh:%s", checksum),
			},
		}
	}

	return archives, nil
}

func toListProviderInstallationResponse(providers []*core.Provider, sha256Sums *core.Sha256Sums) (*ListProviderInstallationResponse, error) {
	archives := &ListProviderInstallationResponse{
		Archives:     map[string]Archive{},
		mirrorSource: mirrorSource{isMirror: true},
	}

	for _, p := range providers {
		checksum, err := sha256Sums.Checksum(p.ArchiveFileName())
		if err != nil {
			return nil, err
		}

		key := fmt.Sprintf("%s_%s", p.OS, p.Arch)
		archives.Archives[key] = Archive{
			Url: p.DownloadURL,
			Hashes: []string{
				// The checksum has to be prefixed with the `zh:` prefix
				// See the documentation for more context:
				// https://developer.hashicorp.com/terraform/language/files/dependency-lock#zh
				fmt.Sprintf("zh:%s", checksum),
			},
		}
	}

	return archives, nil
}

func toListProviderVersionsResponse(l *core.ProviderVersions) *ListProviderVersionsResponse {
	transformed := &ListProviderVersionsResponse{
		Versions:     map[string]EmptyObject{},
		mirrorSource: mirrorSource{isMirror: false},
	}
	for _, version := range l.Versions {
		transformed.Versions[version.Version] = EmptyObject{}
	}
	return transformed
}

func versionExists(version string, versions *core.ProviderVersions) bool {
	for _, v := range versions.Versions {
		if v.Version == version {
			return true
		}
	}

	return false
}
