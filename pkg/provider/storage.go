package provider

import (
	"context"
	"github.com/TierMobility/boring-registry/pkg/core"
)

// Storage represents the Storage of Terraform providers.
type Storage interface {
	GetProvider(ctx context.Context, namespace, name, version, os, arch string) (core.Provider, error)
	ListProviderVersions(ctx context.Context, namespace, name string) ([]core.ProviderVersion, error)
}
