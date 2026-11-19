package mirror

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"
	"github.com/maypok86/otter/v2"
	"github.com/prometheus/client_golang/prometheus"
)

// Contains the cache configuration
type CacheConfig struct {
	Enabled   bool
	TTL       time.Duration
	MaxSizeMB int
}

// revalidationCooldown bounds how often a cached version list is revalidated against
// upstream when a requested version is absent from it. Without it, repeated requests for
// a version that genuinely doesn't exist would bypass the cache on every call.
const revalidationCooldown = 30 * time.Second

// Represents a cache entry
type cacheEntry struct {
	data      any
	sizeBytes int
	storedAt  time.Time
}

// Wraps an upstreamProvider with caching
type cachedUpstreamProvider struct {
	upstream upstreamProvider
	cache    *otter.Cache[string, *cacheEntry]
	config   CacheConfig
	metrics  *o11y.MirrorMetrics
}

// Builds a cache key for listProviderVersions
func buildVersionsKey(provider *core.Provider) string {
	return fmt.Sprintf("versions:%s/%s/%s", provider.Hostname, provider.Namespace, provider.Name)
}

// Builds a cache key for getProvider
func buildProviderKey(provider *core.Provider) string {
	return fmt.Sprintf("provider:%s/%s/%s/%s/%s/%s",
		provider.Hostname, provider.Namespace, provider.Name,
		provider.Version, provider.OS, provider.Arch)
}

// Builds a cache key for shaSums
// Note: SHA256SUMS are the same for all platforms of a given version
func buildShaSumsKey(provider *core.Provider) string {
	return fmt.Sprintf("shasums:%s/%s/%s/%s",
		provider.Hostname, provider.Namespace, provider.Name, provider.Version)
}

// Estimates the size in bytes of an object via JSON marshaling (usefull for cache Weigther func)
func estimateSize(data any) (int, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	return len(bytes), nil
}

// isStaleFor reports whether a cached version list has to be revalidated against upstream.
// A cached list is only authoritative for the versions it contains: when a specific version is
// requested and is absent, the list may simply predate that version's publication upstream.
// Treating that as a definitive answer makes the pull-through mirror reject a provider until the
// entry expires, so the list is revalidated instead, rate-limited by revalidationCooldown.
func (c *cachedUpstreamProvider) isStaleFor(provider *core.Provider, versions *core.ProviderVersions, entry *cacheEntry) bool {
	if provider.Version == "" || versionExists(provider.Version, versions) {
		return false
	}

	return time.Since(entry.storedAt) >= revalidationCooldown
}

// Implements upstreamProvider's listProviderVersions method, with caching
func (c *cachedUpstreamProvider) listProviderVersions(ctx context.Context, provider *core.Provider) (*core.ProviderVersions, error) {
	key := buildVersionsKey(provider)

	// Try to get from cache
	if entry, ok := c.cache.GetIfPresent(key); ok {
		if versions, ok := entry.data.(*core.ProviderVersions); ok {
			if !c.isStaleFor(provider, versions, entry) {
				c.metrics.ListProviderVersionsCacheHit.With(prometheus.Labels{
					o11y.HostnameLabel:  provider.Hostname,
					o11y.NamespaceLabel: provider.Namespace,
					o11y.NameLabel:      provider.Name,
				}).Inc()
				return versions, nil
			}

			slog.Debug("cached version list does not contain the requested version, revalidating against upstream",
				slog.String("hostname", provider.Hostname),
				slog.String("namespace", provider.Namespace),
				slog.String("name", provider.Name),
				slog.String("version", provider.Version))
		}
	}

	// Cache miss - call upstream
	versions, err := c.upstream.listProviderVersions(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Estimate cache entry size
	sizeBytes, err := estimateSize(versions)
	if err != nil {
		slog.Warn("failed to estimate the byte size of provider's versions list. Cache set will be skipped", slog.String("error", err.Error()))
		return versions, nil
	}

	// Store in cache
	entry := &cacheEntry{
		data:      versions,
		sizeBytes: sizeBytes,
		storedAt:  time.Now(),
	}
	c.cache.Set(key, entry)

	return versions, nil
}

// Implements upstreamProvider's getProvider method, with caching
func (c *cachedUpstreamProvider) getProvider(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
	key := buildProviderKey(provider)

	// Try to get from cache
	if entry, ok := c.cache.GetIfPresent(key); ok {
		if prov, ok := entry.data.(*core.Provider); ok {
			c.metrics.GetProviderCacheHit.With(prometheus.Labels{
				o11y.HostnameLabel:  provider.Hostname,
				o11y.NamespaceLabel: provider.Namespace,
				o11y.NameLabel:      provider.Name,
				o11y.VersionLabel:   provider.Version,
				o11y.OsLabel:        provider.OS,
				o11y.ArchLabel:      provider.Arch,
			}).Inc()
			return prov, nil
		}
	}

	// Cache miss - call upstream
	prov, err := c.upstream.getProvider(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Estimate cache entry size
	sizeBytes, err := estimateSize(prov)
	if err != nil {
		slog.Warn("failed to estimate the byte size of provider's list. Cache set will be skipped", slog.String("error", err.Error()))
		return prov, nil
	}

	// Store in cache
	entry := &cacheEntry{
		data:      prov,
		sizeBytes: sizeBytes,
		storedAt:  time.Now(),
	}
	c.cache.Set(key, entry)

	return prov, nil
}

// Implements upstreamProvider's shaSums method, with caching
func (c *cachedUpstreamProvider) shaSums(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error) {
	key := buildShaSumsKey(provider)

	// Try to get from cache
	if entry, ok := c.cache.GetIfPresent(key); ok {
		if sums, ok := entry.data.(*core.Sha256Sums); ok {
			c.metrics.GetShaSumsCacheHit.With(prometheus.Labels{
				o11y.HostnameLabel:  provider.Hostname,
				o11y.NamespaceLabel: provider.Namespace,
				o11y.NameLabel:      provider.Name,
				o11y.VersionLabel:   provider.Version,
			}).Inc()
			return sums, nil
		}
	}

	// Cache miss - call upstream
	sums, err := c.upstream.shaSums(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Estimate cache entry size
	sizeBytes, err := estimateSize(sums)
	if err != nil {
		slog.Warn("failed to estimate the byte size of provider's SHA sums. Cache set will be skipped", slog.String("error", err.Error()))
		return sums, nil
	}

	// Store in cache
	entry := &cacheEntry{
		data:      sums,
		sizeBytes: sizeBytes,
		storedAt:  time.Now(),
	}
	c.cache.Set(key, entry)

	return sums, nil
}

// Creates a new upstream provider wrapper with caching
func newCachedUpstreamProvider(upstream upstreamProvider, config CacheConfig, metrics *o11y.MirrorMetrics) (*cachedUpstreamProvider, error) {
	// Convert MB to bytes
	maxWeightBytes := config.MaxSizeMB * 1024 * 1024

	// Configure otter cache
	opts := &otter.Options[string, *cacheEntry]{
		MaximumWeight: uint64(maxWeightBytes),
		Weigher: func(key string, value *cacheEntry) uint32 {
			// Check for uint32 overflow before doing the addition
			if value.sizeBytes > math.MaxUint32-len(key) {
				return math.MaxUint32
			}
			// Weight = size of key + size of value
			return uint32(len(key) + value.sizeBytes)
		},
		ExpiryCalculator: otter.ExpiryWriting[string, *cacheEntry](config.TTL),
		InitialCapacity:  100,
	}

	cache, err := otter.New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &cachedUpstreamProvider{
		upstream: upstream,
		cache:    cache,
		config:   config,
		metrics:  metrics,
	}, nil
}
