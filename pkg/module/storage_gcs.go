package module

import (
	"context"
	"fmt"
	"io"
	"time"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"
)

// GCSStorage is a Storage implementation backed by Google Cloud Storage.
type GCSStorage struct {
	sc              *storage.Client
	bucket          string
	bucketPrefix    string
	signedURL       bool
	signedURLExpiry int64
	serviceAccount  string
}

func (s *GCSStorage) GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error) {
	o := s.sc.Bucket(s.bucket).Object(moduleObjectKey(namespace, name, provider, version, s.bucketPrefix))
	attrs, err := o.Attrs(ctx)
	if err != nil {
		return Module{}, errors.Wrap(ErrNotFound, err.Error())
	}
	var url string
	if s.signedURL {
		url, err = s.generateV4GetObjectSignedURL(attrs.Bucket, attrs.Name)
	} else {
		url, err = s.generateDownloadURL(attrs.Bucket, attrs.Name)
	}
	if err != nil {
		return Module{}, errors.Wrap(ErrNotFound, err.Error())
	}
	return Module{
		Namespace: namespace,
		Name:      attrs.Name,
		Provider:  provider,
		Version:   version,
		/* https://www.terraform.io/docs/internals/module-registry-protocol.html#sample-response-1
		e.g. "gcs::https://www.googleapis.com/storage/v1/modules/foomodule.zip
		*/
		DownloadURL: url,
	}, nil
}

func (s *GCSStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error) {
	var modules []Module
	prefix := moduleObjectKeyBase(namespace, name, provider, s.bucketPrefix)

	query := &storage.Query{
		Prefix: prefix,
	}
	it := s.sc.Bucket(s.bucket).Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return modules, err
		}
		metadata := objectMetadata(attrs.Name)

		version, ok := metadata["version"]
		if !ok {
			continue
		}

		module := Module{
			Version: version,
		}

		modules = append(modules, module)
	}
	return modules, nil
}

func (s *GCSStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (Module, error) {
	if namespace == "" {
		return Module{}, errors.New("namespace not defined")
	}

	if name == "" {
		return Module{}, errors.New("name not defined")
	}

	if provider == "" {
		return Module{}, errors.New("provider not defined")
	}

	if version == "" {
		return Module{}, errors.New("version not defined")
	}

	key := moduleObjectKey(namespace, name, provider, version, s.bucketPrefix)
	if _, err := s.GetModule(ctx, namespace, name, provider, version); err == nil {
		return Module{}, errors.Wrap(ErrAlreadyExists, key)
	}

	wc := s.sc.Bucket(s.bucket).Object(key).NewWriter(ctx)
	if _, err := io.Copy(wc, body); err != nil {
		return Module{}, errors.Wrapf(ErrUploadFailed, err.Error())
	}
	if err := wc.Close(); err != nil {
		return Module{}, errors.Wrapf(ErrUploadFailed, err.Error())
	}

	return s.GetModule(ctx, namespace, name, provider, version)
}

// GCSStorageOption provides additional options for the GCSStorage.
type GCSStorageOption func(*GCSStorage)

// WithGCSStorageBucketPrefix configures the s3 storage to work under a given prefix.
func WithGCSStorageBucketPrefix(prefix string) GCSStorageOption {
	return func(s *GCSStorage) {
		s.bucketPrefix = prefix
	}
}

// WithGCSStorageSignedURL configures the s3 storage to work under a given prefix.
func WithGCSStorageSignedURL(set bool) GCSStorageOption {
	return func(s *GCSStorage) {
		s.signedURL = set
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

func (s *GCSStorage) generateDownloadURL(bucket, key string) (string, error) {
	return fmt.Sprintf("gcs::https://www.googleapis.com/storage/v1/%s/%s", bucket, key), nil
}

// https://github.com/GoogleCloudPlatform/golang-samples/blob/73d60a5de091dcdda5e4f753b594ef18eee67906/storage/objects/generate_v4_get_object_signed_url.go#L28
// generateV4GetObjectSignedURL generates object signed URL with GET method.
func (s *GCSStorage) generateV4GetObjectSignedURL(bucket, object string) (string, error) {
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

		url, err = storage.SignedURL(bucket, object, &storage.SignedURLOptions{
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
		url, err = storage.SignedURL(bucket, object, opts)
		if err != nil {
			return "", fmt.Errorf("storage.signedURL: %v", err)
		}
	}

	return url, nil
}
