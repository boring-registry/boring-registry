package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"

	assertion "github.com/stretchr/testify/assert"
)

type mockOCIClient struct {
	artifactExists      func(ctx context.Context, reference string) (bool, error)
	downloadArtifact    func(ctx context.Context, reference string) ([]byte, error)
	uploadArtifact      func(ctx context.Context, reference string, content io.Reader, overwrite bool) error
	listTags            func(ctx context.Context, repository string, callback func(tags []string) error) error
	generateDownloadURL func(ctx context.Context, reference string) (string, error)
}

func (m *mockOCIClient) ArtifactExists(ctx context.Context, reference string) (bool, error) {
	if m.artifactExists != nil {
		return m.artifactExists(ctx, reference)
	}
	return true, nil
}

func (m *mockOCIClient) DownloadArtifact(ctx context.Context, reference string) ([]byte, error) {
	if m.downloadArtifact != nil {
		return m.downloadArtifact(ctx, reference)
	}
	return nil, nil
}

func (m *mockOCIClient) UploadArtifact(ctx context.Context, reference string, content io.Reader, overwrite bool) error {
	if m.uploadArtifact != nil {
		return m.uploadArtifact(ctx, reference, content, overwrite)
	}
	return nil
}

func (m *mockOCIClient) ListTags(ctx context.Context, repository string, callback func(tags []string) error) error {
	if m.listTags != nil {
		return m.listTags(ctx, repository, callback)
	}
	return callback([]string{})
}

func (m *mockOCIClient) GenerateDownloadURL(ctx context.Context, reference string) (string, error) {
	if m.generateDownloadURL != nil {
		return m.generateDownloadURL(ctx, reference)
	}
	return reference, nil
}

func artifactExists(ctx context.Context, reference string) (bool, error) {
	return true, nil
}

func artifactNotExists(ctx context.Context, reference string) (bool, error) {
	return false, nil
}

func TestOCIStorage_UploadProviderReleaseFiles(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		namespace   string
		name        string
		filename    string
		content     string
		client      ociClientAPI
		wantErr     assertion.ErrorAssertionFunc
	}{
		{
			description: "provider file exists already",
			namespace:   "hashicorp",
			name:        "random",
			filename:    "terraform-provider-random_2.0.0_linux_amd64.zip",
			client: &mockOCIClient{
				artifactExists: artifactExists,
			},
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.Error(t, err)
			},
		},
		{
			description: "upload file successfully",
			namespace:   "hashicorp",
			name:        "random",
			filename:    "terraform-provider-random_2.0.0_linux_amd64.zip",
			content:     "test",
			client: &mockOCIClient{
				artifactExists: artifactNotExists,
				uploadArtifact: func(ctx context.Context, reference string, content io.Reader, overwrite bool) error {
					return nil
				},
			},
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			s := OCIStorage{
				client:     tc.client,
				registry:   "registry.example.com",
				repository: "boring-registry",
			}
			err := s.UploadProviderReleaseFiles(context.Background(), tc.namespace, tc.name, tc.filename, strings.NewReader(tc.content))
			tc.wantErr(t, err)
		})
	}
}

func TestOCISigningKeys(t *testing.T) {
	var (
		validGPGPublicKey = core.GPGPublicKey{
			KeyID:      "51852D87348FFC4C",
			ASCIIArmor: "-----BEGIN LPGP PUBLIC KEY BLOCK-----\\nVersion: GnuPG v1\\n\\nmQENBFMORM0BCADBRyKO1MhCirazOSVwcfTr1xUxjPvfxD3hjUwHtjsOy/bT6p9f\\nW2mRPfwnq2JB5As+paL3UGDsSRDnK9KAxQb0NNF4+eVhr/EJ18s3wwXXDMjpIifq\\nfIm2WyH3G+aRLTLPIpscUNKDyxFOUbsmgXAmJ46Re1fn8uKxKRHbfa39aeuEYWFA\\n3drdL1WoUngvED7f+RnKBK2G6ZEpO+LDovQk19xGjiMTtPJrjMjZJ3QXqPvx5wca\\nKSZLr4lMTuoTI/ZXyZy5bD4tShiZz6KcyX27cD70q2iRcEZ0poLKHyEIDAi3TM5k\\nSwbbWBFd5RNPOR0qzrb/0p9ksKK48IIfH2FvABEBAAG0K0hhc2hpQ29ycCBTZWN1\\ncml0eSA8c2VjdXJpdHlAaGFzaGljb3JwLmNvbT6JATgEEwECACIFAlMORM0CGwMG\\nCwkIBwMCBhUIAgkKCwQWAgMBAh4BAheAAAoJEFGFLYc0j/xMyWIIAIPhcVqiQ59n\\nJc07gjUX0SWBJAxEG1lKxfzS4Xp+57h2xxTpdotGQ1fZwsihaIqow337YHQI3q0i\\nSqV534Ms+j/tU7X8sq11xFJIeEVG8PASRCwmryUwghFKPlHETQ8jJ+Y8+1asRydi\\npsP3B/5Mjhqv/uOK+Vy3zAyIpyDOMtIpOVfjSpCplVRdtSTFWBu9Em7j5I2HMn1w\\nsJZnJgXKpybpibGiiTtmnFLOwibmprSu04rsnP4ncdC2XRD4wIjoyA+4PKgX3sCO\\nklEzKryWYBmLkJOMDdo52LttP3279s7XrkLEE7ia0fXa2c12EQ0f0DQ1tGUvyVEW\\nWmJVccm5bq25AQ0EUw5EzQEIANaPUY04/g7AmYkOMjaCZ6iTp9hB5Rsj/4ee/ln9\\nwArzRO9+3eejLWh53FoN1rO+su7tiXJA5YAzVy6tuolrqjM8DBztPxdLBbEi4V+j\\n2tK0dATdBQBHEh3OJApO2UBtcjaZBT31zrG9K55D+CrcgIVEHAKY8Cb4kLBkb5wM\\nskn+DrASKU0BNIV1qRsxfiUdQHZfSqtp004nrql1lbFMLFEuiY8FZrkkQ9qduixo\\nmTT6f34/oiY+Jam3zCK7RDN/OjuWheIPGj/Qbx9JuNiwgX6yRj7OE1tjUx6d8g9y\\n0H1fmLJbb3WZZbuuGFnK6qrE3bGeY8+AWaJAZ37wpWh1p0cAEQEAAYkBHwQYAQIA\\nCQUCUw5EzQIbDAAKCRBRhS2HNI/8TJntCAClU7TOO/X053eKF1jqNW4A1qpxctVc\\nz8eTcY8Om5O4f6a/rfxfNFKn9Qyja/OG1xWNobETy7MiMXYjaa8uUx5iFy6kMVaP\\n0BXJ59NLZjMARGw6lVTYDTIvzqqqwLxgliSDfSnqUhubGwvykANPO+93BBx89MRG\\nunNoYGXtPlhNFrAsB1VR8+EyKLv2HQtGCPSFBhrjuzH3gxGibNDDdFQLxxuJWepJ\\nEK1UbTS4ms0NgZ2Uknqn1WRU1Ki7rE4sTy68iZtWpKQXZEJa0IGnuI2sSINGcXCJ\\noEIgXTMyCILo34Fa/C6VCm2WBgz9zZO8/rHIiQm1J5zqz0DrDwKBUM9C\\n=LYpS\\n-----END PGP PUBLIC KEY BLOCK-----",
			Source:     "HashiCorp",
			SourceURL:  "https://www.hashicorp.com/security.html",
		}
		validSigningKeys = core.SigningKeys{
			GPGPublicKeys: []core.GPGPublicKey{
				validGPGPublicKey,
			},
		}
	)

	validSigningKeysBytes, err := json.Marshal(validSigningKeys)
	if err != nil {
		t.Fatal(err)
	}

	validGPGPublicKeyBytes, err := json.Marshal(validGPGPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		annotation    string
		data          map[string][]byte
		namespace     string
		returnError   bool
		expectedError bool
		expect        core.SigningKeys
	}{
		{
			annotation:    "empty namespace",
			namespace:     "",
			expectedError: true,
			expect:        validSigningKeys,
		},
		{
			annotation: "fetch fails",
			data: map[string][]byte{
				"registry.example.com/boring-registry/providers/hashicorp:signing-keys.json": validSigningKeysBytes,
			},
			namespace:     "hashicorp",
			returnError:   true,
			expectedError: true,
			expect:        validSigningKeys,
		},
		{
			annotation: "empty object",
			data: map[string][]byte{
				"registry.example.com/boring-registry/providers/hashicorp:signing-keys.json": []byte(""),
			},
			namespace:     "hashicorp",
			expectedError: true,
			expect:        validSigningKeys,
		},
		{
			annotation: "only single gpg_public_key for the provider namespace",
			data: map[string][]byte{
				"registry.example.com/boring-registry/providers/hashicorp:signing-keys.json": validGPGPublicKeyBytes,
			},
			namespace:     "hashicorp",
			expectedError: false,
			expect:        validSigningKeys,
		},
		{
			annotation: "signing_keys with a single gpg_public_key",
			data: map[string][]byte{
				"registry.example.com/boring-registry/providers/hashicorp:signing-keys.json": validSigningKeysBytes,
			},
			namespace:     "hashicorp",
			expectedError: false,
			expect:        validSigningKeys,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.annotation, func(t *testing.T) {
			s := OCIStorage{
				registry:   "registry.example.com",
				repository: "boring-registry",
				client: &mockOCIClient{
					artifactExists: func(ctx context.Context, reference string) (bool, error) {
						return !tc.returnError, nil
					},
					downloadArtifact: func(ctx context.Context, reference string) ([]byte, error) {
						if tc.returnError {
							return nil, errors.New("fetch error")
						}
						if data, exists := tc.data[reference]; exists {
							return data, nil
						}
						return nil, errors.New("not found")
					},
				},
			}

			result, err := s.SigningKeys(context.Background(), tc.namespace)

			if !tc.expectedError {
				assertion.NoError(t, err)
			} else {
				assertion.Error(t, err)
				return
			}

			assertion.Equal(t, &tc.expect, result)
		})
	}
}

func TestOCIStorage_getProvider(t *testing.T) {
	type fields struct {
		client ociClientAPI
	}
	type args struct {
		pt       providerType
		provider *core.Provider
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *core.Provider
		wantErr bool
	}{
		{
			name: "provider does not exist",
			fields: fields{
				client: &mockOCIClient{
					artifactExists: artifactNotExists,
				},
			},
			args: args{
				pt: internalProviderType,
				provider: &core.Provider{
					Namespace: "example",
					Name:      "dummy",
					Version:   "1.0.0",
					OS:        "linux",
					Arch:      "amd64",
				},
			},
			wantErr: true,
		},
		{
			name: "internal provider exists",
			fields: fields{
				client: &mockOCIClient{
					artifactExists: artifactExists,
					generateDownloadURL: func(ctx context.Context, reference string) (string, error) {
						return reference, nil
					},
					downloadArtifact: func(ctx context.Context, reference string) ([]byte, error) {
						if strings.Contains(reference, "SHA256SUMS") && !strings.Contains(reference, ".sig") {
							return []byte("10488a12525ed674359585f83e3ee5e74818b5c98e033798351678b21b2f7d89  terraform-provider-dummy_1.0.0_linux_amd64.zip"), nil
						}
						if strings.Contains(reference, "signing-keys.json") {
							return []byte(`{"gpg_public_keys":[{"key_id":"47422B4AA9FA381B","ascii_armor":"test"}]}`), nil
						}
						return nil, nil
					},
				},
			},
			args: args{
				pt: internalProviderType,
				provider: &core.Provider{
					Namespace: "example",
					Name:      "dummy",
					Version:   "1.0.0",
					OS:        "linux",
					Arch:      "amd64",
				},
			},
			want: &core.Provider{
				Namespace:           "example",
				Name:                "dummy",
				Version:             "1.0.0",
				OS:                  "linux",
				Arch:                "amd64",
				Filename:            "terraform-provider-dummy_1.0.0_linux_amd64.zip",
				DownloadURL:         "registry.example.com/boring-registry/providers/example/dummy:terraform-provider-dummy_1.0.0_linux_amd64.zip",
				Shasum:              "10488a12525ed674359585f83e3ee5e74818b5c98e033798351678b21b2f7d89",
				SHASumsURL:          "registry.example.com/boring-registry/providers/example/dummy:terraform-provider-dummy_1.0.0_SHA256SUMS",
				SHASumsSignatureURL: "registry.example.com/boring-registry/providers/example/dummy:terraform-provider-dummy_1.0.0_SHA256SUMS.sig",
				SigningKeys: core.SigningKeys{
					GPGPublicKeys: []core.GPGPublicKey{
						{
							KeyID:      "47422B4AA9FA381B",
							ASCIIArmor: "test",
						},
					},
				},
			},
		},
		{
			name: "mirrored provider exists",
			fields: fields{
				client: &mockOCIClient{
					artifactExists: artifactExists,
					generateDownloadURL: func(ctx context.Context, reference string) (string, error) {
						return reference, nil
					},
					downloadArtifact: func(ctx context.Context, reference string) ([]byte, error) {
						if strings.Contains(reference, "SHA256SUMS") && !strings.Contains(reference, ".sig") {
							return []byte("10488a12525ed674359585f83e3ee5e74818b5c98e033798351678b21b2f7d89  terraform-provider-dummy_1.0.0_linux_amd64.zip"), nil
						}
						if strings.Contains(reference, "signing-keys.json") {
							return []byte(`{"gpg_public_keys":[{"key_id":"47422B4AA9FA381B","ascii_armor":"test"}]}`), nil
						}
						return nil, nil
					},
				},
			},
			args: args{
				pt: mirrorProviderType,
				provider: &core.Provider{
					Hostname:  "terraform.example.com",
					Namespace: "example",
					Name:      "dummy",
					Version:   "1.0.0",
					OS:        "linux",
					Arch:      "amd64",
				},
			},
			want: &core.Provider{
				Hostname:            "terraform.example.com",
				Namespace:           "example",
				Name:                "dummy",
				Version:             "1.0.0",
				OS:                  "linux",
				Arch:                "amd64",
				Filename:            "terraform-provider-dummy_1.0.0_linux_amd64.zip",
				DownloadURL:         "registry.example.com/boring-registry/mirror/providers/terraform.example.com/example/dummy:terraform-provider-dummy_1.0.0_linux_amd64.zip",
				Shasum:              "10488a12525ed674359585f83e3ee5e74818b5c98e033798351678b21b2f7d89",
				SHASumsURL:          "registry.example.com/boring-registry/mirror/providers/terraform.example.com/example/dummy:terraform-provider-dummy_1.0.0_SHA256SUMS",
				SHASumsSignatureURL: "registry.example.com/boring-registry/mirror/providers/terraform.example.com/example/dummy:terraform-provider-dummy_1.0.0_SHA256SUMS.sig",
				SigningKeys: core.SigningKeys{
					GPGPublicKeys: []core.GPGPublicKey{
						{
							KeyID:      "47422B4AA9FA381B",
							ASCIIArmor: "test",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &OCIStorage{
				registry:   "registry.example.com",
				repository: "boring-registry",
				client:     tt.fields.client,
			}
			got, err := s.getProvider(context.Background(), tt.args.pt, tt.args.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("OCIStorage.getProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OCIStorage.getProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOCIStorage_GetModule(t *testing.T) {
	tests := []struct {
		name        string
		client      ociClientAPI
		namespace   string
		moduleName  string
		provider    string
		version     string
		want        core.Module
		wantErr     bool
	}{
		{
			name: "module exists",
			client: &mockOCIClient{
				artifactExists: artifactExists,
				generateDownloadURL: func(ctx context.Context, reference string) (string, error) {
					return reference, nil
				},
			},
			namespace:  "example",
			moduleName: "mymodule",
			provider:   "aws",
			version:    "1.0.0",
			want: core.Module{
				Namespace:   "example",
				Name:        "mymodule",
				Provider:    "aws",
				Version:     "1.0.0",
				DownloadURL: "registry.example.com/boring-registry/modules/example/mymodule/aws:example-mymodule-aws-1.0.0",
			},
			wantErr: false,
		},
		{
			name: "module does not exist",
			client: &mockOCIClient{
				artifactExists: artifactNotExists,
			},
			namespace:  "example",
			moduleName: "mymodule",
			provider:   "aws",
			version:    "1.0.0",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &OCIStorage{
				registry:   "registry.example.com",
				repository: "boring-registry",
				client:     tt.client,
			}
			got, err := s.GetModule(context.Background(), tt.namespace, tt.moduleName, tt.provider, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("OCIStorage.GetModule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OCIStorage.GetModule() = %v, want %v", got, tt.want)
			}
		})
	}
}