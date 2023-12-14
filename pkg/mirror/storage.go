package mirror

import (
	"context"
	"io"

	"github.com/boring-registry/boring-registry/pkg/core"
)

type Storage interface {
	// ListMirroredProviderVersions returns all matching provider versions for a given hostname, namespace, and name
	// The provider version can be set to narrow-down the search and return only a single provider
	ListMirroredProviders(ctx context.Context, provider *core.Provider) ([]*core.Provider, error)

	// GetMirroredProvider returns the mirrored provider or a core.ProviderError in case it cannot be located
	GetMirroredProvider(ctx context.Context, provider *core.Provider) (*core.Provider, error)

	// UploadMirroredFile uploads a file that belongs to a provider release
	UploadMirroredFile(ctx context.Context, provider *core.Provider, fileName string, reader io.Reader) error

	// MirroredSigningKeys retrieves the signing keys for mirrored providers
	MirroredSigningKeys(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error)

	// UploadMirroredSigningKeys uploads signing keys for mirrored providers
	// Existing signing keys are overwritten
	UploadMirroredSigningKeys(ctx context.Context, hostname, namespace string, signingKeys *core.SigningKeys) error

	// Retrieve the SHA256SUM from storage
	MirroredSha256Sum(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error)
}
