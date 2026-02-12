package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	HostnameLabel     = "hostname"
	NamespaceLabel    = "namespace"
	NameLabel         = "name"
	ProviderLabel     = "provider"
	VersionLabel      = "version"
	OsLabel           = "os"
	ArchLabel         = "arch"
	ProxyFailureLabel = "failure"

	ProxyFailureUrl      = "bad-url"
	ProxyFailureRequest  = "invalid-request"
	ProxyFailureDownload = "download"
)

type ServerMetrics struct {
	Mirror   *MirrorMetrics
	Module   *ModuleMetrics
	Provider *ProviderMetrics
	Proxy    *ProxyMetrics
	Http     *HttpMetrics
}
type MirrorMetrics struct {
	ListProviderVersions         *prometheus.CounterVec
	ListProviderInstallation     *prometheus.CounterVec
	RetrieveProviderArchive      *prometheus.CounterVec
	ListProviderVersionsCacheHit *prometheus.CounterVec
	GetProviderCacheHit          *prometheus.CounterVec
	GetShaSumsCacheHit           *prometheus.CounterVec
}
type ModuleMetrics struct {
	ListVersions *prometheus.CounterVec
	Download     *prometheus.CounterVec
}
type ProviderMetrics struct {
	ListVersions *prometheus.CounterVec
	Download     *prometheus.CounterVec
}
type ProxyMetrics struct {
	Download *prometheus.CounterVec
	Failure  *prometheus.CounterVec
}
type HttpMetrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestSize     *prometheus.SummaryVec
	ResponseSize    *prometheus.SummaryVec
}

func NewMetrics(buckets []float64) *ServerMetrics {
	boringNamespace := "boring_registry"
	httpNamespace := "http"

	mirrorsSubsystem := "mirrors"
	providersSubsystem := "providers"
	proxySubsystem := "proxy"
	modulesSubsystem := "modules"
	requestSubsystem := "request"
	responseSubsystem := "response"

	if buckets == nil {
		buckets = prometheus.ExponentialBuckets(0.05, 1.6, 10)
	}

	metrics := &ServerMetrics{
		Mirror: &MirrorMetrics{
			ListProviderVersions: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: mirrorsSubsystem,
					Name:      "list_provider_versions_total",
					Help:      "The total number of provider versions requests by mirror",
				},
				[]string{HostnameLabel, NamespaceLabel, NameLabel},
			),
			ListProviderVersionsCacheHit: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: mirrorsSubsystem,
					Name:      "list_provider_versions_cache_hit_total",
					Help:      "The total number of cache hit for provider versions requests by mirror",
				},
				[]string{HostnameLabel, NamespaceLabel, NameLabel},
			),
			ListProviderInstallation: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: mirrorsSubsystem,
					Name:      "list_provider_installations_total",
					Help:      "The total number of provider installations requests by mirror",
				},
				[]string{HostnameLabel, NamespaceLabel, NameLabel, VersionLabel},
			),
			RetrieveProviderArchive: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: mirrorsSubsystem,
					Name:      "download_version_total",
					Help:      "The total number of provider retreive requests by mirror",
				},
				[]string{HostnameLabel, NamespaceLabel, NameLabel, VersionLabel, OsLabel, ArchLabel},
			),
			GetProviderCacheHit: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: mirrorsSubsystem,
					Name:      "download_version_cache_hit_total",
					Help:      "The total number of cache hit for provider retreive requests by mirror",
				},
				[]string{HostnameLabel, NamespaceLabel, NameLabel, VersionLabel, OsLabel, ArchLabel},
			),
			GetShaSumsCacheHit: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: mirrorsSubsystem,
					Name:      "download_sha_sums_cache_hit_total",
					Help:      "The total number of cache hit for provider version's SHA sums file",
				},
				[]string{HostnameLabel, NamespaceLabel, NameLabel, VersionLabel},
			),
		},
		Provider: &ProviderMetrics{
			ListVersions: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: providersSubsystem,
					Name:      "list_versions_total",
					Help:      "The total number of provider versions requests",
				},
				[]string{NamespaceLabel, NameLabel},
			),
			Download: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: providersSubsystem,
					Name:      "download_version_total",
					Help:      "The total number of provider download requests",
				},
				[]string{NamespaceLabel, NameLabel, VersionLabel, OsLabel, ArchLabel},
			),
		},
		Module: &ModuleMetrics{
			ListVersions: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: modulesSubsystem,
					Name:      "list_versions_total",
					Help:      "The total number of module versions requests",
				},
				[]string{NamespaceLabel, NameLabel, ProviderLabel},
			),
			Download: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: modulesSubsystem,
					Name:      "download_version_total",
					Help:      "The total number of module download requests",
				},
				[]string{NamespaceLabel, NameLabel, ProviderLabel, VersionLabel},
			),
		},
		Proxy: &ProxyMetrics{
			Download: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: proxySubsystem,
					Name:      "download_total",
					Help:      "The total number of download requests",
				},
				[]string{},
			),
			Failure: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: boringNamespace,
					Subsystem: proxySubsystem,
					Name:      "download_failure_total",
					Help:      "The total number of download failures",
				},
				[]string{ProxyFailureLabel},
			),
		},
		Http: &HttpMetrics{
			RequestsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: httpNamespace,
					Subsystem: requestSubsystem,
					Name:      "total",
					Help:      "The total number of HTTP requests",
				}, []string{"method", "code"},
			),
			RequestDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: httpNamespace,
					Subsystem: requestSubsystem,
					Name:      "duration_seconds",
					Help:      "The HTTP request latencies in seconds",
					Buckets:   buckets,
				},
				[]string{"method", "code"},
			),
			RequestSize: promauto.NewSummaryVec(
				prometheus.SummaryOpts{
					Namespace: httpNamespace,
					Subsystem: requestSubsystem,
					Name:      "size_bytes",
					Help:      "The HTTP request sizes in bytes",
				},
				[]string{"method", "code"},
			),
			ResponseSize: promauto.NewSummaryVec(
				prometheus.SummaryOpts{
					Namespace: httpNamespace,
					Subsystem: responseSubsystem,
					Name:      "size_bytes",
					Help:      "The HTTP reponse sizes in bytes",
				},
				[]string{"method", "code"},
			),
		},
	}

	return metrics
}
