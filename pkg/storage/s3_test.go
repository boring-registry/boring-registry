package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/TierMobility/boring-registry/pkg/core"

	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	assertion "github.com/stretchr/testify/assert"
)

type mockS3Client struct {
	headObject func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return m.headObject(ctx, params, optFns...)
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, f ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	panic("not yet implemented, as we don't have tests using it")
}

func (m *mockS3Client) CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	panic("not yet implemented, as we don't have tests using it")
}

type mockS3Uploader struct {
	b   *bytes.Buffer
	err error
}

func (m *mockS3Uploader) Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	m.b = new(bytes.Buffer)
	if _, err := io.Copy(m.b, input.Body); err != nil {
		return nil, err
	}

	return nil, m.err
}

type mockS3Downloader struct {
	// data is a map that contains data which should be served under a given key
	data  map[string][]byte
	error bool
}

// Not 100% sure if that works correctly for large byte arrays
// Implements the s3DownloaderAPI interface
func (m *mockS3Downloader) Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(api *s3manager.Downloader)) (n int64, err error) {
	if m.error {
		return 0, errors.New("mocked error")
	}

	data, exists := m.data[*input.Key]
	if !exists {
		panic(fmt.Sprintf("key %s does not exist in mocked payload map", *input.Key))
	}

	var off int64 = 0
	for {
		written, err := w.WriteAt(data, off)
		if err != nil {
			return 0, err
		}
		off += int64(written)
		if off == int64(len(m.data[*input.Key])) {
			break
		}
	}
	return 0, nil
}

type mockS3PresignClient struct{}

func (m *mockS3PresignClient) PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*signer.PresignedHTTPRequest, error) {
	return &signer.PresignedHTTPRequest{
		URL: fmt.Sprintf("%s?presigned=true", *params.Key),
	}, nil
}

func headExistingObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return &s3.HeadObjectOutput{}, nil
}

func headNonExistingObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return nil, &awshttp.ResponseError{
		ResponseError: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{
				Response: &http.Response{
					StatusCode: http.StatusNotFound,
				},
			},
		},
	}
}

func TestS3Storage_UploadProviderReleaseFiles(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		namespace   string
		name        string
		filename    string
		content     string
		client      s3ClientAPI
		wantErr     assertion.ErrorAssertionFunc
	}{
		{
			description: "provider file exists already",
			namespace:   "hashicorp",
			name:        "random",
			filename:    "terraform-provider-random_2.0.0_linux_amd64.zip",
			client: &mockS3Client{
				headObject: headExistingObject,
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
			client: &mockS3Client{
				headObject: headNonExistingObject,
			},
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return !assertion.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			u := &mockS3Uploader{}
			s := S3Storage{
				client:   tc.client,
				uploader: u,
			}
			s.uploader = u
			err := s.UploadProviderReleaseFiles(context.Background(), tc.namespace, tc.name, tc.filename, strings.NewReader(tc.content))
			if tc.wantErr(t, err) {
				return
			}

			assertion.Equal(t, tc.content, u.b.String())
		})
	}
}

func TestSigningKeys(t *testing.T) {
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
			annotation: "empty namespace",
			data: map[string][]byte{
				"providers/hashicorp/signing-keys.json": validSigningKeysBytes,
			},
			namespace:     "",
			expectedError: true,
			expect:        validSigningKeys,
		},
		{
			annotation: "download fails",
			data: map[string][]byte{
				"providers/hashicorp/signing-keys.json": validSigningKeysBytes,
			},
			namespace:     "hashicorp",
			returnError:   true,
			expectedError: true,
			expect:        validSigningKeys,
		},
		{
			annotation: "empty object",
			data: map[string][]byte{
				"providers/hashicorp/signing-keys.json": []byte(""),
			},
			namespace:     "hashicorp",
			expectedError: true,
			expect:        validSigningKeys,
		},
		{
			annotation: "only single gpg_public_key for the provider namespace",
			data: map[string][]byte{
				"providers/hashicorp/signing-keys.json": validGPGPublicKeyBytes,
			},
			namespace:     "hashicorp",
			expectedError: false,
			expect:        validSigningKeys,
		},
		{
			annotation: "signing_keys with a single gpg_public_key",
			data: map[string][]byte{
				"providers/hashicorp/signing-keys.json": validSigningKeysBytes,
			},
			namespace:     "hashicorp",
			expectedError: false,
			expect:        validSigningKeys,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.annotation, func(t *testing.T) {
			s := S3Storage{
				downloader: &mockS3Downloader{data: tc.data, error: tc.returnError},
				client: &mockS3Client{
					headObject: headExistingObject,
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

func TestS3Storage_getProvider(t *testing.T) {
	type fields struct {
		client     s3ClientAPI
		downloader s3DownloaderAPI
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
				client: &mockS3Client{
					headObject: headNonExistingObject,
				},
				downloader: &mockS3Downloader{},
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
				client: &mockS3Client{
					headObject: headExistingObject,
				},
				downloader: &mockS3Downloader{
					data: map[string][]byte{
						"providers/example/dummy/terraform-provider-dummy_1.0.0_SHA256SUMS": []byte("10488a12525ed674359585f83e3ee5e74818b5c98e033798351678b21b2f7d89  terraform-provider-dummy_1.0.0_linux_amd64.zip"),
						"providers/example/signing-keys.json":                               []byte(`{"gpg_public_keys":[{"key_id":"47422B4AA9FA381B","ascii_armor":"test"}]}`),
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
				DownloadURL:         "providers/example/dummy/terraform-provider-dummy_1.0.0_linux_amd64.zip?presigned=true",
				Shasum:              "10488a12525ed674359585f83e3ee5e74818b5c98e033798351678b21b2f7d89",
				SHASumsURL:          "providers/example/dummy/terraform-provider-dummy_1.0.0_SHA256SUMS?presigned=true",
				SHASumsSignatureURL: "providers/example/dummy/terraform-provider-dummy_1.0.0_SHA256SUMS.sig?presigned=true",
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
				client: &mockS3Client{
					headObject: headExistingObject,
				},
				downloader: &mockS3Downloader{
					data: map[string][]byte{
						"mirror/providers/terraform.example.com/example/dummy/terraform-provider-dummy_1.0.0_SHA256SUMS": []byte("10488a12525ed674359585f83e3ee5e74818b5c98e033798351678b21b2f7d89  terraform-provider-dummy_1.0.0_linux_amd64.zip"),
						"mirror/providers/terraform.example.com/example/signing-keys.json":                               []byte(`{"gpg_public_keys":[{"key_id":"47422B4AA9FA381B","ascii_armor":"test"}]}`),
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
				DownloadURL:         "mirror/providers/terraform.example.com/example/dummy/terraform-provider-dummy_1.0.0_linux_amd64.zip?presigned=true",
				Shasum:              "10488a12525ed674359585f83e3ee5e74818b5c98e033798351678b21b2f7d89",
				SHASumsURL:          "mirror/providers/terraform.example.com/example/dummy/terraform-provider-dummy_1.0.0_SHA256SUMS?presigned=true",
				SHASumsSignatureURL: "mirror/providers/terraform.example.com/example/dummy/terraform-provider-dummy_1.0.0_SHA256SUMS.sig?presigned=true",
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
			s := &S3Storage{
				client:        tt.fields.client,
				presignClient: &mockS3PresignClient{},
				downloader:    tt.fields.downloader,
			}
			got, err := s.getProvider(context.Background(), tt.args.pt, tt.args.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("S3Storage.getProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("S3Storage.getProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}
