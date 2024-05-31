package proxy

import (
	"context"
	"io"
	"net/http"

	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	"github.com/go-kit/kit/endpoint"
	"github.com/prometheus/client_golang/prometheus"
)

type proxyRequest struct {
	signature string
	expiry    int64
	url       string
}

type proxyResponse struct {
	StatusCode int
	Body       io.ReadCloser
	Header     http.Header
}

func proxyEndpoint(proxy ProxyUrlService, metrics *o11y.ProxyMetrics) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		input := request.(proxyRequest)

		metrics.Download.With(prometheus.Labels{}).Inc()

		isExpired := proxy.IsExpired(ctx, input.expiry)
		if isExpired {
			metrics.Failure.With(prometheus.Labels{
				o11y.ProxyFailureLabel: o11y.ProxyFailureExpired,
			}).Inc()
			return nil, ErrExpiredUrl
		}

		ok := proxy.CheckSignedUrl(ctx, input.signature, input.expiry, input.url)
		if !ok {
			metrics.Failure.With(prometheus.Labels{
				o11y.ProxyFailureLabel: o11y.ProxyFailureSignature,
			}).Inc()
			return nil, ErrInvalidSignature
		}

		// Creating a new HTTP request to the target destination
		req, err := http.NewRequest("GET", input.url, nil)
		if err != nil {
			metrics.Failure.With(prometheus.Labels{
				o11y.ProxyFailureLabel: o11y.ProxyFailureRequest,
			}).Inc()
			return nil, ErrInvalidRequestUrl
		}

		// Send the HTTP request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			metrics.Failure.With(prometheus.Labels{
				o11y.ProxyFailureLabel: o11y.ProxyFailureDownload,
			}).Inc()
			return nil, ErrCantDownloadFile
		}

		pResp := proxyResponse{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			Body:       resp.Body,
		}

		return pResp, nil
	}
}
