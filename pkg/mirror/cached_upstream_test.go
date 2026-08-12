package mirror

import (
	"context"
	"testing"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"
	"github.com/prometheus/client_golang/prometheus"
)

// testMirrorMetrics builds unregistered collectors so that each test gets an independent set,
// as o11y.NewMetrics registers with the default registry and panics when called repeatedly.
func testMirrorMetrics() *o11y.MirrorMetrics {
	counter := func(name string, labels ...string) *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{Name: name}, labels)
	}

	providerLabels := []string{o11y.HostnameLabel, o11y.NamespaceLabel, o11y.NameLabel}
	versionLabels := append(append([]string{}, providerLabels...), o11y.VersionLabel)
	platformLabels := append(append([]string{}, versionLabels...), o11y.OsLabel, o11y.ArchLabel)

	return &o11y.MirrorMetrics{
		ListProviderVersions:         counter("list_provider_versions_total", providerLabels...),
		ListProviderInstallation:     counter("list_provider_installation_total", providerLabels...),
		RetrieveProviderArchive:      counter("retrieve_provider_archive_total", platformLabels...),
		ListProviderVersionsCacheHit: counter("list_provider_versions_cache_hit_total", providerLabels...),
		GetProviderCacheHit:          counter("get_provider_cache_hit_total", platformLabels...),
		GetShaSumsCacheHit:           counter("get_sha_sums_cache_hit_total", versionLabels...),
	}
}

func testCachedUpstream(t *testing.T, upstream upstreamProvider) *cachedUpstreamProvider {
	t.Helper()

	cached, err := newCachedUpstreamProvider(upstream, CacheConfig{
		Enabled:   true,
		TTL:       24 * time.Hour,
		MaxSizeMB: 16,
	}, testMirrorMetrics())
	if err != nil {
		t.Fatalf("failed to create cached upstream provider: %v", err)
	}

	return cached
}

// ageCachedEntry backdates a cache entry so that it is considered eligible for revalidation.
func ageCachedEntry(t *testing.T, cached *cachedUpstreamProvider, key string) {
	t.Helper()

	entry, ok := cached.cache.GetIfPresent(key)
	if !ok {
		t.Fatalf("expected key %q to be cached", key)
	}
	entry.storedAt = time.Now().Add(-revalidationCooldown - time.Second)
	cached.cache.Set(key, entry)
}

func providerVersions(versions ...string) *core.ProviderVersions {
	response := &core.ProviderVersions{}
	for _, version := range versions {
		response.Versions = append(response.Versions, core.ProviderVersion{
			Version:   version,
			Platforms: []core.Platform{{OS: "linux", Arch: "amd64"}},
		})
	}
	return response
}

func TestCachedUpstreamProviderListProviderVersions(t *testing.T) {
	t.Parallel()

	provider := &core.Provider{
		Hostname:  "registry.opentofu.org",
		Namespace: "grafana",
		Name:      "grafana",
	}

	t.Run("serves a cached list without calling upstream", func(t *testing.T) {
		t.Parallel()

		var calls int
		cached := testCachedUpstream(t, &mockedUpstreamProvider{
			customListProviderVersions: func(_ context.Context, _ *core.Provider) (*core.ProviderVersions, error) {
				calls++
				return providerVersions("4.45.0"), nil
			},
		})

		for range 3 {
			if _, err := cached.listProviderVersions(context.Background(), provider); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		if calls != 1 {
			t.Errorf("expected 1 upstream call, got %d", calls)
		}
	})

	// A version published after the list was cached must not be reported as missing, otherwise
	// the pull-through mirror falls back to storage and rejects the provider until the entry expires.
	t.Run("revalidates when the requested version is absent from the cached list", func(t *testing.T) {
		t.Parallel()

		var calls int
		cached := testCachedUpstream(t, &mockedUpstreamProvider{
			customListProviderVersions: func(_ context.Context, _ *core.Provider) (*core.ProviderVersions, error) {
				calls++
				if calls == 1 {
					return providerVersions("4.45.0"), nil
				}
				return providerVersions("4.45.0", "4.45.1"), nil
			},
		})

		if _, err := cached.listProviderVersions(context.Background(), provider); err != nil {
			t.Fatalf("unexpected error priming the cache: %v", err)
		}
		ageCachedEntry(t, cached, buildVersionsKey(provider))

		requested := provider.Clone()
		requested.Version = "4.45.1"
		versions, err := cached.listProviderVersions(context.Background(), requested)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calls != 2 {
			t.Errorf("expected upstream to be revalidated, got %d call(s)", calls)
		}
		if !versionExists("4.45.1", versions) {
			t.Error("expected the revalidated list to contain 4.45.1")
		}
	})

	t.Run("does not revalidate when the requested version is present", func(t *testing.T) {
		t.Parallel()

		var calls int
		cached := testCachedUpstream(t, &mockedUpstreamProvider{
			customListProviderVersions: func(_ context.Context, _ *core.Provider) (*core.ProviderVersions, error) {
				calls++
				return providerVersions("4.45.0", "4.45.1"), nil
			},
		})

		if _, err := cached.listProviderVersions(context.Background(), provider); err != nil {
			t.Fatalf("unexpected error priming the cache: %v", err)
		}

		requested := provider.Clone()
		requested.Version = "4.45.1"
		if _, err := cached.listProviderVersions(context.Background(), requested); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calls != 1 {
			t.Errorf("expected the cached list to be served, got %d upstream call(s)", calls)
		}
	})

	// Without the cooldown, requests for a version that genuinely doesn't exist would bypass
	// the cache on every call and defeat the purpose of caching upstream responses.
	t.Run("cooldown prevents repeated revalidation for an unknown version", func(t *testing.T) {
		t.Parallel()

		var calls int
		cached := testCachedUpstream(t, &mockedUpstreamProvider{
			customListProviderVersions: func(_ context.Context, _ *core.Provider) (*core.ProviderVersions, error) {
				calls++
				return providerVersions("4.45.0"), nil
			},
		})

		requested := provider.Clone()
		requested.Version = "99.0.0"
		for range 5 {
			if _, err := cached.listProviderVersions(context.Background(), requested); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		if calls != 1 {
			t.Errorf("expected the cooldown to suppress revalidation, got %d upstream call(s)", calls)
		}
	})

	// index.json requests carry no version, so they must never trigger revalidation.
	t.Run("does not revalidate when no version is requested", func(t *testing.T) {
		t.Parallel()

		var calls int
		cached := testCachedUpstream(t, &mockedUpstreamProvider{
			customListProviderVersions: func(_ context.Context, _ *core.Provider) (*core.ProviderVersions, error) {
				calls++
				return providerVersions("4.45.0"), nil
			},
		})

		if _, err := cached.listProviderVersions(context.Background(), provider); err != nil {
			t.Fatalf("unexpected error priming the cache: %v", err)
		}
		ageCachedEntry(t, cached, buildVersionsKey(provider))

		if _, err := cached.listProviderVersions(context.Background(), provider); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calls != 1 {
			t.Errorf("expected the cached list to be served, got %d upstream call(s)", calls)
		}
	})
}
