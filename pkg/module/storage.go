package module

import (
	"context"
	"github.com/TierMobility/boring-registry/pkg/core"
	"io"
)

// Storage represents the repository of Terraform modules.
type Storage interface {
	GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error)
	UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error)
}
