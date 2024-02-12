package module

import (
	"context"
	"io"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// Storage represents the repository of Terraform modules.
type Storage interface {
	// GetModule should return an ErrModuleNotFound error if the requested module version cannot be found
	GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error)
	UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error)
}
