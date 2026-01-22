package mirror

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/maypok86/otter/v2"
)

// Contains the cache configuration
type CacheConfig struct {
	Enabled   bool
	TTL       time.Duration
	MaxSizeMB int
}

// Represents a cache entry
type cacheEntry struct {
	data      interface{}
	timestamp time.Time
	sizeBytes int
}

// Wraps an upstreamProvider with caching
type cachedUpstreamProvider struct {
	upstream upstreamProvider
	cache    *otter.Cache[string, *cacheEntry]
	config   CacheConfig
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
func estimateSize(data interface{}) int {
	bytes, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	return len(bytes)
}

// Implements upstreamProvider's listProviderVersions method, with caching
func (c *cachedUpstreamProvider) listProviderVersions(ctx context.Context, provider *core.Provider) (*core.ProviderVersions, error) {
	key := buildVersionsKey(provider)

	// Try to get from cache
	if entry, ok := c.cache.GetIfPresent(key); ok {
		if versions, ok := entry.data.(*core.ProviderVersions); ok {
			return versions, nil
		}
	}

	// Cache miss - call upstream
	versions, err := c.upstream.listProviderVersions(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Store in cache
	entry := &cacheEntry{
		data:      versions,
		timestamp: time.Now(),
		sizeBytes: estimateSize(versions),
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
			return prov, nil
		}
	}

	// Cache miss - call upstream
	prov, err := c.upstream.getProvider(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Store in cache
	entry := &cacheEntry{
		data:      prov,
		timestamp: time.Now(),
		sizeBytes: estimateSize(prov),
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
			return sums, nil
		}
	}

	// Cache miss - call upstream
	sums, err := c.upstream.shaSums(ctx, provider)
	if err != nil {
		return nil, err
	}

	// Store in cache
	entry := &cacheEntry{
		data:      sums,
		timestamp: time.Now(),
		sizeBytes: estimateSize(sums),
	}
	c.cache.Set(key, entry)

	return sums, nil
}

// Creates a new upstream provider wrapper with caching
func newCachedUpstreamProvider(upstream upstreamProvider, config CacheConfig) (*cachedUpstreamProvider, error) {
	// Convert MB to bytes
	maxWeightBytes := config.MaxSizeMB * 1024 * 1024

	// Configure otter cache
	opts := &otter.Options[string, *cacheEntry]{
		MaximumWeight: uint64(maxWeightBytes),
		Weigher: func(key string, value *cacheEntry) uint32 {
			// Weight = size of key + size of value
			return uint32(len(key) + value.sizeBytes)
		},
		ExpiryCalculator: otter.ExpiryWriting[string, *cacheEntry](config.TTL),
		InitialCapacity:  1000,
	}

	cache, err := otter.New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &cachedUpstreamProvider{
		upstream: upstream,
		cache:    cache,
		config:   config,
	}, nil
}
