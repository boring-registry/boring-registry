package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"
)

type mockProviderStorage struct {
	uploadProviderReleaseFilesFn func(ctx context.Context, namespace, name, fileName string, content io.Reader) error
}

// GetProvider implements provider.Storage
func (m *mockProviderStorage) GetProvider(ctx context.Context, namespace string, name string, version string, os string, arch string) (*core.Provider, error) {
	panic("unimplemented")
}

// ListProviderVersions implements provider.Storage
func (m *mockProviderStorage) ListProviderVersions(ctx context.Context, namespace string, name string) (*core.ProviderVersions, error) {
	panic("unimplemented")
}

// SigningKeys implements provider.Storage
func (m *mockProviderStorage) SigningKeys(ctx context.Context, namespace string) (*core.SigningKeys, error) {
	panic("unimplemented")
}

// UploadProviderReleaseFiles implements provider.Storage
func (m *mockProviderStorage) UploadProviderReleaseFiles(ctx context.Context, namespace, name, fileName string, content io.Reader) error {
	return m.uploadProviderReleaseFilesFn(ctx, namespace, name, fileName, content)
}

func TestUploadProviderRunner_uploadArtifacts(t *testing.T) {
	tests := []struct {
		name                     string
		flagProviderArchivePaths []string
		flagFileSha256Sums       string
		flagProviderNamespace    string
		uploadError              error
		wantErr                  bool
	}{
		{
			name:                     "successful upload with multiple archive paths",
			flagProviderArchivePaths: []string{"/path/to/archive1.zip", "/path/to/archive2.zip"},
			flagFileSha256Sums:       "/path/to/SHA256SUMS",
			flagProviderNamespace:    "test-namespace",
			wantErr:                  false,
		},
		{
			name:                     "successful upload without archive paths",
			flagProviderArchivePaths: nil,
			flagFileSha256Sums:       "/path/to/SHA256SUMS",
			flagProviderNamespace:    "test-namespace",
			wantErr:                  false,
		},
		{
			name:                     "upload error",
			flagProviderArchivePaths: []string{"/path/to/archive.zip"},
			flagFileSha256Sums:       "/path/to/SHA256SUMS",
			flagProviderNamespace:    "test-namespace",
			uploadError:              errors.New("upload failed"),
			wantErr:                  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test state
			flagProviderArchivePaths = tt.flagProviderArchivePaths
			flagFileSha256Sums = tt.flagFileSha256Sums
			flagProviderNamespace = tt.flagProviderNamespace

			u := &uploadProviderRunner{
				artifactUploader: func(ctx context.Context, path, namespace, name string) error {

					return tt.uploadError
				},
			}

			// Create mock SHA256SUMS
			sums := &core.Sha256Sums{
				Entries: map[string][]byte{
					"test-provider.zip": []byte("test-checksum"),
				},
			}

			err := u.uploadArtifacts(context.Background(), sums, "/test/path", "test-namespace", "test-provider")

			if (err != nil) != tt.wantErr {
				t.Errorf("uploadArtifacts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUploadProviderRunner_uploadProviderReleaseFile(t *testing.T) {
	type file struct {
		name      string
		content   []byte
		notExists bool
	}

	tests := []struct {
		name          string
		file          file
		namespace     string
		providerName  string
		uploadError   error
		wantErr       bool
		expectedError string
	}{
		{
			name:         "successful upload",
			file:         file{name: "test-provider.zip", content: []byte("dummy content")},
			namespace:    "test-namespace",
			providerName: "test-provider",
			wantErr:      false,
		},
		{
			name:          "nonexistent file error",
			file:          file{name: "nonexistent.zip", notExists: true},
			namespace:     "test-namespace",
			providerName:  "test-provider",
			wantErr:       true,
			expectedError: "no such file or directory",
		},
		{
			name:          "storage upload error",
			file:          file{name: "test-provider.zip"},
			namespace:     "test-namespace",
			providerName:  "test-provider",
			uploadError:   errors.New("upload failed"),
			wantErr:       true,
			expectedError: "upload failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tt.file.name)
			if !tt.file.notExists {
				if err := os.WriteFile(path, tt.file.content, 0644); err != nil {
					t.Fatalf("failed to create temp file: %v", err)
				}
			}

			u := &uploadProviderRunner{
				storage: &mockProviderStorage{
					uploadProviderReleaseFilesFn: func(ctx context.Context, namespace, name, fileName string, content io.Reader) error {
						return tt.uploadError
					},
				},
			}

			err := u.uploadProviderReleaseFile(context.Background(), path, tt.namespace, tt.providerName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q but got %q", tt.expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
