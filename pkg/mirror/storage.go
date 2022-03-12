package mirror

import (
	"context"
	"github.com/TierMobility/boring-registry/pkg/core"
	"io"
)

type Storage interface {
	// EnumerateMirroredProviders retrieves all matching providers from the storage backend.
	//
	// The core.Provider input parameter has to be non-nil and
	// the Hostname, Namespace and Name attributes have to be set.
	// Optionally, the Version can be set in order to narrow down the search.
	EnumerateMirroredProviders(ctx context.Context, provider core.Provider) (*[]core.Provider, error)

	// RetrieveMirroredProviderArchive returns an io.ReadCloser of a mirrored provider archive.
	//
	// The core.Provider input parameter has to be non-nil and
	// the Hostname, Namespace, Name, Version, OS, and Arch need to be set.
	//
	// If a provider is not mirrored, a storage.ErrProviderNotMirrored error is returned.
	RetrieveMirroredProviderArchive(ctx context.Context, provider core.Provider) (io.ReadCloser, error)

	// StoreMirroredProvider stores a given provider archive, and it's SHA sum and signature files in the storage backend.
	//
	// The core.Provider input parameter has to be non-nil and
	// the Hostname, Namespace, Name, Version, OS, and Arch need to be set.
	StoreMirroredProvider(ctx context.Context, provider core.Provider, binary, shasum, shasumSignature io.Reader) error
}
