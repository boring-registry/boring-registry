package module

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"io"
)

// GCSRegistry is a Registry implementation backed by Google Cloud Storage.
type GCSRegistry struct {
	sc           *storage.Client
	bucket       string
	bucketPrefix string
}

func (s *GCSRegistry) GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error) {
	o := s.sc.Bucket(s.bucket).Object(moduleObjectKey(namespace, name, provider, version, s.bucketPrefix))
	attrs, err := o.Attrs(ctx)
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
		DownloadURL: s.generateDownloadURL(attrs.Bucket, attrs.Name),
	}, nil
}

func (s *GCSRegistry) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error) {
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

func (s *GCSRegistry) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (Module, error) {
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

// GCSRegistryOption provides additional options for the S3Registry.
type GCSRegistryOption func(*GCSRegistry)

// WithS3RegistryBucketPrefix configures the s3 storage to work under a given prefix.
func WithGCSRegistryBucketPrefix(prefix string) GCSRegistryOption {
	return func(s *GCSRegistry) {
		s.bucketPrefix = prefix
	}
}

func NewGCSRegistry(bucket string, options ...GCSRegistryOption) (Registry, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	s := &GCSRegistry{
		sc:     client,
		bucket: bucket,
	}

	for _, option := range options {
		option(s)
	}

	return s, nil
}

// XXX: support presigned URLs?
func (s *GCSRegistry) generateDownloadURL(bucket, key string) string {
	return fmt.Sprintf("gcs::https://www.googleapis.com/storage/v1/%s/%s", bucket, key)
}
