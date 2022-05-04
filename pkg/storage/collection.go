package storage

import (
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
)

type Collection struct {
	m map[string]core.ProviderVersion
}

func NewCollection() *Collection {
	return &Collection{
		m: make(map[string]core.ProviderVersion),
	}
}

func (s *Collection) List() []core.ProviderVersion {
	var out []core.ProviderVersion

	for _, provider := range s.m {
		out = append(out, provider)
	}

	return out
}

func (s *Collection) Add(provider core.Provider) {
	id := fmt.Sprintf("%s/%s/%s", provider.Namespace, provider.Name, provider.Version)

	if _, ok := s.m[id]; !ok {
		s.m[id] = core.ProviderVersion{
			Namespace: provider.Namespace,
			Name:      provider.Name,
			Version:   provider.Version,
		}
	}

	ver := s.m[id]
	ver.Platforms = append(ver.Platforms, core.Platform{OS: provider.OS, Arch: provider.Arch})
	s.m[id] = ver
}
