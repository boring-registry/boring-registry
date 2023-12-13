package module

import (
	"context"
	"io"

	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/kit/log"
)

// Storage represents the repository of Terraform modules.
type Storage interface {
	// GetModule should return an ErrModuleNotFound error if the requested module version cannot be found
	GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error)
	UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error)

	// MigrateModules is needed for the migration from 0.7.0 to 0.8.0
	MigrateModules(ctx context.Context, logger log.Logger, dryRun bool) error
}
