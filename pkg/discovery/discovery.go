package discovery

// See https://developer.hashicorp.com/terraform/internals/remote-service-discovery
type Discovery struct {
	LoginV1     *LoginV1 `json:"login.v1,omitempty"`
	ModulesV1   string   `json:"modules.v1,omitempty"`
	ProvidersV1 string   `json:"providers.v1,omitempty"`
}

type LoginV1 struct {
	Client     string   `json:"client,omitempty"`
	GrantTypes []string `json:"grant_types,omitempty"`
	Authz      string   `json:"authz,omitempty"`
	Token      string   `json:"token,omitempty"`
	Ports      []int    `json:"ports,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
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
