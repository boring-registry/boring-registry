package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/boring-registry/boring-registry/pkg/audit"
	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	"github.com/go-kit/kit/endpoint"
	"github.com/prometheus/client_golang/prometheus"
)

type listRequest struct {
	namespace string
	name      string
}

func listEndpoint(svc Service, metrics *o11y.ProviderMetrics, auditLogger audit.Logger) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		start := time.Now()
		req := request.(listRequest)

		metrics.ListVersions.With(prometheus.Labels{
			o11y.NamespaceLabel: req.namespace,
			o11y.NameLabel:      req.name,
		}).Inc()

		result, err := svc.ListProviderVersions(ctx, req.namespace, req.name)
		if err != nil {
			return nil, err
		}

		resource := fmt.Sprintf("%s/%s", req.namespace, req.name)
		audit.LogRegistryAccess(ctx, auditLogger, "provider", resource, audit.ActionList, time.Since(start))

		return result, nil
	}
}

type downloadRequest struct {
	namespace string
	name      string
	version   string
	os        string
	arch      string
}

type downloadResponse struct {
	OS                  string           `json:"os"`
	Arch                string           `json:"arch"`
	Filename            string           `json:"filename"`
	DownloadURL         string           `json:"download_url"`
	Shasum              string           `json:"shasum"`
	ShasumsURL          string           `json:"shasums_url"`
	ShasumsSignatureURL string           `json:"shasums_signature_url"`
	SigningKeys         core.SigningKeys `json:"signing_keys"`
}

func downloadEndpoint(svc Service, metrics *o11y.ProviderMetrics, auditLogger audit.Logger) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		start := time.Now()
		req := request.(downloadRequest)

		metrics.Download.With(prometheus.Labels{
			o11y.NamespaceLabel: req.namespace,
			o11y.NameLabel:      req.name,
			o11y.VersionLabel:   req.version,
			o11y.OsLabel:        req.os,
			o11y.ArchLabel:      req.arch,
		}).Inc()

		res, err := svc.GetProvider(ctx, req.namespace, req.name, req.version, req.os, req.arch)
		if err != nil {
			return nil, err
		}

		resource := fmt.Sprintf("%s/%s/%s/%s/%s", req.namespace, req.name, req.version, req.os, req.arch)
		audit.LogRegistryAccess(ctx, auditLogger, "provider", resource, audit.ActionDownload, time.Since(start))

		return downloadResponse{
			OS:                  res.OS,
			Arch:                res.Arch,
			DownloadURL:         res.DownloadURL,
			Filename:            res.Filename,
			Shasum:              res.Shasum,
			SigningKeys:         res.SigningKeys,
			ShasumsURL:          res.SHASumsURL,
			ShasumsSignatureURL: res.SHASumsSignatureURL,
		}, nil
	}
}
