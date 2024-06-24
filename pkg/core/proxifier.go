package core

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	httptransport "github.com/go-kit/kit/transport/http"
)

type muxVar string

const (
	RootUrlContextKey muxVar = "rootUrl"
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
	rootUrl, ok := ctx.Value(RootUrlContextKey).(string)
	if !ok {
		return "", fmt.Errorf("%w: rootUrl is not in context", ErrVarMissing)
	}

	parsedUrl, err := url.Parse(downloadUrl)
	if err != nil {
		return "", fmt.Errorf("downloadUrl cannot be parsed: %s", downloadUrl)
	}

	baseUrl := fmt.Sprintf("%s://%s/", parsedUrl.Scheme, parsedUrl.Host)
	pathUrl := downloadUrl[len(baseUrl):]
	excapedUrl := url.QueryEscape(pathUrl)
	finalUrl := fmt.Sprintf("%s%s/%s", rootUrl, p.ProxyPath, excapedUrl)
	return finalUrl, nil
}

// ExtractRootUrl return an URl composed of the scheme (http or https) and the host of the incoming request
func ExtractRootUrl() httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		rootUrl := getRootURLFromRequest(r)

		// Add the rootUrl to the context
		ctx = context.WithValue(ctx, RootUrlContextKey, rootUrl)

		return ctx
	}
}

// Get the root part of the URL of the request
func getRootURLFromRequest(r *http.Request) string {
	// Find the protocol (http ou https)
	var protocol string
	if r.TLS != nil {
		protocol = "https://"
	} else {
		protocol = "http://"
	}

	// Build root URL
	rootUrl := protocol + r.Host
	return rootUrl
}
