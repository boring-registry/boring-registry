package provider

import (
	"context"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// Service implements the Provider Registry Protocol.
// For more information see: https://www.terraform.io/docs/internals/provider-registry-protocol.html.
type Service interface {
	GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error)
	ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error)
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

func (s *service) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error) {
	p, err := s.storage.GetProvider(ctx, namespace, name, version, os, arch)
	if err != nil {
		return p, err
	}

	if s.proxy.IsProxyEnabled(ctx) {
		signedUrl, err := s.proxy.GetProxyUrl(ctx, p.DownloadURL)
		if err != nil {
			return p, err
		}
		p.DownloadURL = signedUrl

		signedUrl, err = s.proxy.GetProxyUrl(ctx, p.SHASumsURL)
		if err != nil {
			return p, err
		}
		p.SHASumsURL = signedUrl

		signedUrl, err = s.proxy.GetProxyUrl(ctx, p.SHASumsSignatureURL)
		if err != nil {
			return p, err
		}
		p.SHASumsSignatureURL = signedUrl
	}

	return p, err
}

func (s *service) ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error) {
	return s.storage.ListProviderVersions(ctx, namespace, name)
}
