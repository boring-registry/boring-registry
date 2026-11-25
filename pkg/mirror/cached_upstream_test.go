package mirror

import (
	"context"
	"testing"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// Unregistered counters, so the test does not touch the global registry.
func newTestMirrorMetrics() *o11y.MirrorMetrics {
	labels := []string{o11y.HostnameLabel, o11y.NamespaceLabel, o11y.NameLabel}
	return &o11y.MirrorMetrics{
		ListProviderVersionsCacheHit:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "hit"}, labels),
		ListProviderVersionsCacheMiss: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "miss"}, labels),
	}
}

func counterValue(t *testing.T, vec *prometheus.CounterVec, provider *core.Provider) float64 {
	t.Helper()
	m := &dto.Metric{}
	err := vec.With(prometheus.Labels{
		o11y.HostnameLabel:  provider.Hostname,
		o11y.NamespaceLabel: provider.Namespace,
		o11y.NameLabel:      provider.Name,
	}).Write(m)
	if err != nil {
		t.Fatalf("reading counter: %v", err)
	}
	return m.GetCounter().GetValue()
}

func TestCachedListProviderVersionsCountsHitsAndMisses(t *testing.T) {
	upstreamCalls := 0
	upstream := &mockedUpstreamProvider{
		customListProviderVersions: func(ctx context.Context, provider *core.Provider) (*core.ProviderVersions, error) {
			upstreamCalls++
			return &core.ProviderVersions{Versions: []core.ProviderVersion{{Version: "1.0.0"}}}, nil
		},
	}
	metrics := newTestMirrorMetrics()
	cached, err := newCachedUpstreamProvider(upstream, CacheConfig{Enabled: true, TTL: time.Hour, MaxSizeMB: 1}, metrics)
	if err != nil {
		t.Fatalf("creating the cached upstream: %v", err)
	}

	provider := &core.Provider{Hostname: "registry.opentofu.org", Namespace: "hashicorp", Name: "random"}
	for i := 0; i < 3; i++ {
		if _, err := cached.listProviderVersions(context.Background(), provider); err != nil {
			t.Fatalf("lookup %d: %v", i, err)
		}
	}

	if upstreamCalls != 1 {
		t.Errorf("upstream calls = %d, want 1", upstreamCalls)
	}
	if got := counterValue(t, metrics.ListProviderVersionsCacheMiss, provider); got != 1 {
		t.Errorf("cache misses = %v, want 1", got)
	}
	if got := counterValue(t, metrics.ListProviderVersionsCacheHit, provider); got != 2 {
		t.Errorf("cache hits = %v, want 2", got)
	}
}
