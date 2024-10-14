package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const (
	wellKnownEndpoint = ".well-known/terraform.json"
	contentTypeKey    = "Content-Type"
	applicationJson   = "application/json"
	httpsScheme       = "https"
)

type ServiceDiscoveryResolver interface {
	Resolve(ctx context.Context, host string) (*DiscoveredRemoteService, error)
}

// DiscoveredRemoteService holds the information retrieved during the discovery process
type DiscoveredRemoteService struct {
	// URL holds the host name that returned the service discovery payload.
	// The URLs host can be different from the initial request host due to redirects.
	// It is the base URL for relative provider paths.
	URL url.URL
	WellKnownEndpointResponse
}

// discoveredRemoteServiceMap wraps sync.Map to prevent mixing up the types
type discoveredRemoteServiceMap struct {
	m sync.Map
}

func (d *discoveredRemoteServiceMap) Load(host string) (*DiscoveredRemoteService, bool) {
	v, ok := d.m.Load(host)
	if !ok {
		return nil, ok
	}

	discovered, ok := v.(DiscoveredRemoteService)
	if !ok {
		panic("failed to type assert to DiscoveredRemoteService")
	}

	return &discovered, true
}

func (d *discoveredRemoteServiceMap) Store(host string, ds DiscoveredRemoteService) {
	d.m.Store(host, ds)
}

type WellKnownEndpointResponse struct {
	ModulesV1   string `json:"modules.v1,omitempty"`
	ProvidersV1 string `json:"providers.v1,omitempty"`
}

// The RemoteServiceDiscovery struct caches HTTP path prefixes for providers which were discovered over the well-known service discovery mechanism.
// See: https://developer.hashicorp.com/terraform/internals/remote-service-discovery
type RemoteServiceDiscovery struct {
	// We use a sync.Map here as we only write the entry for a given key once but read it many times.
	// This data structure is optimized for this use case.
	// The sync.Map is wrapped to enforce storing/retrieving only DiscoveredRemoteService.
	m      discoveredRemoteServiceMap
	client *http.Client
}

// Resolve returns the relative provider path for a given URL
func (r *RemoteServiceDiscovery) Resolve(ctx context.Context, host string) (*DiscoveredRemoteService, error) {
	existing, ok := r.m.Load(host)
	if !ok {
		discovered, err := r.discover(ctx, host)
		if err != nil {
			return nil, err
		}

		r.m.Store(host, *discovered)
		return discovered, nil
	}

	return existing, nil
}

func (r *RemoteServiceDiscovery) discover(ctx context.Context, host string) (*DiscoveredRemoteService, error) {
	discovered, err := r.wellKnownEndpoint(ctx, host)
	if err != nil {
		return nil, err
	}

	// The remote service discovery protocol allows for absolute URLs to be returned.
	// We check whether it's an absolute URL and try to parse it, so that we can return both the path and the host
	if strings.HasPrefix(discovered.ProvidersV1, "https") {
		absoluteUrl, err := url.Parse(discovered.ProvidersV1)
		if err != nil {
			return nil, fmt.Errorf("failed to parse absolute url: %w", err)
		}
		discovered.ProvidersV1 = absoluteUrl.Path
		discovered.URL.Host = absoluteUrl.Host
	}

	return discovered, nil
}

// wellKnownEndpoint returns the response from the discovered upstream registry, as well as the final hostname which served the response.
// This is relevant in case the HTTP client was redirected to another URL.
func (r *RemoteServiceDiscovery) wellKnownEndpoint(ctx context.Context, host string) (*DiscoveredRemoteService, error) {
	u := url.URL{
		Scheme: httpsScheme,
		Host:   host,
		Path:   wellKnownEndpoint,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve well known endpoint for %s: %w", u.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to resolve well known endpoint for %s: status code is %d", u.String(), resp.StatusCode)
	}

	if value := resp.Header.Get(contentTypeKey); value != applicationJson {
		return nil, fmt.Errorf("reponse is of unsupported content-type %s", value)
	}

	decoder := json.NewDecoder(resp.Body)
	response := WellKnownEndpointResponse{}
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}

	return &DiscoveredRemoteService{
		URL: url.URL{
			Scheme: httpsScheme,
			Host:   resp.Request.Host,
		},
		WellKnownEndpointResponse: response,
	}, nil
}

func NewRemoteServiceDiscovery(client *http.Client) ServiceDiscoveryResolver {
	return &RemoteServiceDiscovery{
		client: client,
	}
}
