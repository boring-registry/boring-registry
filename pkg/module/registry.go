package module

import (
	"context"
	"io"
)

// Registry represents the Registrysitory of Terraform modules.
type Registry interface {
	GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error)
	UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (Module, error)
}
