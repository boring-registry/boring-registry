package mirror

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/discovery"
)

type mockedRemoteServiceDiscovery struct {
	resolve func(ctx context.Context, host string) (*discovery.DiscoveredRemoteService, error)
}

func (m *mockedRemoteServiceDiscovery) Resolve(ctx context.Context, host string) (*discovery.DiscoveredRemoteService, error) {
	return m.resolve(ctx, host)
}

func newMockedServiceDiscovery(server *httptest.Server) *mockedRemoteServiceDiscovery {
	return &mockedRemoteServiceDiscovery{
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
				body := []byte(`{
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
}`)
				if _, err := writer.Write(body); err != nil {
					panic(err)
				}
			}))
			u := &upstreamProviderRegistry{
				client:                 server.Client(),
				remoteServiceDiscovery: newMockedServiceDiscovery(server),
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

func Test_upstreamProviderRegistry_getProvider(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		provider   *core.Provider
		want       *core.Provider
		wantErr    bool
	}{
		{
			name:       "successfully retrieve provider",
			statusCode: http.StatusOK,
			provider: &core.Provider{
				Hostname:  "terraform.example.com",
				Namespace: "hashicorp",
				Name:      "random",
				Version:   "2.0.0",
				OS:        "linux",
				Arch:      "amd64",
			},
			want: &core.Provider{
				Hostname:            "terraform.example.com",
				Namespace:           "hashicorp",
				Name:                "random",
				Version:             "2.0.0",
				OS:                  "linux",
				Arch:                "amd64",
				Filename:            "terraform-provider-random_2.0.0_linux_amd64.zip",
				DownloadURL:         "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_linux_amd64.zip",
				SHASumsURL:          "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_SHA256SUMS",
				SHASumsSignatureURL: "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_SHA256SUMS.sig",
				Shasum:              "5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a",
				SigningKeys: core.SigningKeys{
					GPGPublicKeys: []core.GPGPublicKey{
						{
							KeyID:      "51852D87348FFC4C",
							ASCIIArmor: "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFMORM0BCADBRyKO1MhCirazOSVwcfTr1xUxjPvfxD3hjUwHtjsOy/bT6p9f\nW2mRPfwnq2JB5As+paL3UGDsSRDnK9KAxQb0NNF4+eVhr/EJ18s3wwXXDMjpIifq\nfIm2WyH3G+aRLTLPIpscUNKDyxFOUbsmgXAmJ46Re1fn8uKxKRHbfa39aeuEYWFA\n3drdL1WoUngvED7f+RnKBK2G6ZEpO+LDovQk19xGjiMTtPJrjMjZJ3QXqPvx5wca\nKSZLr4lMTuoTI/ZXyZy5bD4tShiZz6KcyX27cD70q2iRcEZ0poLKHyEIDAi3TM5k\nSwbbWBFd5RNPOR0qzrb/0p9ksKK48IIfH2FvABEBAAG0K0hhc2hpQ29ycCBTZWN1\ncml0eSA8c2VjdXJpdHlAaGFzaGljb3JwLmNvbT6JATgEEwECACIFAlMORM0CGwMG\nCwkIBwMCBhUIAgkKCwQWAgMBAh4BAheAAAoJEFGFLYc0j/xMyWIIAIPhcVqiQ59n\nJc07gjUX0SWBJAxEG1lKxfzS4Xp+57h2xxTpdotGQ1fZwsihaIqow337YHQI3q0i\nSqV534Ms+j/tU7X8sq11xFJIeEVG8PASRCwmryUwghFKPlHETQ8jJ+Y8+1asRydi\npsP3B/5Mjhqv/uOK+Vy3zAyIpyDOMtIpOVfjSpCplVRdtSTFWBu9Em7j5I2HMn1w\nsJZnJgXKpybpibGiiTtmnFLOwibmprSu04rsnP4ncdC2XRD4wIjoyA+4PKgX3sCO\nklEzKryWYBmLkJOMDdo52LttP3279s7XrkLEE7ia0fXa2c12EQ0f0DQ1tGUvyVEW\nWmJVccm5bq25AQ0EUw5EzQEIANaPUY04/g7AmYkOMjaCZ6iTp9hB5Rsj/4ee/ln9\nwArzRO9+3eejLWh53FoN1rO+su7tiXJA5YAzVy6tuolrqjM8DBztPxdLBbEi4V+j\n2tK0dATdBQBHEh3OJApO2UBtcjaZBT31zrG9K55D+CrcgIVEHAKY8Cb4kLBkb5wM\nskn+DrASKU0BNIV1qRsxfiUdQHZfSqtp004nrql1lbFMLFEuiY8FZrkkQ9qduixo\nmTT6f34/oiY+Jam3zCK7RDN/OjuWheIPGj/Qbx9JuNiwgX6yRj7OE1tjUx6d8g9y\n0H1fmLJbb3WZZbuuGFnK6qrE3bGeY8+AWaJAZ37wpWh1p0cAEQEAAYkBHwQYAQIA\nCQUCUw5EzQIbDAAKCRBRhS2HNI/8TJntCAClU7TOO/X053eKF1jqNW4A1qpxctVc\nz8eTcY8Om5O4f6a/rfxfNFKn9Qyja/OG1xWNobETy7MiMXYjaa8uUx5iFy6kMVaP\n0BXJ59NLZjMARGw6lVTYDTIvzqqqwLxgliSDfSnqUhubGwvykANPO+93BBx89MRG\nunNoYGXtPlhNFrAsB1VR8+EyKLv2HQtGCPSFBhrjuzH3gxGibNDDdFQLxxuJWepJ\nEK1UbTS4ms0NgZ2Uknqn1WRU1Ki7rE4sTy68iZtWpKQXZEJa0IGnuI2sSINGcXCJ\noEIgXTMyCILo34Fa/C6VCm2WBgz9zZO8/rHIiQm1J5zqz0DrDwKBUM9C\n=LYpS\n-----END PGP PUBLIC KEY BLOCK-----",
							Source:     "HashiCorp",
							SourceURL:  "https://www.hashicorp.com/security.html",
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
				body := []byte(`{
					"protocols": ["4.0", "5.1"],
					"os": "linux",
					"arch": "amd64",
					"filename": "terraform-provider-random_2.0.0_linux_amd64.zip",
					"download_url": "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_linux_amd64.zip",
					"shasums_url": "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_SHA256SUMS",
					"shasums_signature_url": "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_SHA256SUMS.sig",
					"shasum": "5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a",
					"signing_keys": {
					  "gpg_public_keys": [
						{
						  "key_id": "51852D87348FFC4C",
						  "ascii_armor": "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFMORM0BCADBRyKO1MhCirazOSVwcfTr1xUxjPvfxD3hjUwHtjsOy/bT6p9f\nW2mRPfwnq2JB5As+paL3UGDsSRDnK9KAxQb0NNF4+eVhr/EJ18s3wwXXDMjpIifq\nfIm2WyH3G+aRLTLPIpscUNKDyxFOUbsmgXAmJ46Re1fn8uKxKRHbfa39aeuEYWFA\n3drdL1WoUngvED7f+RnKBK2G6ZEpO+LDovQk19xGjiMTtPJrjMjZJ3QXqPvx5wca\nKSZLr4lMTuoTI/ZXyZy5bD4tShiZz6KcyX27cD70q2iRcEZ0poLKHyEIDAi3TM5k\nSwbbWBFd5RNPOR0qzrb/0p9ksKK48IIfH2FvABEBAAG0K0hhc2hpQ29ycCBTZWN1\ncml0eSA8c2VjdXJpdHlAaGFzaGljb3JwLmNvbT6JATgEEwECACIFAlMORM0CGwMG\nCwkIBwMCBhUIAgkKCwQWAgMBAh4BAheAAAoJEFGFLYc0j/xMyWIIAIPhcVqiQ59n\nJc07gjUX0SWBJAxEG1lKxfzS4Xp+57h2xxTpdotGQ1fZwsihaIqow337YHQI3q0i\nSqV534Ms+j/tU7X8sq11xFJIeEVG8PASRCwmryUwghFKPlHETQ8jJ+Y8+1asRydi\npsP3B/5Mjhqv/uOK+Vy3zAyIpyDOMtIpOVfjSpCplVRdtSTFWBu9Em7j5I2HMn1w\nsJZnJgXKpybpibGiiTtmnFLOwibmprSu04rsnP4ncdC2XRD4wIjoyA+4PKgX3sCO\nklEzKryWYBmLkJOMDdo52LttP3279s7XrkLEE7ia0fXa2c12EQ0f0DQ1tGUvyVEW\nWmJVccm5bq25AQ0EUw5EzQEIANaPUY04/g7AmYkOMjaCZ6iTp9hB5Rsj/4ee/ln9\nwArzRO9+3eejLWh53FoN1rO+su7tiXJA5YAzVy6tuolrqjM8DBztPxdLBbEi4V+j\n2tK0dATdBQBHEh3OJApO2UBtcjaZBT31zrG9K55D+CrcgIVEHAKY8Cb4kLBkb5wM\nskn+DrASKU0BNIV1qRsxfiUdQHZfSqtp004nrql1lbFMLFEuiY8FZrkkQ9qduixo\nmTT6f34/oiY+Jam3zCK7RDN/OjuWheIPGj/Qbx9JuNiwgX6yRj7OE1tjUx6d8g9y\n0H1fmLJbb3WZZbuuGFnK6qrE3bGeY8+AWaJAZ37wpWh1p0cAEQEAAYkBHwQYAQIA\nCQUCUw5EzQIbDAAKCRBRhS2HNI/8TJntCAClU7TOO/X053eKF1jqNW4A1qpxctVc\nz8eTcY8Om5O4f6a/rfxfNFKn9Qyja/OG1xWNobETy7MiMXYjaa8uUx5iFy6kMVaP\n0BXJ59NLZjMARGw6lVTYDTIvzqqqwLxgliSDfSnqUhubGwvykANPO+93BBx89MRG\nunNoYGXtPlhNFrAsB1VR8+EyKLv2HQtGCPSFBhrjuzH3gxGibNDDdFQLxxuJWepJ\nEK1UbTS4ms0NgZ2Uknqn1WRU1Ki7rE4sTy68iZtWpKQXZEJa0IGnuI2sSINGcXCJ\noEIgXTMyCILo34Fa/C6VCm2WBgz9zZO8/rHIiQm1J5zqz0DrDwKBUM9C\n=LYpS\n-----END PGP PUBLIC KEY BLOCK-----",
						  "trust_signature": "",
						  "source": "HashiCorp",
						  "source_url": "https://www.hashicorp.com/security.html"
						}
					  ]
					}
				  }
				  `)
				if _, err := writer.Write(body); err != nil {
					panic(err)
				}
			}))
			u := &upstreamProviderRegistry{
				client:                 server.Client(),
				remoteServiceDiscovery: newMockedServiceDiscovery(server),
			}

			got, err := u.getProvider(context.Background(), tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("upstreamProviderRegistry.getProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("upstreamProviderRegistry.getProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_upstreamProviderRegistry_shaSums(t *testing.T) {
	tests := []struct {
		name     string
		provider *core.Provider
		want     *core.Sha256Sums
		wantErr  bool
	}{
		{
			name: "successfully retrieve SHASUMS file",
			provider: &core.Provider{
				Hostname:  "terraform.example.com",
				Namespace: "hashicorp",
				Name:      "random",
				Version:   "2.0.0",
				OS:        "linux",
				Arch:      "amd64",
			},
			want: &core.Sha256Sums{
				Filename: "terraform-provider-random_2.0.0_SHA256SUMS",
				Entries: func() map[string][]byte {
					amd64Sum, err := hex.DecodeString("5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a")
					if err != nil {
						panic(err)
					}
					armSum, err := hex.DecodeString("29df160b8b618227197cc9984c47412461ad66a300a8fc1db4052398bf5656ac")
					if err != nil {
						panic(err)
					}
					return map[string][]byte{
						"terraform-provider-random_2.0.0_linux_amd64.zip": amd64Sum,
						"terraform-provider-random_2.0.0_linux_arm.zip":   armSum,
					}
				}(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusOK)
				body := []byte(`5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a  terraform-provider-random_2.0.0_linux_amd64.zip
29df160b8b618227197cc9984c47412461ad66a300a8fc1db4052398bf5656ac  terraform-provider-random_2.0.0_linux_arm.zip`)
				if _, err := writer.Write(body); err != nil {
					panic(err)
				}
			}))
			u := &upstreamProviderRegistry{
				client:                 server.Client(),
				remoteServiceDiscovery: newMockedServiceDiscovery(server),
			}
			tt.provider.SHASumsURL = server.URL

			got, err := u.shaSums(context.Background(), tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("upstreamProviderRegistry.shaSums() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("upstreamProviderRegistry.shaSums() = %v, want %v", got, tt.want)
			}
		})
	}
}
