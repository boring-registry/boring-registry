package module

import (
	"context"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// Service implements the Module Registry Protocol.
// For more information see: https://www.terraform.io/docs/internals/module-registry-protocol.html.
type Service interface {
	GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error)
}

type service struct {
	storage Storage
	proxy   core.ProxyUrlService
}

// NewService returns a fully initialized Service.
func NewService(storage Storage, proxy core.ProxyUrlService) Service {
	return &service{
		storage: storage,
		proxy:   proxy,
	}
}

func (s *service) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	res, err := s.storage.GetModule(ctx, namespace, name, provider, version)
	if err != nil {
		return core.Module{}, err
	}

	if s.proxy.IsProxyEnabled(ctx) {
		signedUrl, err := s.proxy.GetProxyUrl(ctx, res.DownloadURL)
		if err != nil {
			return core.Module{}, err
		}

		res.DownloadURL = signedUrl
	}

	return res, err
}

func (s *service) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	res, err := s.storage.ListModuleVersions(ctx, namespace, name, provider)
	if err != nil {
		return nil, err
	}

	return res, nil
}
