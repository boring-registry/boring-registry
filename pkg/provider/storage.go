package provider

import (
	"context"

	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/kit/log"
)

// Storage represents the Storage of Terraform providers.
type Storage interface {
	GetProvider(ctx context.Context, namespace, name, version, os, arch string) (core.Provider, error)
	ListProviderVersions(ctx context.Context, namespace, name string) ([]core.ProviderVersion, error)

	// MigrateProviders is needed for the migration from 0.7.0 to 0.8.0
	MigrateProviders(ctx context.Context, logger log.Logger, dryRun bool) error
}
