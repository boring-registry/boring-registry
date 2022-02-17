package mirror

import (
	"context"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
	"io"
)

// Service implements the Provider Network MirrorProtocol.
// For more information see: https://www.terraform.io/docs/internals/provider-network-mirror-protocol.html
type Service interface {
	// ListProviderVersions determines which versions are currently available for a particular provider
	// https://www.terraform.io/docs/internals/provider-network-mirror-protocol.html#list-available-versions
	ListProviderVersions(ctx context.Context, provider core.Provider) (*ProviderVersions, error)

	// ListProviderInstallation returns download URLs and associated metadata for the distribution packages for a particular version of a provider
	// https://www.terraform.io/docs/internals/provider-network-mirror-protocol.html#list-available-installation-packages
	ListProviderInstallation(ctx context.Context, provider core.Provider) (*Archives, error)

	// RetrieveProviderArchive returns an io.Reader of a zip archive containing the provider binary for a given provider
	RetrieveProviderArchive(ctx context.Context, provider core.Provider) (io.Reader, error)

	// MirrorProvider stores the provider zip archive in the configured storage backend
	// The operation has to be idempotent, as a provider could be mirrored multiple times at the same time, possibly also from multiple replicas of the service
	MirrorProvider(ctx context.Context, provider core.Provider, binary, shasum, shasumSignature io.Reader) error
}

type service struct {
	storage Storage
}

func (s *service) ListProviderVersions(ctx context.Context, provider core.Provider) (*ProviderVersions, error) {
	providers, err := s.storage.EnumerateMirroredProviders(ctx, provider)
	if err != nil {
		return nil, err
	}

	return newProviderVersions(providers), nil
}

func (s *service) ListProviderInstallation(ctx context.Context, provider core.Provider) (*Archives, error) {
	providers, err := s.storage.EnumerateMirroredProviders(ctx, provider)
	if err != nil {
		return nil, err
	}

	archives := &Archives{Archives: make(map[string]Archive)}
	for _, p := range *providers {
		key := fmt.Sprintf("%s_%s", p.OS, p.Arch)
		archives.Archives[key] = Archive{
			Url: p.ArchiveFileName(),
			// Computing the hash is unfortunately quite complex
			// https://www.terraform.io/language/files/dependency-lock#new-provider-package-checksums
			Hashes: nil,
		}
	}

	return archives, nil
}

func (s *service) RetrieveProviderArchive(ctx context.Context, provider core.Provider) (io.Reader, error) {
	return s.storage.RetrieveMirroredProviderArchive(ctx, provider)
}

func (s *service) MirrorProvider(ctx context.Context, p core.Provider, binary, shamsum, shasumSignature io.Reader) error {
	return s.storage.StoreMirroredProvider(ctx, p, binary, shamsum, shasumSignature)
}

// NewService returns a fully initialized Service.
func NewService(storage Storage) Service {
	return &service{
		storage: storage,
	}
}

// EmptyObject exists to return an `{}` JSON object to match the protocol spec
type EmptyObject struct{}

// TODO(oliviermichaelis): could be renamed as it clashes with the other core.ProviderVersion

// ProviderVersions holds the response that is passed up to the endpoint
type ProviderVersions struct {
	Versions map[string]EmptyObject `json:"versions"`
}

func newProviderVersions(providers *[]core.Provider) *ProviderVersions {
	p := &ProviderVersions{
		Versions: make(map[string]EmptyObject),
	}

	for _, provider := range *providers {
		p.Versions[provider.Version] = EmptyObject{}
	}
	return p
}

type Archives struct {
	Archives map[string]Archive `json:"archives"`
}

type Archive struct {
	Url    string   `json:"url"`
	Hashes []string `json:"hashes,omitempty"`
}
