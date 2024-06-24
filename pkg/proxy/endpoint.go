package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	"github.com/go-kit/kit/endpoint"
	"github.com/prometheus/client_golang/prometheus"
)

type proxyRequest struct {
	url string
}

type proxyResponse struct {
	StatusCode int
	Body       io.ReadCloser
	Header     http.Header
}

func proxyEndpoint(storage Storage, metrics *o11y.ProxyMetrics) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		input := request.(proxyRequest)

		metrics.Download.With(prometheus.Labels{}).Inc()

		slog.Info("Common baby!", slog.String("URL", input.url))

		downloadUrl, err := storage.GetDownloadUrl(ctx, input.url)
		if err != nil {
			metrics.Failure.With(prometheus.Labels{
				o11y.ProxyFailureLabel: o11y.ProxyFailureUrl,
			}).Inc()
			return nil, ErrInvalidRequestUrl
		}

		// Creating a new HTTP request to the target destination
		req, err := http.NewRequest("GET", downloadUrl, nil)
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

		headers := resp.Header.Clone()

		// Add Content-Disposition header if not there
		_, ok := headers["Content-Disposition"]
		if !ok {
			fileName, err := getFileNameFromURL(downloadUrl)
			if err == nil {
				headers.Add("Content-Disposition", `attachment;filename="`+fileName+`"`)
			}
		}

		slog.Info("respone!", slog.Any("headers", resp.Header))
		pResp := proxyResponse{
			StatusCode: resp.StatusCode,
			Header:     headers,
			Body:       resp.Body,
		}

		return pResp, nil
	}
}

// Extract zip filename from the path part of the URL, which should be located at the end of the path
func getFileNameFromURL(downloadUrl string) (string, error) {
	parsedUrl, err := url.Parse(downloadUrl)
	if err != nil {
		return "", fmt.Errorf("downloadUrl cannot be parsed: %s", downloadUrl)
	}

	lastIndex := strings.LastIndex(parsedUrl.Path, "/")
	return parsedUrl.Path[lastIndex+1:], nil
}
