package provider

import (
	"context"
	"strings"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// Service implements the Provider Registry Protocol.
// For more information see: https://www.terraform.io/docs/internals/provider-registry-protocol.html.
type Service interface {
	GetProvider(ctx context.Context, namespace, name, version, os, arch, proxyUrl string) (*core.Provider, error)
	ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error)
}

type service struct {
	storage Storage
	proxy   bool
}

// NewService returns a fully initialized Service.
func NewService(storage Storage, proxy bool) Service {
	return &service{
		storage: storage,
		proxy:   proxy,
	}
}

func (s *service) GetProvider(ctx context.Context, namespace, name, version, os, arch, proxyUrl string) (*core.Provider, error) {
	p, err := s.storage.GetProvider(ctx, namespace, name, version, os, arch)
	if err != nil {
		return p, err
	}

	if s.proxy {
		downloadFileName := getUrlFileName(p.DownloadURL)
		p.DownloadURL = strings.ReplaceAll(strings.Join([]string{proxyUrl, downloadFileName}, "/"), "//", "/")

		shaSumsFileName := getUrlFileName(p.SHASumsURL)
		p.SHASumsURL = strings.ReplaceAll(strings.Join([]string{proxyUrl, shaSumsFileName}, "/"), "//", "/")

		shaSumsSignatureFileName := getUrlFileName(p.SHASumsSignatureURL)
		p.SHASumsSignatureURL = strings.ReplaceAll(strings.Join([]string{proxyUrl, shaSumsSignatureFileName}, "/"), "//", "/")
	}

	return p, err
}

func (s *service) ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error) {
	return s.storage.ListProviderVersions(ctx, namespace, name)
}

func getUrlFileName(url string) string {
	queryStringIndex := strings.Index(url, "?")
	if queryStringIndex == -1 {
		queryStringIndex = len(url)
	}

	lastPartIndex := strings.LastIndex(url, "/")
	return url[lastPartIndex+1 : queryStringIndex]
}
