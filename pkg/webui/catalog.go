package webui

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"

	"github.com/hashicorp/go-version"
	"golang.org/x/sync/errgroup"
)

// ModuleEntry is a module with all of its versions.
type ModuleEntry struct {
	Namespace, Name, Provider string
	Versions                  []string // sorted descending
	Latest                    string
}

// ProviderVersionEntry is a single provider version with its platforms.
type ProviderVersionEntry struct {
	Version   string
	Platforms []string // "os_arch", sorted ascending
}

// ProviderEntry is a provider with all of its versions.
type ProviderEntry struct {
	Hostname  string // empty for internal providers
	Namespace string
	Name      string
	Versions  []ProviderVersionEntry // sorted descending by Version
	Latest    string
}

type catalog struct {
	Modules   []ModuleEntry
	Providers []ProviderEntry
	Mirrored  []ProviderEntry
}

func buildCatalog(modules []core.Module, providers, mirrored []*core.Provider) *catalog {
	return &catalog{
		Modules:   groupModules(modules),
		Providers: groupProviders(providers),
		Mirrored:  groupProviders(mirrored),
	}
}

func groupModules(modules []core.Module) []ModuleEntry {
	type key struct{ ns, name, provider string }
	versions := map[key]map[string]struct{}{}
	for _, m := range modules {
		k := key{m.Namespace, m.Name, m.Provider}
		if versions[k] == nil {
			versions[k] = map[string]struct{}{}
		}
		versions[k][m.Version] = struct{}{}
	}

	entries := make([]ModuleEntry, 0, len(versions))
	for k, vs := range versions {
		sorted := sortVersionsDesc(setToSlice(vs))
		entries = append(entries, ModuleEntry{
			Namespace: k.ns,
			Name:      k.name,
			Provider:  k.provider,
			Versions:  sorted,
			Latest:    latestVersion(sorted),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Provider < b.Provider
	})
	return entries
}

func groupProviders(providers []*core.Provider) []ProviderEntry {
	type key struct{ host, ns, name string }
	platforms := map[key]map[string]map[string]struct{}{}
	for _, p := range providers {
		k := key{p.Hostname, p.Namespace, p.Name}
		if platforms[k] == nil {
			platforms[k] = map[string]map[string]struct{}{}
		}
		if platforms[k][p.Version] == nil {
			platforms[k][p.Version] = map[string]struct{}{}
		}
		platforms[k][p.Version][p.OS+"_"+p.Arch] = struct{}{}
	}

	entries := make([]ProviderEntry, 0, len(platforms))
	for k, byVersion := range platforms {
		vs := make([]string, 0, len(byVersion))
		for v := range byVersion {
			vs = append(vs, v)
		}
		sorted := sortVersionsDesc(vs)
		versions := make([]ProviderVersionEntry, 0, len(sorted))
		for _, v := range sorted {
			ps := setToSlice(byVersion[v])
			sort.Strings(ps)
			versions = append(versions, ProviderVersionEntry{Version: v, Platforms: ps})
		}
		entries = append(entries, ProviderEntry{
			Hostname:  k.host,
			Namespace: k.ns,
			Name:      k.name,
			Versions:  versions,
			Latest:    latestVersion(sorted),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Hostname != b.Hostname {
			return a.Hostname < b.Hostname
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	return entries
}

func setToSlice(set map[string]struct{}) []string {
	s := make([]string, 0, len(set))
	for v := range set {
		s = append(s, v)
	}
	return s
}

// sortVersionsDesc sorts valid semantic versions descending, followed by unparseable versions descending lexicographically.
func sortVersionsDesc(vs []string) []string {
	type parsed struct {
		raw string
		v   *version.Version
	}
	items := make([]parsed, 0, len(vs))
	for _, raw := range vs {
		v, err := version.NewVersion(raw)
		if err != nil {
			v = nil
		}
		items = append(items, parsed{raw: raw, v: v})
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch {
		case a.v != nil && b.v != nil:
			return a.v.GreaterThan(b.v)
		case a.v != nil:
			return true
		case b.v != nil:
			return false
		default:
			return a.raw > b.raw
		}
	})

	sorted := make([]string, 0, len(items))
	for _, item := range items {
		sorted = append(sorted, item.raw)
	}
	return sorted
}

// latestVersion returns the highest stable version of a descending list, falling back to its first element.
func latestVersion(sortedDesc []string) string {
	if len(sortedDesc) == 0 {
		return ""
	}
	for _, raw := range sortedDesc {
		v, err := version.NewVersion(raw)
		if err == nil && v.Prerelease() == "" {
			return raw
		}
	}
	return sortedDesc[0]
}

type catalogLoader struct {
	storage   Storage
	ttl       time.Duration
	mu        sync.Mutex
	cached    *catalog
	fetchedAt time.Time
}

// get returns the cached catalog or loads it from the storage. Concurrent refreshes are coalesced by the mutex.
func (l *catalogLoader) get(ctx context.Context) (*catalog, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.ttl > 0 && l.cached != nil && time.Since(l.fetchedAt) < l.ttl {
		return l.cached, nil
	}

	var (
		modules            []core.Module
		providers, mirrors []*core.Provider
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		modules, err = l.storage.ListAllModules(gctx)
		return err
	})
	g.Go(func() (err error) {
		providers, err = l.storage.ListAllProviders(gctx)
		return err
	})
	g.Go(func() (err error) {
		mirrors, err = l.storage.ListAllMirroredProviders(gctx)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	l.cached = buildCatalog(modules, providers, mirrors)
	l.fetchedAt = time.Now()
	return l.cached, nil
}
