package module

import (
	"context"
	"fmt"
	"time"

	"github.com/boring-registry/boring-registry/pkg/audit"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	"github.com/go-kit/kit/endpoint"
	"github.com/prometheus/client_golang/prometheus"
)

type listRequest struct {
	namespace string
	name      string
	provider  string
}

type listResponseVersion struct {
	Version string `json:"version,omitempty"`
}

type listResponseModule struct {
	Versions []listResponseVersion `json:"versions,omitempty"`
}

type listResponse struct {
	Modules []listResponseModule `json:"modules,omitempty"`
}

func listEndpoint(svc Service, metrics *o11y.ModuleMetrics, auditLogger audit.Logger) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		start := time.Now()
		req := request.(listRequest)

		metrics.ListVersions.With(prometheus.Labels{
			o11y.NamespaceLabel: req.namespace,
			o11y.NameLabel:      req.name,
			o11y.ProviderLabel:  req.provider,
		}).Inc()

		res, err := svc.ListModuleVersions(ctx, req.namespace, req.name, req.provider)
		if err != nil {
			return nil, err
		}

		resource := fmt.Sprintf("%s/%s/%s", req.namespace, req.name, req.provider)
		audit.LogRegistryAccess(ctx, auditLogger, "module", resource, audit.ActionList, time.Since(start))

		var versions []listResponseVersion

		for _, module := range res {
			versions = append(versions, listResponseVersion{
				Version: module.Version,
			})
		}

		return listResponse{
			Modules: []listResponseModule{
				{
					Versions: versions,
				},
			},
		}, nil
	}
}

type downloadRequest struct {
	namespace string
	name      string
	provider  string
	version   string
}

type downloadResponse struct{ url string }

func downloadEndpoint(svc Service, metrics *o11y.ModuleMetrics, auditLogger audit.Logger) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		start := time.Now()
		req := request.(downloadRequest)

		metrics.Download.With(prometheus.Labels{
			o11y.NamespaceLabel: req.namespace,
			o11y.NameLabel:      req.name,
			o11y.ProviderLabel:  req.provider,
			o11y.VersionLabel:   req.version,
		}).Inc()

		res, err := svc.GetModule(ctx, req.namespace, req.name, req.provider, req.version)
		if err != nil {
			return nil, err
		}

		resource := fmt.Sprintf("%s/%s/%s/%s", req.namespace, req.name, req.provider, req.version)
		audit.LogRegistryAccess(ctx, auditLogger, "module", resource, audit.ActionDownload, time.Since(start))

		return downloadResponse{
			url: res.DownloadURL,
		}, nil
	}
}
