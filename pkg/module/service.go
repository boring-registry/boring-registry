package module

import (
	"context"
	"strings"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// Service implements the Module Registry Protocol.
// For more information see: https://www.terraform.io/docs/internals/module-registry-protocol.html.
type Service interface {
	GetModule(ctx context.Context, namespace, name, provider, version, proxyUrl string) (core.Module, error)
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error)
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

func (s *service) GetModule(ctx context.Context, namespace, name, provider, version, proxyUrl string) (core.Module, error) {
	res, err := s.storage.GetModule(ctx, namespace, name, provider, version)
	if err != nil {
		return core.Module{}, err
	}

	if s.proxy {
		downloadFileName := getUrlFileName(res.DownloadURL)
		res.DownloadURL = strings.ReplaceAll(strings.Join([]string{proxyUrl, downloadFileName}, "/"), "//", "/")
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

func getUrlFileName(url string) string {
	queryStringIndex := strings.Index(url, "?")
	if queryStringIndex == -1 {
		queryStringIndex = len(url)
	}

	lastPartIndex := strings.LastIndex(url, "/")
	return url[lastPartIndex+1 : queryStringIndex]
}
