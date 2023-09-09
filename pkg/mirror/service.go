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
	ListProviderVersions(ctx context.Context, provider *core.Provider) (*ProviderVersions, error)

	// ListProviderInstallation returns download URLs and associated metadata for the distribution packages for a particular version of a provider
	// https://www.terraform.io/docs/internals/provider-network-mirror-protocol.html#list-available-installation-packages
	ListProviderInstallation(ctx context.Context, provider *core.Provider) (*Archives, error)

	// RetrieveProviderArchive returns an io.Reader of a zip archive containing the provider binary for a given provider
	RetrieveProviderArchive(ctx context.Context, provider *core.Provider) (*retrieveProviderArchiveResponse, error)
}

type service struct {
	upstream     upstreamProvider
	storage      Storage
	mirrorCopier Copier
}

func (s *service) ListProviderVersions(ctx context.Context, provider *core.Provider) (*ProviderVersions, error) {
	upstreamCtx, cancelUpstreamCtx := context.WithTimeout(ctx, 10*time.Second)
	defer cancelUpstreamCtx()
	providerVersionsResponse, err := s.upstream.listProviderVersions(upstreamCtx, provider)
	if err == nil {
		// The request to the upstream registry was successful, we can transform and return the response
		return providerVersionsResponse.providerVersions(), nil
	}

	var urlError *url.Error
	if isUrlError := errors.As(err, &urlError); !isUrlError {
		// It's not a network-related error
		return nil, err
	}

	// We try to return a response based on the mirror
	providers, err := s.storage.ListMirroredProviderVersions(ctx, provider)
	if err != nil {
		return nil, err
	}
	response := &ProviderVersions{Versions: map[string]EmptyObject{}, fromMirror: true}
	for _, p := range providers {
		response.Versions[p.Version] = EmptyObject{}
	}
	return response, nil
}

func (s *service) ListProviderInstallation(ctx context.Context, provider *core.Provider) (*Archives, error) {
	upstreamCtx, cancelUpstreamCtx := context.WithTimeout(ctx, 10*time.Second)
	defer cancelUpstreamCtx()
	response, err := s.upstream.listProviderVersions(upstreamCtx, provider)
	if err == nil {
		// The request to the upstream registry was successful, we can return the response
		sha256Sums, err := s.upstreamSha256Sums(ctx, provider, response)
		if err != nil {
			return nil, err
		}
		for _, version := range response.Versions {
			if version.Version != provider.Version {
				continue
			}
			return transformToArchives(provider, version.Platforms, sha256Sums)
		}
	}

	var urlError *url.Error
	if isUrlError := errors.As(err, &urlError); !isUrlError {
		// It's not a network-related error
		return nil, err
	}

	// We try to return a response based on the mirror
	providers, err := s.storage.ListMirroredProviderVersions(ctx, provider)
	if err != nil {
		return nil, err
	}
	if len(providers) > 1 {
		return nil, errors.New("length of returned providers is unexpected")
	}

	sha256Sums, err := s.mirroredSha256Sums(ctx, provider, providers)
	if err != nil {
		return nil, err
	}
	archives, err := transformToArchives(provider, providers[0].Platforms, sha256Sums)
	if err != nil {
		return nil, err
	}
	archives.fromMirror = true
	return archives, nil
}

func (s *service) RetrieveProviderArchive(ctx context.Context, provider *core.Provider) (*retrieveProviderArchiveResponse, error) {
	// If it's in cache, then redirect to storage
	mirrored, err := s.storage.GetMirroredProvider(ctx, provider)
	if err == nil {
		return &retrieveProviderArchiveResponse{
			location:     mirrored.DownloadURL,
			mirrorSource: mirrorSource{isMirror: true},
		}, nil
	}
	var providerError *core.ProviderError
	if !errors.As(err, &providerError) {
		// It's not a core.ProviderError, and therefore an error that needs to be passed to the caller
		return nil, err
	}

	// If not, then redirect to upstream download and start the mirror process
	upstream, err := s.upstream.getProvider(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Download the provider from upstream and upload to the mirror
	go s.mirrorCopier.copy(upstream)

	return &retrieveProviderArchiveResponse{
		location:     upstream.DownloadURL,
		mirrorSource: mirrorSource{isMirror: false},
	}, nil
}

func (s *service) upstreamSha256Sums(ctx context.Context, provider *core.Provider, listResponse *listResponse) (*core.Sha256Sums, error) {
	if len(listResponse.Versions) == 0 || len(listResponse.Versions[0].Platforms) == 0 {
		return nil, errors.New("listResponse doesn't contain any platforms")
	}

	clone := provider.Clone()
	clone.OS = listResponse.Versions[0].Platforms[0].OS
	clone.Arch = listResponse.Versions[0].Platforms[0].Arch
	providerUpstream, err := s.upstream.getProvider(ctx, clone)
	if err != nil {
		return nil, err
	}
	return s.upstream.shaSums(ctx, providerUpstream)
}

func (s *service) mirroredSha256Sums(ctx context.Context, provider *core.Provider, providerVersions []core.ProviderVersion) (*core.Sha256Sums, error) {
	if len(providerVersions) == 0 || len(providerVersions[0].Platforms) == 0 {
		return nil, errors.New("response doesn't contain any platforms")
	}

	clone := provider.Clone()
	clone.OS = providerVersions[0].Platforms[0].OS
	clone.Arch = providerVersions[0].Platforms[0].Arch
	mirroredProvider, err := s.storage.GetMirroredProvider(ctx, clone)
	if err != nil {
		return nil, err
	}
	return s.storage.MirroredSha256Sum(ctx, mirroredProvider)
}

func NewService(s Storage, c Copier) Service {
	remoteServiceDiscovery := discovery.NewRemoteServiceDiscovery(http.DefaultClient)
	svc := &service{
		upstream:     newUpstreamProviderRegistry(remoteServiceDiscovery),
		storage:      s,
		mirrorCopier: c,
	}

	return svc
}

func transformToArchives(provider *core.Provider, platforms []core.Platform, sha256Sums *core.Sha256Sums) (*Archives, error) {
	archives := &Archives{
		Archives: map[string]Archive{},
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
