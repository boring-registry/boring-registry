package module

import (
	"context"
	"fmt"
	"io"
	"path"
	"sync"

	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

// InmemStorage is a Storage implementation
// This storage is typically used for testing purposes.
type InmemStorage struct {
	mu            sync.RWMutex
	modules       map[string]core.Module
	moduleData    map[string]io.Reader
	archiveFormat string
}

// GetModule retrieves information about a module from the in-memory storage.
func (s *InmemStorage) GetModule(_ context.Context, namespace, name, provider, version string) (core.Module, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := core.Module{
		Namespace: namespace,
		Name:      name,
		Provider:  provider,
		Version:   version,
	}
	module, ok := s.modules[m.ID(true)]
	if !ok {
		return core.Module{}, errors.Wrap(errors.New("module not found"), "id")
	}

	return module, nil
}

func (s *InmemStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var modules []core.Module

	for _, module := range s.modules {
		if module.Namespace == namespace && module.Name == name && module.Provider == provider {
			f := fmt.Sprintf("%s-%s-%s-%s.%s", namespace, name, provider, module.Version, s.archiveFormat)
			module.DownloadURL = path.Join("prefix", "inmem", namespace, name, provider, f)
			modules = append(modules, module)
		}
	}

	if len(modules) == 0 {
		return nil, errors.Errorf("no modules found for namespace=%s name=%s provider=%s", namespace, name, provider)
	}

	return modules, nil
}

func (s *InmemStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
	if namespace == "" {
		return core.Module{}, errors.New("namespace not defined")
	}

	if name == "" {
		return core.Module{}, errors.New("name not defined")
	}

	if provider == "" {
		return core.Module{}, errors.New("provider not defined")
	}

	if version == "" {
		return core.Module{}, errors.New("version not defined")
	}

	s.mu.Lock()

	m := core.Module{
		Namespace: namespace,
		Name:      name,
		Provider:  provider,
		Version:   version,
	}

	id := m.ID(true)
	if _, ok := s.modules[id]; ok {
		return core.Module{}, errors.Wrap(errors.New("exists already"), "id")
	}

	s.modules[id] = m

	s.moduleData[id] = body
	s.mu.Unlock()

	return s.GetModule(ctx, namespace, name, provider, version)
}

func (s *InmemStorage) MigrateModules(ctx context.Context, logger log.Logger, dryRun bool) error {
	panic("MigrateModules should not be called for InmemStorage")
}

// InmemStorageOption provides additional options for the InmemStorage.
type InmemStorageOption func(*InmemStorage)

// WithInmemArchiveFormat configures the module archive format (zip, tar, tgz, etc.)
func WithInmemArchiveFormat(archiveFormat string) InmemStorageOption {
	return func(s *InmemStorage) {
		s.archiveFormat = archiveFormat
	}
}

// NewInmemStorage returns a fully initialized in-memory storage.
func NewInmemStorage(options ...InmemStorageOption) Storage {
	s := &InmemStorage{
		modules:       make(map[string]core.Module),
		moduleData:    make(map[string]io.Reader),
		archiveFormat: "tar.gz",
	}

	for _, option := range options {
		option(s)
	}

	return s
}
