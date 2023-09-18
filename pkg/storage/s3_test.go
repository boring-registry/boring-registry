package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/TierMobility/boring-registry/pkg/core"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	assertion "github.com/stretchr/testify/assert"
)

type mockS3Client struct {
	errorFunc func() error
}

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return nil, m.errorFunc()
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
	payload []byte
	error   bool
}

// Not 100% sure if that works correctly for large byte arrays
// Implements the s3DownloaderAPI interface
func (m *mockS3Downloader) Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(api *s3manager.Downloader)) (n int64, err error) {
	if m.error {
		return 0, errors.New("mocked error")
	}

	var off int64 = 0
	for {
		written, err := w.WriteAt(m.payload, off)
		if err != nil {
			return 0, err
		}
		off += int64(written)
		if off == int64(len(m.payload)) {
			break
		}
	}
	return 0, nil
}

func TestS3Storage_UploadProviderReleaseFiles(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description       string
		namespace         string
		name              string
		filename          string
		content           string
		s3ClientErrorFunc func() error
		wantErr           assert.ErrorAssertionFunc
	}{
		{
			description: "provider file exists already",
			namespace:   "hashicorp",
			name:        "random",
			filename:    "terraform-provider-random_2.0.0_linux_amd64.zip",
			s3ClientErrorFunc: func() error {
				return nil
			},
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.Error(t, err)
			},
		},
		{
			description: "upload file",
			namespace:   "hashicorp",
			name:        "random",
			filename:    "terraform-provider-random_2.0.0_linux_amd64.zip",
			content:     "test",
			s3ClientErrorFunc: func() error {
				return &awshttp.ResponseError{
					ResponseError: &smithyhttp.ResponseError{
						Response: &smithyhttp.Response{
							Response: &http.Response{
								StatusCode: http.StatusNotFound,
							},
						},
					},
				}
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
				client: &mockS3Client{
					errorFunc: tc.s3ClientErrorFunc,
				},
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
		payload       []byte
		namespace     string
		returnError   bool
		expectedError bool
		expect        core.SigningKeys
	}{
		{
			annotation:    "empty namespace",
			payload:       validSigningKeysBytes,
			namespace:     "",
			expectedError: true,
			expect:        validSigningKeys,
		},
		{
			annotation:    "download fails",
			payload:       validSigningKeysBytes,
			namespace:     "hashicorp",
			returnError:   true,
			expectedError: true,
			expect:        validSigningKeys,
		},
		{
			annotation:    "empty object",
			payload:       []byte(""),
			namespace:     "hashicorp",
			expectedError: true,
			expect:        validSigningKeys,
		},
		{
			annotation:    "only a single gpg_public_key for the provider namespace",
			payload:       validGPGPublicKeyBytes,
			namespace:     "hashicorp",
			expectedError: false,
			expect:        validSigningKeys,
		},
		{
			annotation:    "signing_keys with a single gpg_public_key",
			payload:       validSigningKeysBytes,
			namespace:     "hashicorp",
			expectedError: false,
			expect:        validSigningKeys,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.annotation, func(t *testing.T) {
			s := S3Storage{
				downloader: &mockS3Downloader{payload: tc.payload, error: tc.returnError},
				client: &mockS3Client{
					errorFunc: func() error {
						return nil
					},
				},
			}

			result, err := s.SigningKeys(context.Background(), tc.namespace)

			if !tc.expectedError {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, &tc.expect, result)
		})
	}
}
