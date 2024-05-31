package provider

import (
	"context"
	"fmt"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/proxy"
)

// Service implements the Provider Registry Protocol.
// For more information see: https://www.terraform.io/docs/internals/provider-registry-protocol.html.
type Service interface {
	GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error)
	ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error)
}

type service struct {
	storage Storage
	proxy   proxy.ProxyUrlService
}

// NewService returns a fully initialized Service.
func NewService(storage Storage, proxy proxy.ProxyUrlService) Service {
	return &service{
		storage: storage,
		proxy:   proxy,
	}
}

func (s *service) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error) {
	p, err := s.storage.GetProvider(ctx, namespace, name, version, os, arch)
	if err != nil {
		return p, err
	}

	if s.proxy.IsProxyEnabled(ctx) {
		rootUrl, ok := ctx.Value(proxy.RootUrlContextKey).(string)
		if !ok {
			return nil, fmt.Errorf("%w: rootUrl is not in context", core.ErrVarMissing)
		}

		signedUrl, err := s.proxy.GetSignedUrl(ctx, p.DownloadURL, rootUrl)
		if err != nil {
			return p, err
		}

		p.DownloadURL = signedUrl
	}

	return p, err
}

func (s *service) ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error) {
	return s.storage.ListProviderVersions(ctx, namespace, name)
}
