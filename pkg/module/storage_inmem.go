package module

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/pkg/errors"
)

// InmemStorage is a Storage implementation
// This storage is typically used for testing purposes.
type InmemStorage struct {
	modules       map[string]Module
	moduleData    map[string]io.Reader
	mu            sync.RWMutex
	archiveFormat string
}

// GetModule retrieves information about a module from the in-memory storage.
func (s *InmemStorage) GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	module, ok := s.modules[s.moduleID(namespace, name, provider, version)]
	if !ok {
		return Module{}, errors.Wrap(ErrNotFound, "id")
	}

	return module, nil
}

func (s *InmemStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var modules []Module

	for _, module := range s.modules {
		if module.Namespace == namespace && module.Name == name && module.Provider == provider {
			module.DownloadURL = storagePath("inmem", namespace, name, provider, module.Version, s.archiveFormat)
			modules = append(modules, module)
		}
	}

	if len(modules) == 0 {
		return nil, errors.Errorf("no modules found for namespace=%s name=%s provider=%s", namespace, name, provider)
	}

	return modules, nil
}

func (s *InmemStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (Module, error) {
	if namespace == "" {
		return Module{}, errors.New("namespace not defined")
	}

	if name == "" {
		return Module{}, errors.New("name not defined")
	}

	if provider == "" {
		return Module{}, errors.New("provider not defined")
	}

	if version == "" {
		return Module{}, errors.New("version not defined")
	}

	s.mu.Lock()

	id := s.moduleID(namespace, name, provider, version)
	if _, ok := s.modules[id]; ok {
		return Module{}, errors.Wrap(ErrAlreadyExists, "id")
	}

	s.modules[id] = Module{
		Namespace: namespace,
		Name:      name,
		Provider:  provider,
		Version:   version,
	}

	s.moduleData[id] = body
	s.mu.Unlock()

	return s.GetModule(ctx, namespace, name, provider, version)
}

func (s *InmemStorage) moduleID(namespace, name, provider, version string) string {
	return fmt.Sprintf("namespace=%s/name=%s/provider=%s/version=%s/format=%s", namespace, name, provider, version, s.archiveFormat)
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
		modules:       make(map[string]Module),
		moduleData:    make(map[string]io.Reader),
		archiveFormat: DefaultArchiveFormat,
	}

	for _, option := range options {
		option(s)
	}

	return s
}
