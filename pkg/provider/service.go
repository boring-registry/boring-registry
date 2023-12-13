package provider

import (
	"context"

	"github.com/TierMobility/boring-registry/pkg/core"
)

// Service implements the Provider Registry Protocol.
// For more information see: https://www.terraform.io/docs/internals/provider-registry-protocol.html.
type Service interface {
	GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error)
	ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error)
}

type service struct {
	storage Storage
}

// NewService returns a fully initialized Service.
func NewService(storage Storage) Service {
	return &service{
		storage: storage,
	}
}

func (s *service) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error) {
	return s.storage.GetProvider(ctx, namespace, name, version, os, arch)
}

func (s *service) ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error) {
	return s.storage.ListProviderVersions(ctx, namespace, name)
}
