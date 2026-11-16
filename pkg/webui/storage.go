package webui

import (
	"context"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// Storage is the read-only view of the storage backend used by the web UI.
type Storage interface {
	// ListAllModules returns every module version in storage. DownloadURL is left empty.
	ListAllModules(ctx context.Context) ([]core.Module, error)
	// ListAllProviders returns one entry per internal provider archive (Namespace, Name, Version, OS, Arch, Filename set; no URLs).
	ListAllProviders(ctx context.Context) ([]*core.Provider, error)
	// ListAllMirroredProviders is ListAllProviders for mirror/providers/<hostname>/...; Hostname is set.
	ListAllMirroredProviders(ctx context.Context) ([]*core.Provider, error)
	// ModuleArchive returns the raw module archive bytes, or module.ErrModuleNotFound.
	ModuleArchive(ctx context.Context, namespace, name, provider, version string) ([]byte, error)
	// SigningKeys is identical to provider.Storage.SigningKeys.
	SigningKeys(ctx context.Context, namespace string) (*core.SigningKeys, error)
}
