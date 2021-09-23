package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"time"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"
)

type GCSStorage struct {
	sc              *storage.Client
	bucket          string
	bucketPrefix    string
	signedURLExpiry int64
	serviceAccount  string
}

func (s *GCSStorage) ListProviderVersions(ctx context.Context, namespace, name string) ([]ProviderVersion, error) {
	prefix := storagePrefix(s.bucketPrefix, namespace, name)

	query := &storage.Query{
		Prefix: fmt.Sprintf("%s/", prefix),
	}

	collection := NewCollection()
	it := s.sc.Bucket(s.bucket).Objects(ctx, query)

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		provider, err := Parse(attrs.Name)
		if err != nil {
			continue
		}

		collection.Add(provider)
	}

	result := collection.List()

	if len(result) == 0 {
		return nil, fmt.Errorf("no provider versions found for %s/%s", namespace, name)
	}

	return result, nil
}

func (s *GCSStorage) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (Provider, error) {
	var (
		pathPkg         = storagePath(s.bucketPrefix, namespace, name, version, os, arch)
		pathSha         = shasumsPath(s.bucketPrefix, namespace, name, version)
		pathSig         = fmt.Sprintf("%s.sig", pathSha)
		pathSigningKeys = signingKeysPath(s.bucketPrefix, namespace)
	)

	shasumsURL, err := s.signedURL(pathSha)
	if err != nil {
		return Provider{}, errors.Wrap(err, pathSig)
	}

	signatureURL, err := s.signedURL(pathSig)
	if err != nil {
		return Provider{}, err
	}

	zipURL, err := s.signedURL(pathPkg)
	if err != nil {
		return Provider{}, err
	}

	signingKeysRaw, err := s.download(pathSigningKeys)
	if err != nil {
		return Provider{}, errors.Wrap(err, pathSigningKeys)
	}

	var signingKey GPGPublicKey
	if err := json.Unmarshal(signingKeysRaw, &signingKey); err != nil {
		return Provider{}, err
	}

	shasums, err := s.download(pathSha)
	if err != nil {
		return Provider{}, err
	}

	shasum, err := readSHASums(bytes.NewReader(shasums), path.Base(pathPkg))
	if err != nil {
		return Provider{}, err
	}

	return Provider{
		Namespace:           namespace,
		Filename:            path.Base(pathPkg),
		Name:                name,
		Version:             version,
		OS:                  os,
		Arch:                arch,
		Shasum:              shasum,
		DownloadURL:         zipURL,
		SHASumsURL:          shasumsURL,
		SHASumsSignatureURL: signatureURL,
		SigningKeys: SigningKeys{
			GPGPublicKeys: []GPGPublicKey{
				signingKey,
			},
		},
	}, nil
}

func (s *GCSStorage) download(path string) ([]byte, error) {
	r, err := s.sc.Bucket(s.bucket).Object(path).NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	defer r.Close()

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *GCSStorage) signedURL(v string) (string, error) {
	ctx := context.Background()
	//https://godoc.org/golang.org/x/oauth2/google#DefaultClient
	cred, err := google.FindDefaultCredentials(ctx, "cloud-platform")
	if err != nil {
		return "", fmt.Errorf("google.FindDefaultCredentials: %v", err)
	}

	var url string
	if s.serviceAccount != "" {
		// needs Service Account Token Creator role
		c, err := credentials.NewIamCredentialsClient(ctx)
		if err != nil {
			return "", fmt.Errorf("credentials.NewIamCredentialsClient: %v", err)
		}

		url, err = storage.SignedURL(s.bucket, v, &storage.SignedURLOptions{
			Scheme:         storage.SigningSchemeV4,
			Method:         "GET",
			GoogleAccessID: s.serviceAccount,
			Expires:        time.Now().Add(time.Duration(s.signedURLExpiry) * time.Second),
			SignBytes: func(b []byte) ([]byte, error) {
				req := &credentialspb.SignBlobRequest{
					Payload: b,
					Name:    s.serviceAccount,
				}
				resp, err := c.SignBlob(ctx, req)
				if err != nil {
					return nil, fmt.Errorf("storage.signedURL.SignBytes: %v", err)
				}
				return resp.SignedBlob, nil
			},
		})
		if err != nil {
			return "", fmt.Errorf("storage.signedURL: %v", err)
		}
	} else {
		conf, err := google.JWTConfigFromJSON(cred.JSON)
		opts := &storage.SignedURLOptions{
			Scheme:         storage.SigningSchemeV4,
			Method:         "GET",
			GoogleAccessID: conf.Email,
			PrivateKey:     conf.PrivateKey,
			Expires:        time.Now().Add(time.Duration(s.signedURLExpiry) * time.Second),
		}
		url, err = storage.SignedURL(s.bucket, v, opts)
		if err != nil {
			return "", fmt.Errorf("storage.signedURL: %v", err)
		}
	}

	return url, nil
}

// GCSStorageOption provides additional options for the GCSStorage.
type GCSStorageOption func(*GCSStorage)

// WithGCSStorageBucketPrefix configures the s3 storage to work under a given prefix.
func WithGCSStorageBucketPrefix(prefix string) GCSStorageOption {
	return func(s *GCSStorage) {
		s.bucketPrefix = prefix
	}
}

// WithGCSServiceAccount configures Application Default Credentials (ADC) service account email.
func WithGCSServiceAccount(sa string) GCSStorageOption {
	return func(s *GCSStorage) {
		s.serviceAccount = sa
	}
}

// WithGCSServiceAccount configures Application Default Credentials (ADC) service account email.
func WithGCSSignedUrlExpiry(seconds int64) GCSStorageOption {
	return func(s *GCSStorage) {
		s.signedURLExpiry = seconds
	}
}

func NewGCSStorage(bucket string, options ...GCSStorageOption) (Storage, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	s := &GCSStorage{
		sc:     client,
		bucket: bucket,
	}

	for _, option := range options {
		option(s)
	}

	return s, nil
}
