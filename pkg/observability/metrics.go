package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ServerMetrics struct {
	Mirrors   *MirrorMetrics
	Modules   *ModulesMetrics
	Providers *ProvidersMetrics
	Http      *HttpMetrics
}
type MirrorMetrics struct {
	ListProviderVersions     *prometheus.CounterVec
	ListProviderInstallation *prometheus.CounterVec
	RetrieveProviderArchive  *prometheus.CounterVec
}
type ModulesMetrics struct {
	ListVersions *prometheus.CounterVec
	Download     *prometheus.CounterVec
}
type ProvidersMetrics struct {
	ListVersions *prometheus.CounterVec
	Download     *prometheus.CounterVec
}
type HttpMetrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestSize     *prometheus.SummaryVec
	ResponseSize    *prometheus.SummaryVec
}

func NewMetrics(buckets []float64) *ServerMetrics {
	boring_namespace := "boring_registry"
	http_namespace := "http"

	mirrors_subsystem := "mirrors"
	providers_subsystem := "providers"
	modules_subsystem := "modules"
	request_subsystem := "request"
	response_subsystem := "response"

	if buckets == nil {
		buckets = prometheus.ExponentialBuckets(0.1, 1.5, 5)
	}

	metrics := &ServerMetrics{
		Mirrors: &MirrorMetrics{
			ListProviderVersions: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boring_namespace,
					Subsystem: mirrors_subsystem,
					Name:      "list_provider_versions",
					Help:      "Number providers versions listing on the mirror.",
				},
				[]string{"hostname", "namespace", "name"},
			),
			ListProviderInstallation: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boring_namespace,
					Subsystem: mirrors_subsystem,
					Name:      "list_provider_installations",
					Help:      "Number providers installations listing on the mirror.",
				},
				[]string{"hostname", "namespace", "name", "version"},
			),
			RetrieveProviderArchive: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boring_namespace,
					Subsystem: mirrors_subsystem,
					Name:      "download_version",
					Help:      "Number providers retreive and archive on the mirror.",
				},
				[]string{"hostname", "namespace", "name", "version", "os", "arch"},
			),
		},
		Providers: &ProvidersMetrics{
			ListVersions: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boring_namespace,
					Subsystem: providers_subsystem,
					Name:      "list_versions",
					Help:      "Number of boring's providers versions listing.",
				},
				[]string{"namespace", "name"},
			),
			Download: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boring_namespace,
					Subsystem: providers_subsystem,
					Name:      "download_version",
					Help:      "Number of boring's providers download.",
				},
				[]string{"namespace", "name", "version", "os", "arch"},
			),
		},
		Modules: &ModulesMetrics{
			ListVersions: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boring_namespace,
					Subsystem: modules_subsystem,
					Name:      "list_versions",
					Help:      "Number of boring's modules versions listing.",
				},
				[]string{"namespace", "name", "provider"},
			),
			Download: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boring_namespace,
					Subsystem: modules_subsystem,
					Name:      "download_version",
					Help:      "Number of boring's modules download.",
				},
				[]string{"namespace", "name", "provider", "version"},
			),
		},
		Http: &HttpMetrics{
			RequestsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: http_namespace,
					Subsystem: request_subsystem,
					Name:      "total",
					Help:      "Tracks the number of HTTP requests.",
				}, []string{"method", "code"},
			),
			RequestDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: http_namespace,
					Subsystem: request_subsystem,
					Name:      "duration_seconds",
					Help:      "Tracks the latencies for HTTP requests.",
					Buckets:   buckets,
				},
				[]string{"method", "code"},
			),
			RequestSize: promauto.NewSummaryVec(
				prometheus.SummaryOpts{
					Namespace: http_namespace,
					Subsystem: request_subsystem,
					Name:      "size_bytes",
					Help:      "Tracks the size of HTTP requests.",
				},
				[]string{"method", "code"},
			),
			ResponseSize: promauto.NewSummaryVec(
				prometheus.SummaryOpts{
					Namespace: http_namespace,
					Subsystem: response_subsystem,
					Name:      "size_bytes",
					Help:      "Tracks the size of HTTP responses.",
				},
				[]string{"method", "code"},
			),
		},
	}

	return metrics
}
