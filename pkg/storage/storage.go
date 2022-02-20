package storage

import (
	"context"
	"github.com/TierMobility/boring-registry/pkg/provider"
)

// Storage TODO(oliviermichaelis): refactor everything
type Storage interface {
	//GetModule(ctx context.Context, namespace, name, provider, version string) (module.Module, error)
	//ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]module.Module, error)
	//UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (module.Module, error)

	ListProviderVersions(ctx context.Context, namespace, name string) ([]provider.ProviderVersion, error)
	//GetProvider(ctx context.Context, namespace, name, version, os, arch string) (provider.Provider, error)
}
