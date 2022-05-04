package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
)

const (
	s3downloadFormat = "%s.s3-%s.amazonaws.com/%s"
)

// S3Storage is a Storage implementation backed by S3.
// S3Storage implements module.Storage and provider.Storage
type S3Storage struct {
	s3                  *s3.S3
	downloader          *s3manager.Downloader
	uploader            *s3manager.Uploader
	bucket              string
	bucketPrefix        string
	bucketRegion        string
	bucketEndpoint      string
	moduleArchiveFormat string
	pathStyle           bool
}

// GetModule retrieves information about a module from the S3 storage.
func (s *S3Storage) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	key := modulePath(s.bucketPrefix, namespace, name, provider, version, s.moduleArchiveFormat)

	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	if _, err := s.s3.HeadObject(input); err != nil {
		return core.Module{}, errors.Wrap(ErrModuleNotFound, err.Error())
	}

	return core.Module{
		Namespace:   namespace,
		Name:        name,
		Provider:    provider,
		Version:     version,
		DownloadURL: fmt.Sprintf(s3downloadFormat, s.bucket, s.bucketRegion, *input.Key),
	}, nil
}

func (s *S3Storage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	var modules []core.Module

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(modulePathPrefix(s.bucketPrefix, namespace, name, provider)),
	}

	fn := func(page *s3.ListObjectsV2Output, last bool) bool {
		for _, obj := range page.Contents {
			metadata := objectMetadata(*obj.Key)

			version, ok := metadata["version"]
			if !ok {
				continue
			}

			m := core.Module{
				Namespace:   namespace,
				Name:        name,
				Provider:    provider,
				Version:     version,
				DownloadURL: fmt.Sprintf(s3downloadFormat, s.bucket, s.bucketRegion, *obj.Key),
			}

			modules = append(modules, m)
		}

		return true
	}

	if err := s.s3.ListObjectsV2Pages(input, fn); err != nil {
		return nil, errors.Wrap(ErrModuleListFailed, err.Error())
	}

	return modules, nil
}

// UploadModule uploads a module to the S3 storage.
func (s *S3Storage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
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

	input := &s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}

	if _, err := s.uploader.Upload(input); err != nil {
		return core.Module{}, errors.Wrapf(ErrModuleUploadFailed, err.Error())
	}

	return s.GetModule(ctx, namespace, name, provider, version)
}

// GetProvider retrieves information about a provider from the S3 storage.
func (s *S3Storage) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (core.Provider, error) {
	archivePath, shasumPath, shasumSigPath, err := internalProviderPath(s.bucketPrefix, namespace, name, version, os, arch)
	if err != nil {
		return core.Provider{}, err
	}

	pathSigningKeys := signingKeysPath(s.bucketPrefix, namespace)

	zipURL, err := s.presignedURL(archivePath)
	if err != nil {
		return core.Provider{}, err
	}
	shasumsURL, err := s.presignedURL(shasumPath)
	if err != nil {
		return core.Provider{}, errors.Wrap(err, shasumPath)
	}
	signatureURL, err := s.presignedURL(shasumSigPath)
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
		Name:                name,
		Version:             version,
		OS:                  os,
		Arch:                arch,
		Shasum:              shasum,
		Filename:            path.Base(archivePath),
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

func (s *S3Storage) ListProviderVersions(ctx context.Context, namespace, name string) ([]core.ProviderVersion, error) {
	prefix, err := providerStoragePrefix(s.bucketPrefix, internalProviderType, "", namespace, name)
	if err != nil {
		return nil, err
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(fmt.Sprintf("%s/", prefix)),
	}

	collection := NewCollection()
	fn := func(page *s3.ListObjectsV2Output, last bool) bool {
		for _, obj := range page.Contents {
			provider, err := core.NewProviderFromArchive(*obj.Key)
			if err != nil {
				continue
			}

			collection.Add(provider)
		}

		return true
	}

	if err := s.s3.ListObjectsV2PagesWithContext(ctx, input, fn); err != nil {
		return nil, errors.Wrap(ErrProviderListFailed, err.Error())
	}

	result := collection.List()

	if len(result) == 0 {
		return nil, fmt.Errorf("no provider versions found for %s/%s", namespace, name)
	}

	return result, nil
}

func (s *S3Storage) determineBucketRegion() (string, error) {
	region, err := s3manager.GetBucketRegionWithClient(context.Background(), s.s3, s.bucket)
	if err != nil {
		return "", err
	}

	return region, nil
}
func (s *S3Storage) presignedURL(v string) (string, error) {
	req, _ := s.s3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(v),
	})

	return req.Presign(15 * time.Minute)
}

func (s *S3Storage) download(ctx context.Context, path string) ([]byte, error) {
	buf := aws.NewWriteAtBuffer([]byte{})

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	if _, err := s.downloader.DownloadWithContext(ctx, buf, input); err != nil {
		return nil, errors.Wrapf(err, "failed to download: %s", path)
	}

	return buf.Bytes(), nil
}

// S3StorageOption provides additional options for the S3Storage.
type S3StorageOption func(*S3Storage)

// WithS3StorageBucketPrefix configures the s3 storage to work under a given prefix.
func WithS3StorageBucketPrefix(prefix string) S3StorageOption {
	return func(s *S3Storage) {
		s.bucketPrefix = prefix
	}
}

// WithS3StorageBucketRegion configures the region for a given s3 storage.
func WithS3StorageBucketRegion(region string) S3StorageOption {
	return func(s *S3Storage) {
		s.bucketRegion = region
	}
}

// WithS3StorageBucketEndpoint configures the endpoint for a given s3 storage. (needed for MINIO)
func WithS3StorageBucketEndpoint(endpoint string) S3StorageOption {
	return func(s *S3Storage) {
		// default value is "", so don't set and leave to aws sdk
		if len(endpoint) > 0 {
			s.s3.Client.Endpoint = endpoint
		}
		s.bucketEndpoint = "aws sdk default"
	}
}

// WithS3ArchiveFormat configures the module archive format (zip, tar, tgz, etc.)
func WithS3ArchiveFormat(archiveFormat string) S3StorageOption {
	return func(s *S3Storage) {
		s.moduleArchiveFormat = archiveFormat
	}
}

// WithS3StoragePathStyle configures if Path Style is used for a given s3 storage. (needed for MINIO)
func WithS3StoragePathStyle(pathStyle bool) S3StorageOption {
	return func(s *S3Storage) {
		// only set if true, default value is false but leave for aws sdk
		if pathStyle {
			s.s3.Client.Config.S3ForcePathStyle = &pathStyle
		}
		s.pathStyle = pathStyle
	}
}

// NewS3Storage returns a fully initialized S3 storage.
func NewS3Storage(bucket string, options ...S3StorageOption) (*S3Storage, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	client := s3.New(sess)
	s := &S3Storage{
		s3:         client,
		uploader:   s3manager.NewUploaderWithClient(client),
		downloader: s3manager.NewDownloaderWithClient(client),
		bucket:     bucket,
	}

	for _, option := range options {
		option(s)
	}

	if s.bucketRegion == "" {
		region, err := s.determineBucketRegion()
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine bucket region")
		}
		s.bucketRegion = region
	}

	return s, nil
}
