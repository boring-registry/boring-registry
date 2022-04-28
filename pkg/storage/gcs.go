package storage

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
	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"
)

// GCSStorage implements provider.Storage
type GCSStorage struct {
	sc              *storage.Client
	bucket          string
	bucketPrefix    string
	useSignedURL    bool
	signedURLExpiry time.Duration
	serviceAccount  string
}

// GetProvider implements provider.Storage
func (s *GCSStorage) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (core.Provider, error) {
	archivePath, shasumPath, shasumSigPath, err := internalProviderPath(s.bucketPrefix, namespace, name, version, os, arch)
	if err != nil {
		return core.Provider{}, err
	}

	pathSigningKeys := signingKeysPath(s.bucketPrefix, namespace)

	zipURL, err := s.generateURL(ctx, archivePath)
	if err != nil {
		return core.Provider{}, err
	}
	shasumsURL, err := s.generateURL(ctx, shasumPath)
	if err != nil {
		return core.Provider{}, errors.Wrap(err, shasumPath)
	}
	signatureURL, err := s.generateURL(ctx, shasumSigPath)
	if err != nil {
		return core.Provider{}, err
	}

	signingKeysRaw, err := s.download(ctx, pathSigningKeys)
	if err != nil {
		return core.Provider{}, errors.Wrap(err, pathSigningKeys)
	}

	var signingKey core.GPGPublicKey
	if err := json.Unmarshal(signingKeysRaw, &signingKey); err != nil {
		return core.Provider{}, err
	}

	shasumBytes, err := s.download(ctx, shasumPath)
	if err != nil {
		return core.Provider{}, err
	}

	shasum, err := readSHASums(bytes.NewReader(shasumBytes), path.Base(archivePath))
	if err != nil {
		return core.Provider{}, err
	}

	return core.Provider{
		Namespace:           namespace,
		Filename:            path.Base(archivePath),
		Name:                name,
		Version:             version,
		OS:                  os,
		Arch:                arch,
		Shasum:              shasum,
		DownloadURL:         zipURL,
		SHASumsURL:          shasumsURL,
		SHASumsSignatureURL: signatureURL,
		SigningKeys: core.SigningKeys{
			GPGPublicKeys: []core.GPGPublicKey{
				signingKey,
			},
		},
	}, nil
}

func (s *GCSStorage) ListProviderVersions(ctx context.Context, namespace, name string) ([]core.ProviderVersion, error) {
	prefix, err := providerStoragePrefix(s.bucketPrefix, internalProviderType, "", namespace, name)
	if err != nil {
		return nil, err
	}

	query := &storage.Query{
		Prefix: fmt.Sprintf("%s/", prefix),
	}

	collection := NewCollection()
	it := s.sc.Bucket(s.bucket).Objects(ctx, query)

	for {
		select { // Check if the context has been canceled in every loop iteration
		case <-ctx.Done():
			return nil, ctx.Err()
		default: // break out of the select statement by not doing anything
		}

		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		provider, err := core.NewProviderFromArchive(attrs.Name)
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

func (s *GCSStorage) generateAPIURL(key string) (string, error) {
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", s.bucket, key), nil
}

func (s *GCSStorage) download(ctx context.Context, path string) ([]byte, error) {
	r, err := s.sc.Bucket(s.bucket).Object(path).NewReader(ctx)
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

func (s *GCSStorage) generateURL(ctx context.Context, v string) (string, error) {
	if s.useSignedURL {
		return s.signedURL(ctx, v)
	}

	return s.generateAPIURL(v)
}

func (s *GCSStorage) signedURL(ctx context.Context, v string) (string, error) {
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
			Expires:        time.Now().Add(s.signedURLExpiry * time.Second),
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
		if err != nil {
			return "", errors.Wrap(err, "could not get jwt config")
		}
		opts := &storage.SignedURLOptions{
			Scheme:         storage.SigningSchemeV4,
			Method:         "GET",
			GoogleAccessID: conf.Email,
			PrivateKey:     conf.PrivateKey,
			Expires:        time.Now().Add(s.signedURLExpiry * time.Second),
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

// WithGCSSignedUrlExpiry configures the duration until the signed url expires
func WithGCSSignedUrlExpiry(t time.Duration) GCSStorageOption {
	return func(s *GCSStorage) {
		s.signedURLExpiry = t
	}
}

func WithGCSUseSignedURL(b bool) GCSStorageOption {
	return func(s *GCSStorage) {
		s.useSignedURL = b
	}
}

func NewGCSStorage(bucket string, options ...GCSStorageOption) (*GCSStorage, error) {
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
