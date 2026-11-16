package webui

import (
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"

	"github.com/stretchr/testify/assert"
)

func TestSortVersionsDesc(t *testing.T) {
	t.Parallel()

	got := sortVersionsDesc([]string{"1.2.0", "1.10.0", "2.0.0-rc1", "bogus", "1.9.9"})
	assert.Equal(t, []string{"2.0.0-rc1", "1.10.0", "1.9.9", "1.2.0", "bogus"}, got)
	assert.Equal(t, "1.10.0", latestVersion(got))
}

func TestLatestVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", latestVersion(nil))
	assert.Equal(t, "2.0.0-rc2", latestVersion(sortVersionsDesc([]string{"2.0.0-rc1", "2.0.0-rc2", "1.0.0-beta"})))
	assert.Equal(t, "zeta", latestVersion(sortVersionsDesc([]string{"alpha", "zeta"})))
}

func TestBuildCatalog(t *testing.T) {
	t.Parallel()

	providers := []*core.Provider{
		{Namespace: "acme", Name: "dummy", Version: "0.1.0", OS: "linux", Arch: "amd64"},
		{Namespace: "acme", Name: "dummy", Version: "0.1.0", OS: "darwin", Arch: "arm64"},
		{Namespace: "acme", Name: "dummy", Version: "0.1.0", OS: "linux", Arch: "amd64"},
		{Namespace: "acme", Name: "dummy", Version: "0.2.0", OS: "linux", Arch: "amd64"},
		{Namespace: "acme", Name: "alpha", Version: "1.0.0", OS: "linux", Arch: "amd64"},
	}
	mirrored := []*core.Provider{
		{Hostname: "registry.terraform.io", Namespace: "hashicorp", Name: "random", Version: "3.6.0", OS: "linux", Arch: "amd64"},
	}
	modules := []core.Module{
		{Namespace: "acme", Name: "vpc", Provider: "aws", Version: "1.2.0"},
		{Namespace: "acme", Name: "vpc", Provider: "aws", Version: "1.3.0"},
		{Namespace: "acme", Name: "eks", Provider: "aws", Version: "0.1.0"},
	}

	c := buildCatalog(modules, providers, mirrored)

	assert.Equal(t, []ModuleEntry{
		{Namespace: "acme", Name: "eks", Provider: "aws", Versions: []string{"0.1.0"}, Latest: "0.1.0"},
		{Namespace: "acme", Name: "vpc", Provider: "aws", Versions: []string{"1.3.0", "1.2.0"}, Latest: "1.3.0"},
	}, c.Modules)

	assert.Equal(t, []ProviderEntry{
		{Namespace: "acme", Name: "alpha", Latest: "1.0.0", Versions: []ProviderVersionEntry{
			{Version: "1.0.0", Platforms: []string{"linux_amd64"}},
		}},
		{Namespace: "acme", Name: "dummy", Latest: "0.2.0", Versions: []ProviderVersionEntry{
			{Version: "0.2.0", Platforms: []string{"linux_amd64"}},
			{Version: "0.1.0", Platforms: []string{"darwin_arm64", "linux_amd64"}},
		}},
	}, c.Providers)

	assert.Equal(t, []ProviderEntry{
		{Hostname: "registry.terraform.io", Namespace: "hashicorp", Name: "random", Latest: "3.6.0", Versions: []ProviderVersionEntry{
			{Version: "3.6.0", Platforms: []string{"linux_amd64"}},
		}},
	}, c.Mirrored)
}
