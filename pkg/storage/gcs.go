package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"time"

	"github.com/TierMobility/boring-registry/pkg/core"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"cloud.google.com/go/storage"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
)

// GCSStorage is a Storage implementation backed by GCS.
// GCSStorage implements module.Storage and provider.Storage
type GCSStorage struct {
	sc                  *storage.Client
	bucket              string
	bucketPrefix        string
	signedURLExpiry     time.Duration
	serviceAccount      string
	moduleArchiveFormat string
}

func (s *GCSStorage) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	o := s.sc.Bucket(s.bucket).Object(modulePath(s.bucketPrefix, namespace, name, provider, version, s.moduleArchiveFormat))
	attrs, err := o.Attrs(ctx)
	if err != nil {
		return core.Module{}, errors.Wrap(ErrModuleNotFound, err.Error())
	}
	url, err := s.presignedURL(ctx, attrs.Name)
	if err != nil {
		return core.Module{}, errors.Wrap(ErrModuleNotFound, err.Error())
	}
	return core.Module{
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

func (s *GCSStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	prefix := modulePathPrefix(s.bucketPrefix, namespace, name, provider)

	query := &storage.Query{
		Prefix: prefix,
	}

	var modules []core.Module
	it := s.sc.Bucket(s.bucket).Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return modules, err
		}
		m, err := moduleFromObject(attrs.Name, s.moduleArchiveFormat)
		if err != nil {
			// TODO: we're skipping possible failures silently
			continue
		}
		modules = append(modules, *m)
	}
	return modules, nil
}

func (s *GCSStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
	if namespace == "" {
		return core.Module{}, errors.New("namespace not defined")
	}

	if name == "" {
		return core.Module{}, errors.New("name not defined")
	}

	if provider == "" {
		return core.Module{}, errors.New("provider not defined")
	}

	if version == "" {
		return core.Module{}, errors.New("version not defined")
	}

	key := modulePath(s.bucketPrefix, namespace, name, provider, version, DefaultModuleArchiveFormat)
	if _, err := s.GetModule(ctx, namespace, name, provider, version); err == nil {
		return core.Module{}, errors.Wrap(ErrModuleAlreadyExists, key)
	}

	wc := s.sc.Bucket(s.bucket).Object(key).NewWriter(ctx)
	if _, err := io.Copy(wc, body); err != nil {
		return core.Module{}, errors.Wrapf(ErrModuleUploadFailed, err.Error())
	}
	if err := wc.Close(); err != nil {
		return core.Module{}, errors.Wrapf(ErrModuleUploadFailed, err.Error())
	}

	return s.GetModule(ctx, namespace, name, provider, version)
}

func (s *GCSStorage) MigrateModules(ctx context.Context, logger log.Logger, dryRun bool) error {
	q := &storage.Query{
		Prefix: modulePathPrefix(s.bucketPrefix, "", "", ""),
	}
	it := s.sc.Bucket(s.bucket).Objects(ctx, q)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		} else if err != nil {
			return err
		}

		// Skip already migrated modules
		if !isUnmigratedModule(s.bucketPrefix, attrs.Name) {
			continue
		}

		targetKey := migrationTargetPath(s.bucketPrefix, s.moduleArchiveFormat, attrs.Name)
		if dryRun {
			_ = logger.Log("message", "skipping due to dry-run", "source", attrs.Name, "target", targetKey)
		} else {
			src := s.sc.Bucket(s.bucket).Object(attrs.Name)
			dst := s.sc.Bucket(s.bucket).Object(targetKey).If(storage.Conditions{DoesNotExist: true})

			if _, err = dst.CopierFrom(src).Run(ctx); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}

			_ = logger.Log("message", "copied module", "source", attrs.Name, "target", targetKey)
		}
	}

	return nil
}

// MigrateProviders is a temporary method needed for the migration from 0.7.0 to 0.8.0 and above
func (s *GCSStorage) MigrateProviders(ctx context.Context, logger log.Logger, dryRun bool) error {
	q := &storage.Query{
		Prefix: modulePathPrefix(s.bucketPrefix, "", "", ""),
	}
	it := s.sc.Bucket(s.bucket).Objects(ctx, q)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		} else if err != nil {
			return err
		}

		directory, err := providerMigrationTargetPath(s.bucketPrefix, attrs.Name)
		if err != nil {
			return err
		}

		targetKey := path.Join(directory, path.Base(attrs.Name))

		if dryRun {
			_ = logger.Log("message", "skipping due to dry-run", "source", attrs.Name, "target", targetKey)
		} else {
			src := s.sc.Bucket(s.bucket).Object(attrs.Name)
			dst := s.sc.Bucket(s.bucket).Object(targetKey).If(storage.Conditions{DoesNotExist: true})

			if _, err = dst.CopierFrom(src).Run(ctx); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}

			_ = logger.Log("message", "copied module", "source", attrs.Name, "target", targetKey)
		}
	}

	return nil
}

// GetProvider implements provider.Storage
func (s *GCSStorage) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (core.Provider, error) {
	archivePath, shasumPath, shasumSigPath, err := internalProviderPath(s.bucketPrefix, namespace, name, version, os, arch)
	if err != nil {
		return core.Provider{}, err
	}

	pathSigningKeys := signingKeysPath(s.bucketPrefix, namespace)

	zipURL, err := s.presignedURL(ctx, archivePath)
	if err != nil {
		return core.Provider{}, err
	}
	shasumsURL, err := s.presignedURL(ctx, shasumPath)
	if err != nil {
		return core.Provider{}, errors.Wrap(err, shasumPath)
	}
	signatureURL, err := s.presignedURL(ctx, shasumSigPath)
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

func (s *GCSStorage) UploadProviderReleaseFiles(ctx context.Context, namespace, name, filename string, file io.Reader) error {
	if namespace == "" {
		return fmt.Errorf("namespace argument is empty")
	}

	if name == "" {
		return fmt.Errorf("name argument is empty")
	}

	if filename == "" {
		return fmt.Errorf("name argument is empty")
	}

	prefix, err := providerStoragePrefix(s.bucketPrefix, internalProviderType, "", namespace, name)
	if err != nil {
		return err
	}

	key := filepath.Join(prefix, filename)
	exists, err := s.objectExists(ctx, key)
	if err != nil {
		return err
	} else if exists {
		return ErrProviderAlreadyExists
	}

	wc := s.sc.Bucket(s.bucket).Object(key).NewWriter(ctx)
	if _, err := io.Copy(wc, file); err != nil {
		return fmt.Errorf("failed to upload provider: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to upload provider: %w", err)
	}

	return nil
}

// SigningKeys downloads the JSON placed in the namespace in GCS and unmarshals it into a core.SigningKeys
func (s *GCSStorage) SigningKeys(ctx context.Context, namespace string) (*core.SigningKeys, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace argument is empty")
	}

	signingKeysRaw, err := s.download(ctx, signingKeysPath(s.bucketPrefix, namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to download signing_keys for namespace %s: %w", namespace, err)
	}

	return unmarshalSigningKeys(signingKeysRaw)
}

func (s *GCSStorage) download(ctx context.Context, path string) ([]byte, error) {
	r, err := s.sc.Bucket(s.bucket).Object(path).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer func(r *storage.Reader) {
		_ = r.Close()
	}(r)

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// https://github.com/GoogleCloudPlatform/golang-samples/blob/73d60a5de091dcdda5e4f753b594ef18eee67906/storage/objects/generate_v4_get_object_signed_url.go#L28
// presignedURL generates object signed URL with GET method.
func (s *GCSStorage) presignedURL(ctx context.Context, object string) (string, error) {
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

		url, err = storage.SignedURL(s.bucket, object, &storage.SignedURLOptions{
			Scheme:         storage.SigningSchemeV4,
			Method:         "GET",
			GoogleAccessID: s.serviceAccount,
			Expires:        time.Now().Add(s.signedURLExpiry),
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
			Expires:        time.Now().Add(s.signedURLExpiry),
		}
		url, err = storage.SignedURL(s.bucket, object, opts)
		if err != nil {
			return "", fmt.Errorf("storage.signedURL: %v", err)
		}
	}

	return url, nil
}

func (s *GCSStorage) objectExists(ctx context.Context, key string) (bool, error) {
	o := s.sc.Bucket(s.bucket).Object(key)
	_, err := o.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
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
