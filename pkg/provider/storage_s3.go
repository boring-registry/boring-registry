package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
)

// S3Storage is a Storage implementation backed by S3.
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
func (s *S3Storage) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (Provider, error) {
	var (
		pathPkg         = storagePath(s.bucketPrefix, namespace, name, version, os, arch)
		pathSha         = shasumsPath(s.bucketPrefix, namespace, name, version)
		pathSig         = fmt.Sprintf("%s.sig", pathSha)
		pathSigningKeys = signingKeysPath(s.bucketPrefix, namespace)
	)

	shasumsURL, err := s.presignedURL(pathSha)
	if err != nil {
		return Provider{}, errors.Wrap(err, pathSig)
	}

	signatureURL, err := s.presignedURL(pathSig)
	if err != nil {
		return Provider{}, err
	}

	zipURL, err := s.presignedURL(pathPkg)
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

func (s *S3Storage) ListProviderVersions(ctx context.Context, namespace, name string) ([]ProviderVersion, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(fmt.Sprintf("%s/", storagePrefix(s.bucketPrefix, namespace, name))),
	}

	collection := NewCollection()
	fn := func(page *s3.ListObjectsV2Output, last bool) bool {
		for _, obj := range page.Contents {
			provider, err := Parse(*obj.Key)
			if err != nil {
				continue
			}

			collection.Add(provider)
		}

		return true
	}

	if err := s.s3.ListObjectsV2Pages(input, fn); err != nil {
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
func NewS3Storage(bucket string, options ...S3StorageOption) (Storage, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	s := &S3Storage{
		s3:         s3.New(sess),
		uploader:   s3manager.NewUploader(sess),
		downloader: s3manager.NewDownloader(sess),
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

func (s *S3Storage) presignedURL(v string) (string, error) {
	req, _ := s.s3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(v),
	})

	return req.Presign(15 * time.Minute)
}

func (s *S3Storage) download(path string) ([]byte, error) {
	buf := aws.NewWriteAtBuffer([]byte{})

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	if _, err := s.downloader.Download(buf, input); err != nil {
		return nil, errors.Wrapf(err, "failed to download: %s", path)
	}

	return buf.Bytes(), nil
}
