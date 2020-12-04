package module

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/pkg/errors"
)

// InmemRegistry is a Registry implementation
// This registry is typically used for testing/dev purposes.
type InmemRegistry struct {
	modules    map[string]Module
	moduleData map[string]io.Reader
	mu         sync.RWMutex
}

// GetModule retrieves information about a module from the in-memory registry.
func (s *InmemRegistry) GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	module, ok := s.modules[s.moduleID(namespace, name, provider, version)]
	if !ok {
		return Module{}, errors.Wrap(ErrNotFound, "id")
	}

	return module, nil
}

func (s *InmemRegistry) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var modules []Module

	for _, module := range modules {
		if module.Namespace == namespace && module.Name == name && module.Provider == provider {
			modules = append(modules, module)
		}
	}

	return modules, nil
}

func (s *InmemRegistry) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (Module, error) {
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

func (s *InmemRegistry) moduleID(namespace, name, provider, version string) string {
	return fmt.Sprintf("namespace=%s/name=%s/provider=%s/version=%s", namespace, name, provider, version)
}

// NewInmemRegistry returns a fully initialized in-memory registry.
func NewInmemRegistry() Registry {
	return &InmemRegistry{
		modules:    make(map[string]Module),
		moduleData: make(map[string]io.Reader),
	}
}
