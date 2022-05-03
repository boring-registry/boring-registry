package module

import (
	"context"
	"fmt"
	"io"
	"path"
)

const (
	DefaultArchiveFormat = "tar.gz"
)

var (
	archiveFormat = DefaultArchiveFormat
)

func SetArchiveFormat(newFormat string) {
	archiveFormat = newFormat
}

// Storage represents the repository of Terraform modules.
type Storage interface {
	GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error)
	UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (Module, error)
}

func storagePrefix(prefix, namespace, name, provider string) string {
	return path.Join(
		prefix,
		fmt.Sprintf("namespace=%s", namespace),
		fmt.Sprintf("name=%s", name),
		fmt.Sprintf("provider=%s", provider),
	)
}

func storagePath(prefix, namespace, name, provider, version string) string {
	return path.Join(
		prefix,
		fmt.Sprintf("namespace=%s", namespace),
		fmt.Sprintf("name=%s", name),
		fmt.Sprintf("provider=%s", provider),
		fmt.Sprintf("version=%s", version),
		fmt.Sprintf("%s-%s-%s-%s.%s", namespace, name, provider, version, archiveFormat),
	)
}
