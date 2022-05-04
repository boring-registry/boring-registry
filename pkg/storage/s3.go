package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
	"path"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
)

// S3Storage is a Storage implementation backed by S3.
// S3Storage implements provider.Storage
type S3Storage struct {
	s3             *s3.S3
	downloader     *s3manager.Downloader
	uploader       *s3manager.Uploader
	bucket         string
	bucketPrefix   string
	bucketRegion   string
	pathStyle      bool
	bucketEndpoint string
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
		return nil, errors.Wrap(ErrListFailed, err.Error())
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
