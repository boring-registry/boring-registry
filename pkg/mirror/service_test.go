package mirror

import (
	"context"
	"errors"
	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

type mockedStorage struct {
	mirroredProvidersResponse *[]core.Provider
	errResponse               error
}

func (m mockedStorage) EnumerateMirroredProviders(ctx context.Context, provider core.Provider) (*[]core.Provider, error) {
	return m.mirroredProvidersResponse, m.errResponse
}

func (m mockedStorage) RetrieveMirroredProviderArchive(ctx context.Context, provider core.Provider) (io.ReadCloser, error) {
	// Method is not implemented yet, as we don't need it for now
	panic("implement me")
}

func (m mockedStorage) StoreMirroredProvider(ctx context.Context, provider core.Provider, binary, shasum, shasumSignature io.Reader) error {
	// Method is not implemented yet, as we don't need it for now
	panic("implement me")
}

func TestService_ListProviderVersions(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testCases := []struct {
		name        string
		provider    core.Provider
		storage     *mockedStorage
		result      *ProviderVersions
		expectError bool
	}{
		{
			name: "propagate storage failure",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
			},
			storage: &mockedStorage{
				errResponse: errors.New("mocked error"),
			},
			expectError: true,
		},
		{
			name: "providers fetched successfully",
			provider: core.Provider{
				Hostname:  "registry.terraform.io",
				Namespace: "hashicorp",
				Name:      "random",
			},
			storage: &mockedStorage{
				mirroredProvidersResponse: &[]core.Provider{
					{
						Hostname:  "registry.terraform.io",
						Namespace: "hashicorp",
						Name:      "Name",
						Version:   "2.0.0",
					},
					{
						Hostname:  "registry.terraform.io",
						Namespace: "hashicorp",
						Name:      "Name",
						Version:   "2.0.1",
					},
				},
			},
			result: &ProviderVersions{
				Versions: map[string]EmptyObject{
					"2.0.0": {},
					"2.0.1": {},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := service{
				storage: tc.storage,
			}

			providerVersions, err := s.ListProviderVersions(context.Background(), tc.provider)
			if tc.expectError {
				assert.Error(err)
				assert.Nil(providerVersions)
				return
			}

			assert.NotNil(providerVersions)
			for version, _ := range providerVersions.Versions {
				_, ok := tc.result.Versions[version]
				assert.Equal(true, ok)
			}
		})
	}
}

func TestService_ListProviderInstallation(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testCases := []struct {
		name        string
		provider    core.Provider
		storage     *mockedStorage
		result      *Archives
		expectError bool
	}{
		{
			name: "propagate storage failure",
			provider: core.Provider{
				Namespace: "hashicorp",
				Name:      "random",
			},
			storage: &mockedStorage{
				errResponse: errors.New("mocked error"),
			},
			expectError: true,
		},
		{
			name: "providers fetched successfully",
			provider: core.Provider{
				Hostname:  "registry.terraform.io",
				Namespace: "hashicorp",
				Name:      "random",
			},
			storage: &mockedStorage{
				mirroredProvidersResponse: &[]core.Provider{
					{
						Hostname:  "registry.terraform.io",
						Namespace: "hashicorp",
						Name:      "Name",
						Version:   "2.0.0",
						OS:        "linux",
						Arch:      "amd64",
					},
					{
						Hostname:  "registry.terraform.io",
						Namespace: "hashicorp",
						Name:      "Name",
						Version:   "2.0.0",
						OS:        "linux",
						Arch:      "arm64",
					},
				},
			},
			result: &Archives{
				Archives: map[string]Archive{
					"linux_amd64": {
						Url: "terraform-provider-Name_2.0.0_linux_amd64.zip",
					},
					"linux_arm64": {
						Url: "terraform-provider-Name_2.0.0_linux_arm64.zip",
					},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := service{
				storage: tc.storage,
			}

			archives, err := s.ListProviderInstallation(context.Background(), tc.provider)
			if tc.expectError {
				assert.Error(err)
				assert.Nil(archives)
				return
			}

			assert.NotNil(archives)
			for version, archive := range archives.Archives {
				_, ok := tc.result.Archives[version]
				assert.Equal(true, ok)
				assert.Equal(tc.result.Archives[version].Url, archive.Url)
			}
		})
	}
}
