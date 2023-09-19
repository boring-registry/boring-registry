package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type serverOptions func(response *WellKnownEndpointResponse, host string)

func withAbsoluteProviderURL() serverOptions {
	return func(r *WellKnownEndpointResponse, host string) {
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   r.ProvidersV1,
		}
		r.ProvidersV1 = u.String()
	}
}

func setupServer(statusCode int, response *WellKnownEndpointResponse, opts ...serverOptions) *httptest.Server {
	// We have to create our own listener, so that we can determine the listening address before starting the server
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		panic(err)
	}

	handler := http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add(contentTypeKey, "application/json")
		res.WriteHeader(statusCode)

		for _, option := range opts {
			option(response, listener.Addr().String())
		}

		var b []byte
		if response != nil {
			var err error
			b, err = json.Marshal(response)
			if err != nil {
				panic(fmt.Errorf("failed to marshal response body: %v", err))
			}
		}

		if _, err := res.Write(b); err != nil {
			panic(err)
		}
	})
	s := httptest.NewUnstartedServer(handler)
	s.Listener = listener
	s.StartTLS()
	return s
}

func Test_serviceDiscovery_resolve(t *testing.T) {
	tests := []struct {
		name    string
		server  *httptest.Server
		want    *DiscoveredRemoteService
		wantErr bool
	}{
		{
			name:   "returning a HTTP 404 status code",
			server: setupServer(http.StatusNotFound, nil),
			//response: nil,
			wantErr: true,
		},
		{
			name: "response with modules",
			server: setupServer(http.StatusOK, &WellKnownEndpointResponse{
				ModulesV1: "/v1/modules",
			}),
			want: &DiscoveredRemoteService{
				WellKnownEndpointResponse: WellKnownEndpointResponse{
					ModulesV1: "/v1/modules",
				},
			},
		},
		{
			name: "successfully adding cache entry with relative URL",
			server: setupServer(http.StatusOK, &WellKnownEndpointResponse{
				ProvidersV1: "/v1/providers",
			}),
			want: &DiscoveredRemoteService{
				WellKnownEndpointResponse: WellKnownEndpointResponse{
					ProvidersV1: "/v1/providers",
				},
			},
		},
		{
			name: "successfully adding cache entry with absolute URL",
			server: setupServer(http.StatusOK,
				&WellKnownEndpointResponse{
					ProvidersV1: "/v1/providers",
				},
				withAbsoluteProviderURL(),
			),
			want: &DiscoveredRemoteService{
				WellKnownEndpointResponse: WellKnownEndpointResponse{
					ProvidersV1: "/v1/providers",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewRemoteServiceDiscovery(tt.server.Client())
			u, err := url.Parse(tt.server.URL)
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.Resolve(context.Background(), u.Host)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.want != nil {
				tt.want.URL = url.URL{
					Scheme: "https",
					Host:   strings.TrimPrefix(tt.server.URL, "https://"),
				}
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
