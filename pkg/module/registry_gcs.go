package module

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"io"
	"log"
	"os"
	"strconv"
	//"golang.org/x/oauth2/google"
	//"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"
)

// S3Registry is a Registry implementation backed by S3.
type GCSRegistry struct {
	sc     *storage.Client
	bucket string
}

func (s *GCSRegistry) GetModule(ctx context.Context, namespace, name, provider, version string) (Module, error) {
	o := s.sc.Bucket(s.bucket).Object(fmt.Sprintf("namespace=%[1]v/name=%[2]v/provider=%[3]v/version=%[4]v/%[1]v-%[2]v-%[3]v-%[4]v.tar.gz", namespace, name, provider, version))
	attrs, err := o.Attrs(ctx)
	if err != nil {
		return Module{}, errors.Wrap(ErrNotFound, err.Error())
	}
	return Module{
		Namespace:   namespace,
		Name:        attrs.Name,
		Provider:    provider,
		Version:     strconv.FormatInt(attrs.Generation, 10),
		DownloadURL: attrs.MediaLink,
	}, nil
}

func (s *GCSRegistry) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]Module, error) {
	var modules []Module

	query := &storage.Query{
		Prefix: fmt.Sprintf("namespace=%s/name=%s/provider=%s", namespace, name, provider),
		//Delimiter: "/",
	}
	it := s.sc.Bucket(s.bucket).Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		module := Module{
			Namespace:   namespace,
			Name:        attrs.Name,
			Provider:    provider,
			Version:     strconv.FormatInt(attrs.Generation, 10),
			DownloadURL: attrs.MediaLink,
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

	key := fmt.Sprintf("namespace=%[1]v/name=%[2]v/provider=%[3]v/version=%[4]v/%[1]v-%[2]v-%[3]v-%[4]v.tar.gz", namespace, name, provider, version)

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

func NewGCSRegistry(bucket string, options ...S3RegistryOption) (Registry, error) {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		fmt.Fprintf(os.Stderr, "GOOGLE_CLOUD_PROJECT environment variable must be set.\n")
		os.Exit(1)
	}
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	s := &GCSRegistry{
		sc:     client,
		bucket: bucket,
	}
	return s, nil
}
