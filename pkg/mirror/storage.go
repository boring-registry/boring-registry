package mirror

import (
	"context"
	"github.com/TierMobility/boring-registry/pkg/core"
	"io"
)

type Storage interface {
	EnumerateMirroredProviders(ctx context.Context, provider core.Provider) (*[]core.Provider, error)
	RetrieveMirroredProviderArchive(ctx context.Context, provider core.Provider) (io.ReadCloser, error)
	StoreMirroredProvider(ctx context.Context, provider core.Provider, reader io.Reader) error
}
