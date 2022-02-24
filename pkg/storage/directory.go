package storage

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/TierMobility/boring-registry/pkg/provider"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mirrorPrefix          = "mirror"
	customProvidersPrefix = "providers"
)

// DirectoryStorage implements mirror.Storage
type DirectoryStorage struct {
	rwMutex sync.RWMutex
	path    string
}

// EnumerateMirroredProviders stems from mirror.Storage
func (d *DirectoryStorage) EnumerateMirroredProviders(ctx context.Context, provider core.Provider) (*[]core.Provider, error) {
	return d.getProviders(ctx, mirrorPrefix, provider)
}

// RetrieveMirroredProviderArchive stems from mirror.Storage
func (d *DirectoryStorage) RetrieveMirroredProviderArchive(ctx context.Context, provider core.Provider) (io.ReadCloser, error) {
	fileName, err := provider.ArchiveFileName()
	if err != nil {
		return nil, err
	}

	f := fmt.Sprintf("%s/%s/%s/%s/%s/%s", d.path, mirrorPrefix, provider.Hostname, provider.Namespace, provider.Name, fileName)
	file, err := os.Open(f)
	if err != nil {
		return nil, &ErrProviderNotMirrored{
			Err:      err,
			Provider: provider,
		}
	}

	return io.NopCloser(bufio.NewReader(file)), nil
}

// StoreMirroredProvider stems from mirror.Storage
func (d *DirectoryStorage) StoreMirroredProvider(ctx context.Context, provider core.Provider, binary, shasum, shasumSignature io.Reader) error {
	// Acquiring lock, as the operation is not an atomic filesystem operation
	d.rwMutex.Lock()
	defer d.rwMutex.Unlock()

	providers, err := d.getProviders(ctx, mirrorPrefix, provider)
	var errProviderNotMirrored *ErrProviderNotMirrored
	if err != nil {
		if !errors.As(err, &errProviderNotMirrored) {
			return err // return on unexpected errors
		}
	} else if len(*providers) != 0 {
		return fmt.Errorf("can't store provider as it exists already")
	}

	fileName, err := provider.ArchiveFileName()
	if err != nil {
		return err
	}

	providerDir := fmt.Sprintf("%s/%s/%s/%s/%s", d.path, mirrorPrefix, provider.Hostname, provider.Namespace, provider.Name)
	providerShasumFilename, err := provider.ShasumFileName()
	if err != nil {
		return err
	}
	providerShasumSignatureFilename, err := provider.ShasumSignatureFileName()
	if err != nil {
		return err
	}

	inputs := []struct {
		path   string
		reader io.Reader
	}{
		{
			path:   fmt.Sprintf("%s/%s", providerDir, fileName),
			reader: binary,
		},
		{
			path:   fmt.Sprintf("%s/%s", providerDir, providerShasumFilename),
			reader: shasum,
		},
		{
			path:   fmt.Sprintf("%s/%s", providerDir, providerShasumSignatureFilename),
			reader: shasumSignature,
		},
	}

	for _, input := range inputs {
		// ensure directory exists
		if err := os.MkdirAll(path.Dir(input.path), 0755); err != nil {
			return err
		}

		f, err := os.Create(input.path)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, input.reader)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *DirectoryStorage) GetModule(ctx context.Context, namespace, name, provider, version string) (module.Module, error) {
	panic("implement me")
}

func (d *DirectoryStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]module.Module, error) {
	panic("implement me")
}

func (d *DirectoryStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (module.Module, error) {
	panic("implement me")
}

func (d *DirectoryStorage) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (provider.Provider, error) {
	panic("getProvider")
}

func (d *DirectoryStorage) ListProviderVersions(ctx context.Context, namespace, name string) ([]provider.ProviderVersion, error) {
	providerDir := fmt.Sprintf("%s/providers", d.path)
	var files []string
	err := filepath.WalkDir(providerDir,
		func(path string, dir fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !dir.IsDir() {
				files = append(files, path)
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	// Shorten the provider paths for further processing into provider
	collection := provider.NewCollection()
	for _, f := range files {
		trim := strings.TrimPrefix(f, providerDir)
		p, err := provider.Parse(trim)
		if err != nil {
			return nil, err
		}

		collection.Add(p)
	}

	return collection.List(), nil
}

func (d *DirectoryStorage) getProviders(ctx context.Context, prefix string, provider core.Provider) (*[]core.Provider, error) {
	p := fmt.Sprintf("%s/%s/%s/%s/%s", d.path, prefix, provider.Hostname, provider.Namespace, provider.Name)
	rootDir := filepath.Clean(p) // remove trailing p separators
	var archives []string
	err := filepath.Walk(rootDir,
		func(path string, file fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// skip directories
			if file.IsDir() {
				return nil
			}

			// skip if file extension does not end with `.zip`
			if filepath.Ext(path) != core.ProviderExtension {
				return nil
			}

			archives = append(archives, path)
			return nil
		})
	if err != nil {
		return nil, &ErrProviderNotMirrored{
			Provider: provider,
			Err:      err,
		}
	}

	var providers []core.Provider
	for _, a := range archives {
		p, err := core.NewProviderFromArchive(a)
		if err != nil {
			return nil, err
		}

		// Filter out providers that don't match the queried version
		if provider.Version != "" {
			if p.Version != provider.Version {
				continue
			}
		}

		shasumFilename, err := p.ShasumFileName()
		if err != nil {
			return nil, err
		}

		// Retrieve the SHASUM if it exists
		shasumFilePath := fmt.Sprintf("%s/%s", path.Dir(a), shasumFilename)
		f, err := os.Open(shasumFilePath)
		if err != nil {
			return nil, err
		}
		p.Shasum, err = ReadSHASums(f, p)
		if err != nil {
			// Even though the hash is optional, we're failing the operation here
			return nil, err
		}
		_ = f.Close() // file needs to be closed explicitly instead of deferred to prevent resource leak in loop

		providers = append(providers, p)
	}

	return &providers, nil
}

func NewDirectoryStorage(path string) (*DirectoryStorage, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Check if directory exists
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}

	return &DirectoryStorage{
		path: p,
	}, nil
}
