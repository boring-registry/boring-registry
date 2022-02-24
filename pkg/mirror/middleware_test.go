package mirror

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/TierMobility/boring-registry/pkg/storage"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockedService struct {
	versionsResponse *ProviderVersions
	versionsError    error
}

func (m *mockedService) ListProviderVersions(_ context.Context, _ core.Provider) (*ProviderVersions, error) {
	return m.versionsResponse, m.versionsError
}

func (m *mockedService) ListProviderInstallation(ctx context.Context, provider core.Provider) (*Archives, error) {
	panic("implement me")
}

func (m *mockedService) RetrieveProviderArchive(ctx context.Context, provider core.Provider) (io.Reader, error) {
	panic("implement me")
}

func (m *mockedService) MirrorProvider(ctx context.Context, provider core.Provider, binary, shasum, shasumSignature io.Reader) error {
	panic("implement me")
}

func TestProxyRegistry_ListProviderVersions(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testCases := []struct {
		name               string
		provider           core.Provider
		upstreamStatusCode int
		service            *mockedService
		expectedVersions   *ProviderVersions
		expectError        bool
	}{
		{
			name: "provider not in upstream and not in mirror",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
			},
			upstreamStatusCode: http.StatusNotFound,
			service: &mockedService{
				versionsResponse: &ProviderVersions{
					Versions: map[string]EmptyObject{},
				},
				versionsError: storage.ErrProviderNotMirrored{
					Err: fmt.Errorf("mocked Error"),
				},
			},
			expectError: true,
		},
		{
			name: "provider not in upstream",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
			},
			upstreamStatusCode: http.StatusNotFound,
			service: &mockedService{
				versionsResponse: &ProviderVersions{
					Versions: map[string]EmptyObject{
						"2.0.0": {},
					},
				},
			},
			expectedVersions: &ProviderVersions{
				Versions: map[string]EmptyObject{
					"2.0.0": {},
				},
			},
			expectError: false,
		},
		{
			name: "provider in upstream and mirror",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
			},
			upstreamStatusCode: http.StatusOK,
			service: &mockedService{
				versionsResponse: &ProviderVersions{
					Versions: map[string]EmptyObject{
						"2.0.0": {},
					},
				},
			},
			expectedVersions: &ProviderVersions{
				map[string]EmptyObject{
					"2.0.0": {},
					"2.0.1": {},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := proxyRegistry{
				next:               tc.service,
				logger:             log.NewNopLogger(),
				upstreamRegistries: make(map[string]endpoint.Endpoint),
			}

			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(r.Method, http.MethodGet)
				assert.Equal(r.URL.Path, "/v1/providers/hashicorp/random/versions")

				w.WriteHeader(tc.upstreamStatusCode)
				if tc.upstreamStatusCode != 200 {
					_, _ = w.Write([]byte("{\"errors\":[\"Not Found\"]}"))
					return
				}

				_, _ = w.Write([]byte(`{"versions":[{"version":"2.0.1"}]}`))
			}))
			defer ts.Close()

			// The client uses TLS, thus we need to create certs and pass them to the client
			cert, err := x509.ParseCertificate(ts.TLS.Certificates[0].Certificate[0])
			if err != nil {
				t.Error(fmt.Sprintf("failed to set up test: %v", err))
			}

			certPool := x509.NewCertPool()
			certPool.AddCert(cert)

			p.upstreamClient = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs: certPool,
					},
				},
			}

			provider := tc.provider
			provider.Hostname = strings.TrimPrefix(ts.URL, "https://") // We need to override the Hostname with the test servers hostname
			providerVersions, err := p.ListProviderVersions(context.Background(), provider)

			if tc.expectError {
				assert.Error(err)
				assert.Nil(providerVersions)
				return
			}

			assert.Equal(len(tc.expectedVersions.Versions), len(providerVersions.Versions))
			for version, _ := range tc.expectedVersions.Versions {
				_, ok := providerVersions.Versions[version]
				assert.Equal(true, ok)
			}
		})
	}
}
