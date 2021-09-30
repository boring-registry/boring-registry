package provider

import (
	"fmt"
)

type Collection struct {
	m map[string]ProviderVersion
}

func NewCollection() *Collection {
	return &Collection{
		m: make(map[string]ProviderVersion),
	}
}

func (s *Collection) List() []ProviderVersion {
	var out []ProviderVersion

	for _, provider := range s.m {
		out = append(out, provider)
	}

	return out
}

func (s *Collection) Add(provider Provider) {
	id := fmt.Sprintf("%s/%s/%s", provider.Namespace, provider.Name, provider.Version)

	if _, ok := s.m[id]; !ok {
		s.m[id] = ProviderVersion{
			Namespace: provider.Namespace,
			Name:      provider.Name,
			Version:   provider.Version,
		}
	}

	ver := s.m[id]
	ver.Platforms = append(ver.Platforms, Platform{OS: provider.OS, Arch: provider.Arch})
	s.m[id] = ver
}
