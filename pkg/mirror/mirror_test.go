package mirror

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
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
	versionsResponse      *ProviderVersions
	archivesResponse      *Archives
	mockedArchiveResponse io.Reader
	mockedErr             error
}

func (m *mockedService) ListProviderVersions(_ context.Context, _ core.Provider) (*ProviderVersions, error) {
	return m.versionsResponse, m.mockedErr
}

func (m *mockedService) ListProviderInstallation(ctx context.Context, provider core.Provider) (*Archives, error) {
	return m.archivesResponse, m.mockedErr
}

func (m *mockedService) RetrieveProviderArchive(ctx context.Context, provider core.Provider) (io.Reader, error) {
	return m.mockedArchiveResponse, m.mockedErr
}

func (m *mockedService) MirrorProvider(ctx context.Context, provider core.Provider, binary, shasum, shasumSignature io.Reader) error {
	return nil
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
				mockedErr: &storage.ErrProviderNotMirrored{
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

			c, err := createTlsClient(ts)
			if err != nil {
				t.Error(err)
			}
			p.defaultClient = c

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

func TestProxyRegistry_ListProviderInstallation(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testCases := []struct {
		name               string
		provider           core.Provider
		upstreamStatusCode int
		service            *mockedService
		expectedArchives   *Archives
		expectError        bool
	}{
		{
			name: "provider not in upstream and not in mirror",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
				Version:   "2.0.0",
			},
			upstreamStatusCode: http.StatusNotFound,
			service: &mockedService{
				mockedErr: &storage.ErrProviderNotMirrored{
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
				Version:   "2.0.0",
			},
			upstreamStatusCode: http.StatusNotFound,
			service: &mockedService{
				archivesResponse: &Archives{
					Archives: map[string]Archive{
						"linux_amd64": {
							Url: "terraform-provider-random_2.0.0_linux_amd64.zip",
						},
					},
				},
			},
			expectedArchives: &Archives{
				Archives: map[string]Archive{
					"linux_amd64": {
						Url: "terraform-provider-random_2.0.0_linux_amd64.zip",
					},
				},
			},
			expectError: false,
		},
		{
			name: "provider not in mirror",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
				Version:   "2.0.0",
			},
			upstreamStatusCode: http.StatusOK,
			service: &mockedService{
				mockedErr: &storage.ErrProviderNotMirrored{Err: errors.New("mocked error")},
			},
			expectedArchives: &Archives{
				Archives: map[string]Archive{
					"linux_amd64": {
						Url: "terraform-provider-random_2.0.0_linux_amd64.zip",
					},
				},
			},
			expectError: false,
		},
		{
			name: "provider in upstream and mirror",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
				Version:   "2.0.0",
			},
			upstreamStatusCode: http.StatusOK,
			service: &mockedService{
				archivesResponse: &Archives{
					Archives: map[string]Archive{
						"linux_amd64": {
							Url: "terraform-provider-random_2.0.0_linux_amd64.zip",
						},
					},
				},
			},
			expectedArchives: &Archives{
				Archives: map[string]Archive{
					"linux_amd64": {
						Url: "terraform-provider-random_2.0.0_linux_amd64.zip",
					},
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

				_, _ = w.Write([]byte(`{"versions":[{"version":"2.0.0","platforms":[{"os":"linux","arch":"amd64"}]}]}`))
			}))
			defer ts.Close()

			c, err := createTlsClient(ts)
			if err != nil {
				t.Error(err)
			}
			p.defaultClient = c

			provider := tc.provider
			provider.Hostname = strings.TrimPrefix(ts.URL, "https://") // We need to override the Hostname with the test servers hostname
			archives, err := p.ListProviderInstallation(context.Background(), provider)

			if tc.expectError {
				assert.Error(err)
				assert.Nil(archives)
				return
			}

			assert.Equal(len(tc.expectedArchives.Archives), len(archives.Archives))
			for archive, expectedValue := range tc.expectedArchives.Archives {
				actualValue, ok := archives.Archives[archive]
				assert.Equal(true, ok)
				assert.Equal(expectedValue.Url, actualValue.Url)
			}
		})
	}
}

func TestProxyRegistry_RetrieveProviderArchive(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testCases := []struct {
		name               string
		provider           core.Provider
		service            *mockedService
		upstreamStatusCode int
		expectError        bool
	}{
		{
			name: "provider not in upstream and not in mirror",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
				Version:   "2.0.0",
				OS:        "linux",
				Arch:      "amd64",
			},
			upstreamStatusCode: http.StatusNotFound,
			service: &mockedService{
				mockedErr: &storage.ErrProviderNotMirrored{
					Err: fmt.Errorf("mocked Error"),
				},
			},
			expectError: true,
		},
		{
			name: "provider not in mirror",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
				Version:   "2.0.0",
				OS:        "linux",
				Arch:      "amd64",
			},
			upstreamStatusCode: http.StatusOK,
			service: &mockedService{
				mockedErr: &storage.ErrProviderNotMirrored{Err: errors.New("mocked error")},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := proxyRegistry{
				next:               tc.service,
				logger:             log.NewNopLogger(),
				upstreamRegistries: make(map[string]endpoint.Endpoint),
			}

			hostname := ""
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(r.Method, http.MethodGet)
				w.WriteHeader(tc.upstreamStatusCode)
				if tc.upstreamStatusCode != 200 {
					_, _ = w.Write([]byte("{\"errors\":[\"Not Found\"]}"))
					return
				} else if r.URL.Path == "/v1/providers/hashicorp/random/2.0.0/download/linux/amd64" {
					f, err := tc.provider.ArchiveFileName()
					assert.NoError(err)
					sha, err := tc.provider.ShasumFileName()
					assert.NoError(err)
					shaSig, err := tc.provider.ShasumSignatureFileName()
					assert.NoError(err)

					request := downloadResponse{
						OS:                  tc.provider.OS,
						Arch:                tc.provider.Arch,
						Filename:            f,
						DownloadURL:         fmt.Sprintf("%s/v1/%s", hostname, f),
						ShasumsURL:          fmt.Sprintf("%s/v1/%s", hostname, sha),
						ShasumsSignatureURL: fmt.Sprintf("%s/v1/%s", hostname, shaSig),
					}
					b, err := json.Marshal(request)
					assert.NoError(err)
					_, _ = w.Write(b)
				} else if r.URL.Path == "/v1/terraform-provider-random_2.0.0_linux_amd64.zip" ||
					r.URL.Path == "/v1/terraform-provider-random_2.0.0_SHA256SUMS" ||
					r.URL.Path == "/v1/terraform-provider-random_2.0.0_SHA256SUMS.sig" {
					// do nothing
				} else {
					assert.Fail("unknown path")
				}
			}))
			defer ts.Close()
			hostname = ts.URL

			defaultClient, err := createTlsClient(ts)
			if err != nil {
				t.Error(err)
			}
			p.defaultClient = defaultClient

			downloadClient, err := createTlsClient(ts)
			if err != nil {
				t.Error(err)
			}
			p.downloadClient = downloadClient

			provider := tc.provider
			provider.Hostname = strings.TrimPrefix(ts.URL, "https://") // We need to override the Hostname with the test servers hostname
			reader, err := p.RetrieveProviderArchive(context.Background(), provider)
			if tc.expectError {
				assert.Error(err)
				assert.Nil(reader)
				return
			}
			assert.NotNil(reader)
		})
	}

}

// The client uses TLS, thus we need to create certs and pass them to the client
func createTlsClient(server *httptest.Server) (*http.Client, error) {
	cert, err := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to set up test: %w", err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert)

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}, nil
}
