package mirror

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/TierMobility/boring-registry/pkg/discovery"
)

type mockedRemoteServiceDiscovery struct {
	resolve func(ctx context.Context, host string) (*discovery.DiscoveredRemoteService, error)
}

func (m *mockedRemoteServiceDiscovery) Resolve(ctx context.Context, host string) (*discovery.DiscoveredRemoteService, error) {
	return m.resolve(ctx, host)
}

func Test_upstreamProviderRegistry_listProviderVersions(t *testing.T) {
	tests := []struct {
		name                   string
		remoteServiceDiscovery discovery.ServiceDiscoveryResolver
		statusCode             int
		provider               *core.Provider
		want                   *core.ProviderVersions
		wantErr                bool
	}{
		{
			name: "successfully retrieve upstream responsei",
			remoteServiceDiscovery: &mockedRemoteServiceDiscovery{
				resolve: func(ctx context.Context, host string) (*discovery.DiscoveredRemoteService, error) {
					return &discovery.DiscoveredRemoteService{}, nil
				},
			},
			statusCode: http.StatusOK,
			provider: &core.Provider{
				Hostname:  "terraform.example.com",
				Namespace: "example",
				Name:      "random",
			},
			want: &core.ProviderVersions{
				Versions: []core.ProviderVersion{
					{
						Version:   "2.0.0",
						Protocols: []string{"4.0", "5.1"},
						Platforms: []core.Platform{
							{
								OS:   "linux",
								Arch: "amd64",
							},
							{
								OS:   "linux",
								Arch: "arm",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(tt.statusCode)
				if _, err := writer.Write([]byte(`{
  "versions": [
    {
      "version": "2.0.0",
      "protocols": ["4.0", "5.1"],
      "platforms": [
        {"os": "linux", "arch": "amd64"},
        {"os": "linux", "arch": "arm"}
      ]
    }
  ]
}`)); err != nil {
					panic(err)
				}
			}))
			mockedServiceDiscovery := &mockedRemoteServiceDiscovery{
				resolve: func(ctx context.Context, host string) (*discovery.DiscoveredRemoteService, error) {
					return &discovery.DiscoveredRemoteService{
						URL: url.URL{
							Scheme: "https",
							Host:   strings.TrimPrefix(server.URL, "https://"),
						},
						WellKnownEndpointResponse: discovery.WellKnownEndpointResponse{
							ProvidersV1: "/v1/providers",
						},
					}, nil
				},
			}
			u := &upstreamProviderRegistry{
				client:                 server.Client(),
				remoteServiceDiscovery: mockedServiceDiscovery,
			}
			got, err := u.listProviderVersions(context.Background(), tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("upstreamProviderRegistry.listProviderVersions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("upstreamProviderRegistry.listProviderVersions() = %v, want %v", got, tt.want)
			}
		})
	}
}
