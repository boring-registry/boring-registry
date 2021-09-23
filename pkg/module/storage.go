package module

import (
	"context"
	"io"
)

// Storage represents the repository of Terraform modules.
type Storage interface {
	GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error)
	UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (Module, error)
}
