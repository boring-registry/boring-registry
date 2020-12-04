package module

import (
	"context"
	"fmt"
)

// Service implements the Module Registry Protocol.
// For more information see: https://www.terraform.io/docs/internals/module-registry-protocol.html.
type Service interface {
	GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error)
}

type service struct {
	registry Registry
}

// NewService returns a fully initialized Service.
func NewService(registry Registry) Service {
	return &service{
		registry: registry,
	}
}

func (s *service) GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error) {
	res, err := s.registry.GetModule(ctx, namespace, name, provider, version)
	if err != nil {
		return Module{}, err
	}

	return res, nil
}

func (s *service) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error) {
	res, err := s.registry.ListModuleVersions(ctx, namespace, name, provider)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Module represents Terraform module metadata.
type Module struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
}

// ID returns the module metadata in a compact format.
func (m *Module) ID(version bool) string {
	id := fmt.Sprintf("namespace=%s/name=%s/provider=%s", m.Namespace, m.Name, m.Provider)
	if version {
		id = fmt.Sprintf("%s/version=%s", id, m.Version)
	}

	return id
}
