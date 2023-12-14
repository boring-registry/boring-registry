package mirror

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"
)

var exampleSigningKeys = core.SigningKeys{
	GPGPublicKeys: []core.GPGPublicKey{
		{
			KeyID: "123456789",
			ASCIIArmor: `-----BEGIN PGP PUBLIC KEY BLOCK-----
-----END PGP PUBLIC KEY BLOCK-----`,
		},
	},
}

func Test_copier_signingKeys(t *testing.T) {
	type fields struct {
		done    chan struct{}
		storage Storage
	}
	tests := []struct {
		name     string
		fields   fields
		provider *core.Provider
		wantErr  bool
	}{
		{
			name: "failed to access storage",
			fields: fields{
				storage: &mockedStorage{
					mirroredSigningKeys: func(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error) {
						return nil, errors.New("this is not an ErrObjectNotFound")
					},
				},
			},
			provider: &core.Provider{
				Hostname:  "terraform.example.com",
				Namespace: "example",
			},
			wantErr: true,
		},
		{
			name: "existing signing keys not found",
			fields: fields{
				storage: &mockedStorage{
					mirroredSigningKeys: func(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error) {
						return nil, core.ErrObjectNotFound
					},
					uploadMirroredSigningKeys: func(ctx context.Context, hostname, namespace string, signingKeys *core.SigningKeys) error {
						return nil
					},
				},
			},
			provider: &core.Provider{
				Hostname:    "terraform.example.com",
				Namespace:   "example",
				SigningKeys: exampleSigningKeys,
			},
		},
		{
			name: "existing keys needs updating",
			fields: fields{
				storage: &mockedStorage{
					mirroredSigningKeys: func(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error) {
						return &exampleSigningKeys, nil
					},
					uploadMirroredSigningKeys: func(ctx context.Context, hostname, namespace string, signingKeys *core.SigningKeys) error {
						return nil
					},
				},
			},
			provider: &core.Provider{
				Hostname:  "terraform.example.com",
				Namespace: "example",
				SigningKeys: core.SigningKeys{
					GPGPublicKeys: []core.GPGPublicKey{
						{
							KeyID: "0987654321",
							ASCIIArmor: `-----BEGIN PGP PUBLIC KEY BLOCK-----
-----END PGP PUBLIC KEY BLOCK-----`,
						},
					},
				},
			},
		},
		{
			name: "signing keys exist already",
			fields: fields{
				storage: &mockedStorage{
					mirroredSigningKeys: func(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error) {
						return &exampleSigningKeys, nil
					},
					uploadMirroredSigningKeys: func(ctx context.Context, hostname, namespace string, signingKeys *core.SigningKeys) error {
						return nil
					},
				},
			},
			provider: &core.Provider{
				Hostname:    "terraform.example.com",
				Namespace:   "example",
				SigningKeys: exampleSigningKeys,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &copier{
				done:    tt.fields.done,
				storage: tt.fields.storage,
			}
			if err := m.signingKeys(context.Background(), tt.provider); (err != nil) != tt.wantErr {
				t.Errorf("signingKeys() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_copier_sha256SumsSignature(t *testing.T) {
	type fields struct {
		done    chan struct{}
		storage Storage
	}
	tests := []struct {
		name       string
		statusCode int
		fields     fields
		provider   *core.Provider
		wantErr    bool
	}{
		{
			name:       "connection error",
			statusCode: http.StatusNotFound,
			fields: fields{
				storage: &mockedStorage{
					uploadMirroredFile: func(ctx context.Context, provider *core.Provider, filename string, reader io.Reader) error {
						return nil
					},
				},
			},
			provider: &core.Provider{
				Hostname:            "terraform.example.com",
				Namespace:           "example",
				Name:                "dummy",
				Version:             "1.2.3",
				SHASumsSignatureURL: "http://terraform.example.com",
			},
			wantErr: true,
		},
		{
			name:       "invalid status code",
			statusCode: http.StatusNotFound,
			fields: fields{
				storage: &mockedStorage{
					uploadMirroredFile: func(ctx context.Context, provider *core.Provider, filename string, reader io.Reader) error {
						return nil
					},
				},
			},
			provider: &core.Provider{
				Hostname:  "terraform.example.com",
				Namespace: "example",
				Name:      "dummy",
				Version:   "1.2.3",
			},
			wantErr: true,
		},
		{
			name:       "success",
			statusCode: http.StatusOK,
			fields: fields{
				storage: &mockedStorage{
					uploadMirroredFile: func(ctx context.Context, provider *core.Provider, filename string, reader io.Reader) error {
						return nil
					},
				},
			},
			provider: &core.Provider{
				Hostname:  "terraform.example.com",
				Namespace: "example",
				Name:      "dummy",
				Version:   "1.2.3",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(tt.statusCode)
				if _, err := writer.Write([]byte("hello-terraform")); err != nil {
					panic(err)
				}
			}))

			m := &copier{
				done:    tt.fields.done,
				storage: tt.fields.storage,
				client:  server.Client(),
			}
			if tt.provider.SHASumsSignatureURL == "" {
				tt.provider.SHASumsSignatureURL = server.URL
			}

			if err := m.sha256SumsSignature(context.Background(), tt.provider); (err != nil) != tt.wantErr {
				t.Errorf("sha256SumsSignature() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_copier_sha256Sums(t *testing.T) {
	tests := []struct {
		name       string
		storage    Storage
		statusCode int
		provider   *core.Provider
		wantErr    bool
	}{
		{
			name:       "invalid status code",
			statusCode: http.StatusInternalServerError,
			provider: &core.Provider{
				Hostname:  "terraform.example.com",
				Namespace: "example",
				Name:      "dummy",
				Version:   "1.2.3",
			},
			wantErr: true,
		},
		{
			name:       "successfully mirror SHA256SUM",
			statusCode: http.StatusOK,
			storage: &mockedStorage{
				uploadMirroredFile: func(ctx context.Context, provider *core.Provider, filename string, reader io.Reader) error {
					return nil
				},
			},
			provider: &core.Provider{
				Hostname:  "terraform.example.com",
				Namespace: "example",
				Name:      "dummy",
				Version:   "1.2.3",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(tt.statusCode)
				if _, err := writer.Write([]byte("hello-terraform")); err != nil {
					panic(err)
				}
			}))
			tt.provider.SHASumsURL = server.URL
			m := &copier{
				storage: tt.storage,
				client:  server.Client(),
			}
			if err := m.sha256Sums(context.Background(), tt.provider); (err != nil) != tt.wantErr {
				t.Errorf("sha256Sums() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
