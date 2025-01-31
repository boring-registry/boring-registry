package discovery

import (
	"errors"
	"fmt"
)

// See:
// https://opentofu.org/docs/internals/remote-service-discovery/
// https://developer.hashicorp.com/terraform/internals/remote-service-discovery
type Discovery struct {
	LoginV1     *LoginV1 `json:"login.v1,omitempty"`
	ModulesV1   string   `json:"modules.v1,omitempty"`
	ProvidersV1 string   `json:"providers.v1,omitempty"`
}

// See: https://opentofu.org/docs/internals/login-protocol/
type LoginV1 struct {
	Client     string   `json:"client,omitempty"`
	GrantTypes []string `json:"grant_types,omitempty"`
	Authz      string   `json:"authz,omitempty"`
	Token      string   `json:"token,omitempty"`
	Ports      []int    `json:"ports,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
}

func (l *LoginV1) Validate() error {
	var err error

	if l.Client == "" {
		err = errors.Join(err, fmt.Errorf("client: client identifier value is required but not configured"))
	}

	if l.Authz == "" {
		err = errors.Join(err, fmt.Errorf("authz: is required but not configured"))
	}

	if l.Token == "" {
		err = errors.Join(err, fmt.Errorf("token: is required but not configured"))
	}

	if len(l.Ports) != 0 && len(l.Ports) != 2 {
		err = errors.Join(err, fmt.Errorf("ports: is expected to be a two-element array, but has %d elements instead", len(l.Ports)))
	} else if len(l.Ports) == 2 {
		if l.Ports[0] > l.Ports[1] {
			err = errors.Join(err, fmt.Errorf("ports: the first array element is larger than the second"))
		} else if l.Ports[0] < 0 || 65535 < l.Ports[0] {
			err = errors.Join(err, fmt.Errorf("ports: the first array element is outside the allowed port range of [0-65535]"))
		} else if l.Ports[1] < 0 || 65535 < l.Ports[1] {
			err = errors.Join(err, fmt.Errorf("ports: the second array element is outside the allowed port range of [0-65535]"))
		}
	}

	for _, scope := range l.Scopes {
		if scope == "" {
			err = errors.Join(err, fmt.Errorf("scopes: an array element is empty"))
		}
	}

	return err
}

type Option func(*Discovery)

func WithModulesV1(v string) Option {
	return func(d *Discovery) {
		if v != "" {
			d.ModulesV1 = v
		}
	}
}

func WithProvidersV1(v string) Option {
	return func(d *Discovery) {
		if v != "" {
			d.ProvidersV1 = v
		}
	}
}

func WithLoginV1(login *LoginV1) Option {
	return func(d *Discovery) {
		if login != nil {
			d.LoginV1 = login
		}
	}
}

func New(options ...Option) *Discovery {
	discovery := &Discovery{}

	for _, option := range options {
		option(discovery)
	}

	return discovery
}
