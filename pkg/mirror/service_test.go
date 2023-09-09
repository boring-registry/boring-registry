package mirror

import (
	"context"
	"errors"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
	"io"
	"net/url"
	"reflect"
	"testing"
	"time"
)

type mockedUpstreamProvider struct {
	customListProviderVersions func(ctx context.Context, provider *core.Provider) (*listResponse, error)
	customGetProvider          func(ctx context.Context, provider *core.Provider) (*core.Provider, error)
	customShaSums              func(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error)
}

func (m *mockedUpstreamProvider) listProviderVersions(ctx context.Context, provider *core.Provider) (*listResponse, error) {
	return m.customListProviderVersions(ctx, provider)
}

func (m *mockedUpstreamProvider) getProvider(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
	return m.customGetProvider(ctx, provider)
}

func (m *mockedUpstreamProvider) shaSums(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error) {
	return m.customShaSums(ctx, provider)
}

type mockedStorage struct {
	listMirrorProviderVersions func(ctx context.Context, provider *core.Provider) ([]core.ProviderVersion, error)
	getMirroredProvider        func(ctx context.Context, provider *core.Provider) (*core.Provider, error)
	mirroredSha256Sum          func(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error)
	uploadMirroredFile         func(ctx context.Context, provider *core.Provider, filename string, reader io.Reader) error
	mirroredSigningKeys        func(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error)
	uploadMirroredSigningKeys  func(ctx context.Context, hostname, namespace string, signingKeys *core.SigningKeys) error
}

func (m *mockedStorage) ListMirroredProviderVersions(ctx context.Context, provider *core.Provider) ([]core.ProviderVersion, error) {
	return m.listMirrorProviderVersions(ctx, provider)
}

func (m *mockedStorage) GetMirroredProvider(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
	return m.getMirroredProvider(ctx, provider)
}

func (m *mockedStorage) UploadMirroredFile(ctx context.Context, provider *core.Provider, fileName string, reader io.Reader) error {
	return m.uploadMirroredFile(ctx, provider, fileName, reader)
}

func (m *mockedStorage) MirroredSigningKeys(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error) {
	return m.mirroredSigningKeys(ctx, hostname, namespace)
}

func (m *mockedStorage) UploadMirroredSigningKeys(ctx context.Context, hostname, namespace string, signingKeys *core.SigningKeys) error {
	return m.uploadMirroredSigningKeys(ctx, hostname, namespace, signingKeys)
}

func (m *mockedStorage) MirroredSha256Sum(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error) {
	return m.mirroredSha256Sum(ctx, provider)
}

func Test_service_ListProviderVersions(t *testing.T) {
	type args struct {
		ctx      context.Context
		provider *core.Provider
	}
	tests := []struct {
		name    string
		svc     Service
		args    args
		want    *ProviderVersions
		wantErr bool
	}{
		{
			name: "expired context",
			svc: &service{
				upstream: &mockedUpstreamProvider{
					customListProviderVersions: func(ctx context.Context, provider *core.Provider) (*listResponse, error) {
						<-ctx.Done()
						return nil, ctx.Err()
					},
				},
			},
			args: func() args {
				// The cancel function has to be ignored, as we cannot easily cancel it
				ctx, _ := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				return args{
					ctx: ctx,
					provider: &core.Provider{
						Namespace: "hashicorp",
						Name:      "random",
					},
				}
			}(),
			wantErr: true,
		},
		{
			name: "failed to retrieve from upstream and from mirror",
			svc: &service{
				upstream: &mockedUpstreamProvider{
					customListProviderVersions: func(ctx context.Context, provider *core.Provider) (*listResponse, error) {
						// mock url.Error from client to upstream to simulate unavailable upstream
						return nil, &url.Error{}
					},
				},
				storage: &mockedStorage{
					listMirrorProviderVersions: func(ctx context.Context, provider *core.Provider) ([]core.ProviderVersion, error) {
						// return core.ProviderError to simulate that providers are not in the mirror
						return nil, &core.ProviderError{
							Reason: "mocked provider error",
						}
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid upstream response",
			svc: &service{
				upstream: &mockedUpstreamProvider{
					customListProviderVersions: func(ctx context.Context, provider *core.Provider) (*listResponse, error) {
						return &listResponse{
							Versions: []listResponseVersion{
								{
									Version: "0.1.2",
									Platforms: []core.Platform{
										{
											OS:   "linux",
											Arch: "amd64",
										},
									},
								},
							},
						}, nil
					},
				},
			},
			want: &ProviderVersions{
				Versions: map[string]EmptyObject{
					"0.1.2": {},
				},
			},
		},
		{
			name: "upstream unavailable, response from mirror",
			svc: &service{
				upstream: &mockedUpstreamProvider{
					customListProviderVersions: func(ctx context.Context, provider *core.Provider) (*listResponse, error) {
						// mock url.Error from client to upstream to simulate unavailable upstream
						return nil, &url.Error{}
					},
				},
				storage: &mockedStorage{
					listMirrorProviderVersions: func(ctx context.Context, provider *core.Provider) ([]core.ProviderVersion, error) {
						return []core.ProviderVersion{
							{
								Namespace: "hashicorp",
								Name:      "random",
								Version:   "0.1.2",
								Platforms: []core.Platform{
									{
										OS:   "linux",
										Arch: "amd64",
									},
									{
										OS:   "linux",
										Arch: "arm64",
									},
								},
							},
							{
								Namespace: "hashicorp",
								Name:      "random",
								Version:   "1.2.3",
							},
						}, nil
					},
				},
			},
			want: &ProviderVersions{
				Versions: map[string]EmptyObject{
					"0.1.2": {},
					"1.2.3": {},
				},
				fromMirror: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.args.ctx != nil {
				ctx = tt.args.ctx
			}

			got, err := tt.svc.ListProviderVersions(ctx, tt.args.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListProviderVersions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListProviderVersions() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_service_ListProviderInstallation(t *testing.T) {
	type args struct {
		ctx      context.Context
		provider *core.Provider
	}
	tests := []struct {
		name    string
		svc     Service
		args    args
		want    *Archives
		wantErr bool
	}{
		{
			name: "failed to retrieve from upstream and from mirror",
			svc: &service{
				upstream: &mockedUpstreamProvider{
					customListProviderVersions: func(ctx context.Context, provider *core.Provider) (*listResponse, error) {
						return nil, &url.Error{}
					},
				},
				storage: &mockedStorage{
					listMirrorProviderVersions: func(ctx context.Context, provider *core.Provider) ([]core.ProviderVersion, error) {
						return nil, errors.New("mocked error")
					},
				},
			},
			args: args{
				ctx: context.Background(),
				provider: &core.Provider{
					Hostname:  "registry.example.com",
					Namespace: "hashicorp",
					Name:      "random",
					Version:   "0.1.2",
				},
			},
			wantErr: true,
		},
		{
			name: "successfully retrieve response from upstream",
			svc: &service{
				upstream: &mockedUpstreamProvider{
					customListProviderVersions: func(ctx context.Context, provider *core.Provider) (*listResponse, error) {
						return &listResponse{
							Versions: []listResponseVersion{
								{
									Version: "0.1.0",
									Platforms: []core.Platform{
										{
											OS:   "linux",
											Arch: "amd64",
										},
										{
											OS:   "linux",
											Arch: "arm64",
										},
									},
								},
								{
									Version: "0.1.2",
									Platforms: []core.Platform{
										{
											OS:   "linux",
											Arch: "amd64",
										},
									},
								},
							},
						}, nil
					},
					customGetProvider: func(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
						return &core.Provider{
							Hostname:   "registry.example.com",
							Namespace:  "hashicorp",
							Name:       "random",
							Version:    "0.1.0",
							SHASumsURL: "https://registry.example.com/this/is/the/shasums/file",
						}, nil
					},
					customShaSums: func(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error) {
						return &core.Sha256Sums{
							Entries: map[string][]byte{
								"terraform-provider-random_0.1.0_linux_amd64.zip": []byte("123456789"),
								"terraform-provider-random_0.1.0_linux_arm64.zip": []byte("987654321"),
							},
						}, nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				provider: &core.Provider{
					Hostname:  "registry.example.com",
					Namespace: "hashicorp",
					Name:      "random",
					Version:   "0.1.0",
				},
			},
			want: &Archives{
				Archives: map[string]Archive{
					"linux_amd64": {
						Url:    "terraform-provider-random_0.1.0_linux_amd64.zip",
						Hashes: []string{fmt.Sprintf("zh:%x", []byte("123456789"))},
					},
					"linux_arm64": {
						Url:    "terraform-provider-random_0.1.0_linux_arm64.zip",
						Hashes: []string{fmt.Sprintf("zh:%x", []byte("987654321"))},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "upstream fails but mirror succeeds",
			svc: &service{
				upstream: &mockedUpstreamProvider{
					customListProviderVersions: func(ctx context.Context, provider *core.Provider) (*listResponse, error) {
						return nil, &url.Error{}
					},
				},
				storage: &mockedStorage{
					listMirrorProviderVersions: func(ctx context.Context, provider *core.Provider) ([]core.ProviderVersion, error) {
						return []core.ProviderVersion{
							{
								Namespace: "hashicorp",
								Name:      "random",
								Version:   "1.2.3",
								Platforms: []core.Platform{
									{
										OS:   "linux",
										Arch: "amd64",
									},
									{
										OS:   "linux",
										Arch: "arm64",
									},
								},
							},
						}, nil
					},
					getMirroredProvider: func(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
						return &core.Provider{
							Hostname:   "registry.example.com",
							Namespace:  "hashicorp",
							Name:       "random",
							Version:    "1.2.3",
							OS:         "linux",
							Arch:       "amd64",
							SHASumsURL: "https://registry.example.com/this/is/the/shasums/file",
						}, nil
					},
					mirroredSha256Sum: func(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error) {
						return &core.Sha256Sums{
							Entries: map[string][]byte{
								"terraform-provider-random_1.2.3_linux_amd64.zip": []byte("123456789"),
								"terraform-provider-random_1.2.3_linux_arm64.zip": []byte("987654321"),
							},
						}, nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				provider: &core.Provider{
					Hostname:  "registry.example.com",
					Namespace: "hashicorp",
					Name:      "random",
					Version:   "1.2.3",
				},
			},
			want: &Archives{
				Archives: map[string]Archive{
					"linux_amd64": {
						Url:    "terraform-provider-random_1.2.3_linux_amd64.zip",
						Hashes: []string{fmt.Sprintf("zh:%x", []byte("123456789"))},
					},
					"linux_arm64": {
						Url:    "terraform-provider-random_1.2.3_linux_arm64.zip",
						Hashes: []string{fmt.Sprintf("zh:%x", []byte("987654321"))},
					},
				},
				fromMirror: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.args.ctx != nil {
				ctx = tt.args.ctx
			}

			got, err := tt.svc.ListProviderInstallation(ctx, tt.args.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListProviderInstallation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListProviderInstallation() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_service_RetrieveProviderArchive(t *testing.T) {
	type args struct {
		ctx      context.Context
		provider *core.Provider
	}
	tests := []struct {
		name    string
		svc     service
		args    args
		want    *retrieveProviderArchiveResponse
		wantErr bool
	}{
		{
			name: "provider exists in the mirror",
			svc: service{
				//upstreamProvider: &mockedUpstreamProvider{},
				storage: &mockedStorage{
					getMirroredProvider: func(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
						provider.DownloadURL = "terraform-provider-random_2.0.0_linux_amd64.zip"
						return provider, nil
					},
				},
			},
			args: args{
				provider: &core.Provider{
					Hostname: "terraform.example.com",
					Name:     "random",
					Version:  "2.0.0",
					OS:       "linux",
					Arch:     "amd64",
				},
			},
			want: &retrieveProviderArchiveResponse{
				location:     "terraform-provider-random_2.0.0_linux_amd64.zip",
				mirrorSource: mirrorSource{isMirror: true},
			},
		},
		{
			name: "a non-core.ProviderError happened while looking up the provider in the mirror",
			svc: service{
				//upstreamProvider: &mockedUpstreamProvider{},
				storage: &mockedStorage{
					getMirroredProvider: func(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
						return &core.Provider{}, errors.New("mocked error")
					},
				},
			},
			wantErr: true,
		},
		//{
		//	name: "the provider is not in the mirror",
		//},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.args.ctx != nil {
				ctx = tt.args.ctx
			}

			got, err := tt.svc.RetrieveProviderArchive(ctx, tt.args.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetrieveProviderArchive() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RetrieveProviderArchive() got = %v, want %v", got, tt.want)
			}
		})
	}
}
