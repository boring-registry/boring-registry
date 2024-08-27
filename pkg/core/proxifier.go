package core

import (
	"context"
	"fmt"
	"net/url"
)

// ProxyUrlService represents Boring tool to manage proxyfied downloads.
type ProxyUrlService interface {
	IsProxyEnabled(ctx context.Context) bool
	GetProxyUrl(ctx context.Context, downloadUrl string) (string, error)
}

type proxyUrlService struct {
	IsEnabled bool
	ProxyPath string
}

// NewProxyUrlService returns a fully initialized Proxy.
func NewProxyUrlService(isEnabled bool, proxyPath string) ProxyUrlService {
	return &proxyUrlService{
		IsEnabled: isEnabled,
		ProxyPath: proxyPath,
	}
}

func (p *proxyUrlService) IsProxyEnabled(ctx context.Context) bool {
	return p.IsEnabled
}

func (p *proxyUrlService) GetProxyUrl(ctx context.Context, downloadUrl string) (string, error) {
	parsedUrl, err := url.ParseRequestURI(downloadUrl)
	if err != nil {
		return "", fmt.Errorf("downloadUrl cannot be parsed '%s': %w", downloadUrl, err)
	}

	baseUrl := fmt.Sprintf("%s://%s/", parsedUrl.Scheme, parsedUrl.Host)
	pathUrl := downloadUrl[len(baseUrl):]
	finalUrl := fmt.Sprintf("%s/%s", p.ProxyPath, pathUrl)

	return finalUrl, nil
}
