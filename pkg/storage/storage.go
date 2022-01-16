package storage

import (
	"context"
	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/TierMobility/boring-registry/pkg/provider"
	"io"
)

// Storage TODO(oliviermichaelis): refactor everything
// Storage interface can only be used for mirror right now
type Storage interface {
	GetModule(ctx context.Context, namespace, name, provider, version string) (module.Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]module.Module, error)
	UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (module.Module, error)

	ListProviderVersions(ctx context.Context, namespace, name string) ([]provider.ProviderVersion, error)
	GetProvider(ctx context.Context, namespace, name, version, os, arch string) (provider.Provider, error)

	GetMirroredProviders(ctx context.Context, opts ProviderOpts) (*[]core.Provider, error)
	GetCustomProviders(ctx context.Context, opts ProviderOpts) (*[]core.Provider, error)
	GetProviderArchive(ctx context.Context, hostname string, p core.Provider) (io.ReadCloser, error)
	StoreProvider(ctx context.Context, hostname string, provider core.Provider, reader io.Reader) error
}

type ProviderOpts struct {
	Hostname  string
	Namespace string
	Name      string
	Version   string
	OS        string
	Arch      string
}
