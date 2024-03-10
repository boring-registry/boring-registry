package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3ClientAPI is used to mock the AWS APIs
// See https://aws.github.io/aws-sdk-go-v2/docs/unit-testing/
type s3ClientAPI interface {
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, f ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
}

// s3UploaderAPI is used to mock the AWS APIs
// See https://aws.github.io/aws-sdk-go-v2/docs/unit-testing/
type s3UploaderAPI interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

// s3DownloaderAPI is used to mock the AWS APIs
// See https://aws.github.io/aws-sdk-go-v2/docs/unit-testing/
type s3DownloaderAPI interface {
	Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(api *s3manager.Downloader)) (n int64, err error)
}

// s3PresignClientAPI is used to mock the AWS APIs
// See https://aws.github.io/aws-sdk-go-v2/docs/unit-testing/
type s3PresignClientAPI interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*signer.PresignedHTTPRequest, error)
}

// S3Storage is a Storage implementation backed by S3.
// S3Storage implements module.Storage, provider.Storage, and mirror.Storage
type S3Storage struct {
	client              s3ClientAPI
	presignClient       s3PresignClientAPI
	downloader          s3DownloaderAPI
	uploader            s3UploaderAPI
	bucket              string
	bucketPrefix        string
	bucketRegion        string
	bucketEndpoint      string
	moduleArchiveFormat string
	forcePathStyle      bool
	signedURLExpiry     time.Duration
}

// GetModule retrieves information about a module from the S3 storage.
func (s *S3Storage) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	key := modulePath(s.bucketPrefix, namespace, name, provider, version, s.moduleArchiveFormat)

	exists, err := s.objectExists(ctx, key)
	if err != nil {
		return core.Module{}, err
	} else if !exists {
		return core.Module{}, module.ErrModuleNotFound
	}

	presigned, err := s.presignedURL(ctx, key)
	if err != nil {
		return core.Module{}, err
	}

	return core.Module{
		Namespace:   namespace,
		Name:        name,
		Provider:    provider,
		Version:     version,
		DownloadURL: presigned,
	}, nil
}

func (s *S3Storage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(modulePathPrefix(s.bucketPrefix, namespace, name, provider)),
	}

	var modules []core.Module
	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("%v: %w", module.ErrModuleListFailed, err)
		}

		for _, obj := range resp.Contents {
			m, err := moduleFromObject(*obj.Key, s.moduleArchiveFormat)
			if err != nil {
				// TODO: we're skipping possible failures silently
				continue
			}

			// The download URL is probably not necessary for ListModules
			m.DownloadURL, err = s.presignedURL(ctx, modulePath(s.bucketPrefix, m.Namespace, m.Name, m.Provider, m.Version, s.moduleArchiveFormat))
			if err != nil {
				return []core.Module{}, err
			}

			modules = append(modules, *m)
		}
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
		return core.Module{}, fmt.Errorf("%w: %s", module.ErrModuleAlreadyExists, key)
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}

	if _, err := s.uploader.Upload(ctx, input); err != nil {
		return core.Module{}, fmt.Errorf("%v: %w", module.ErrModuleUploadFailed, err)
	}

	return s.GetModule(ctx, namespace, name, provider, version)
}

// GetProvider retrieves information about a provider from the S3 storage.
func (s *S3Storage) getProvider(ctx context.Context, pt providerType, provider *core.Provider) (*core.Provider, error) {
	var archivePath, shasumPath, shasumSigPath string
	if pt == internalProviderType {
		archivePath, shasumPath, shasumSigPath = internalProviderPath(s.bucketPrefix, provider.Namespace, provider.Name, provider.Version, provider.OS, provider.Arch)
	} else if pt == mirrorProviderType {
		archivePath, shasumPath, shasumSigPath = mirrorProviderPath(s.bucketPrefix, provider.Hostname, provider.Namespace, provider.Name, provider.Version, provider.OS, provider.Arch)
	}

	if exists, err := s.objectExists(ctx, archivePath); err != nil {
		return nil, err
	} else if !exists {
		return nil, noMatchingProviderFound(provider)
	}

	var err error
	provider.DownloadURL, err = s.presignedURL(ctx, archivePath)
	if err != nil {
		return nil, err
	}
	provider.SHASumsURL, err = s.presignedURL(ctx, shasumPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned url for %s: %w", shasumPath, err)
	}
	provider.SHASumsSignatureURL, err = s.presignedURL(ctx, shasumSigPath)
	if err != nil {
		return nil, err
	}

	shasumBytes, err := s.download(ctx, shasumPath)
	if err != nil {
		return nil, err
	}

	provider.Shasum, err = readSHASums(bytes.NewReader(shasumBytes), path.Base(archivePath))
	if err != nil {
		return nil, err
	}

	var signingKeys *core.SigningKeys
	if pt == internalProviderType {
		signingKeys, err = s.SigningKeys(ctx, provider.Namespace)
	} else if pt == mirrorProviderType {
		signingKeys, err = s.MirroredSigningKeys(ctx, provider.Hostname, provider.Namespace)
	}
	if err != nil {
		return nil, err
	}

	provider.Filename = path.Base(archivePath)
	provider.SigningKeys = *signingKeys
	return provider, nil
}

func (s *S3Storage) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error) {
	p, err := s.getProvider(ctx, internalProviderType, &core.Provider{
		Namespace: namespace,
		Name:      name,
		Version:   version,
		OS:        os,
		Arch:      arch,
	})

	return p, err
}

func (s *S3Storage) GetMirroredProvider(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
	return s.getProvider(ctx, mirrorProviderType, provider)
}

func (s *S3Storage) listProviderVersions(ctx context.Context, pt providerType, provider *core.Provider) ([]*core.Provider, error) {
	prefix := providerStoragePrefix(s.bucketPrefix, pt, provider.Hostname, provider.Namespace, provider.Name)
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(fmt.Sprintf("%s/", prefix)),
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, input)

	var providers []*core.Provider
	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to page next page: %w", err)
		}

		for _, obj := range resp.Contents {
			p, err := core.NewProviderFromArchive(filepath.Base(*obj.Key))
			if err != nil {
				continue
			}

			if provider.Version != "" && provider.Version != p.Version {
				// The provider version doesn't match the requested version
				continue
			}

			p.Hostname = provider.Hostname
			p.Namespace = provider.Namespace
			archiveUrl, err := s.presignedURL(ctx, *obj.Key)
			if err != nil {
				return nil, err
			}
			p.DownloadURL = archiveUrl

			providers = append(providers, &p)
		}
	}

	if len(providers) == 0 {
		return nil, noMatchingProviderFound(provider)
	}

	return providers, nil
}

func (s *S3Storage) ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error) {
	providers, err := s.listProviderVersions(ctx, internalProviderType, &core.Provider{Namespace: namespace, Name: name})
	if err != nil {
		return nil, err
	}

	collection := NewCollection()
	for _, p := range providers {
		collection.Add(p)
	}
	return collection.List(), nil
}

func (s *S3Storage) ListMirroredProviders(ctx context.Context, provider *core.Provider) ([]*core.Provider, error) {
	return s.listProviderVersions(ctx, mirrorProviderType, provider)
}

func (s *S3Storage) UploadProviderReleaseFiles(ctx context.Context, namespace, name, filename string, file io.Reader) error {
	if namespace == "" {
		return fmt.Errorf("namespace argument is empty")
	}

	if name == "" {
		return fmt.Errorf("name argument is empty")
	}

	if filename == "" {
		return fmt.Errorf("filename argument is empty")
	}

	prefix := providerStoragePrefix(s.bucketPrefix, internalProviderType, "", namespace, name)
	key := filepath.Join(prefix, filename)
	return s.upload(ctx, key, file, false)
}

func (s *S3Storage) signingKeys(ctx context.Context, pt providerType, hostname, namespace string) (*core.SigningKeys, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace argument is empty")
	}
	key := signingKeysPath(s.bucketPrefix, pt, hostname, namespace)
	exists, err := s.objectExists(ctx, key)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, core.ErrObjectNotFound
	}

	signingKeysRaw, err := s.download(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to download signing_keys.json for namespace %s: %w", namespace, err)
	}

	return unmarshalSigningKeys(signingKeysRaw)
}

// SigningKeys downloads the JSON placed in the namespace in S3 and unmarshals it into a core.SigningKeys
func (s *S3Storage) SigningKeys(ctx context.Context, namespace string) (*core.SigningKeys, error) {
	return s.signingKeys(ctx, internalProviderType, "", namespace)
}

func (s *S3Storage) MirroredSigningKeys(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error) {
	return s.signingKeys(ctx, mirrorProviderType, hostname, namespace)
}

func (s *S3Storage) uploadSigningKeys(ctx context.Context, pt providerType, hostname, namespace string, signingKeys *core.SigningKeys) error {
	b, err := json.Marshal(signingKeys)
	if err != nil {
		return err
	}
	key := signingKeysPath(s.bucketPrefix, pt, hostname, namespace)
	return s.upload(ctx, key, bytes.NewReader(b), true)
}

func (s *S3Storage) UploadMirroredSigningKeys(ctx context.Context, hostname, namespace string, signingKeys *core.SigningKeys) error {
	return s.uploadSigningKeys(ctx, mirrorProviderType, hostname, namespace, signingKeys)
}

func (s *S3Storage) MirroredSha256Sum(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error) {
	prefix := providerStoragePrefix(s.bucketPrefix, mirrorProviderType, provider.Hostname, provider.Namespace, provider.Name)
	key := filepath.Join(prefix, provider.ShasumFileName())
	shaSumBytes, err := s.download(ctx, key)
	if err != nil {
		return nil, errors.New("failed to download SHA256SUMS")
	}

	return core.NewSha256Sums(provider.ShasumFileName(), bytes.NewReader(shaSumBytes))
}

func (s *S3Storage) UploadMirroredFile(ctx context.Context, provider *core.Provider, fileName string, reader io.Reader) error {
	prefix := providerStoragePrefix(s.bucketPrefix, mirrorProviderType, provider.Hostname, provider.Namespace, provider.Name)
	key := filepath.Join(prefix, fileName)
	return s.upload(ctx, key, reader, true)
}

func (s *S3Storage) presignedURL(ctx context.Context, key string) (string, error) {
	presignResult, err := s.presignClient.PresignGetObject(ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		},
		s3.WithPresignExpires(s.signedURLExpiry),
	)

	return presignResult.URL, err
}

func (s *S3Storage) objectExists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	if _, err := s.client.HeadObject(ctx, input); err != nil {
		var responseError *awshttp.ResponseError
		if errors.As(err, &responseError) && responseError.ResponseError.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s *S3Storage) upload(ctx context.Context, key string, reader io.Reader, overwrite bool) error {
	// If we don't want to overwrite, check if the object exists
	if !overwrite {
		exists, err := s.objectExists(ctx, key)
		if err != nil {
			return err
		} else if exists {
			return fmt.Errorf("failed to upload key %s: %w", key, core.ErrObjectAlreadyExists)
		}
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   reader,
	}

	if _, err := s.uploader.Upload(ctx, input); err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}

	return nil
}

func (s *S3Storage) download(ctx context.Context, key string) ([]byte, error) {
	buf := s3manager.NewWriteAtBuffer([]byte{})

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	if _, err := s.downloader.Download(ctx, buf, input); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", key, err)
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
// TODO: the AWS signing region could be another one as the bucket location
func WithS3StorageBucketRegion(region string) S3StorageOption {
	return func(s *S3Storage) {
		s.bucketRegion = region
	}
}

// WithS3StorageBucketEndpoint configures the endpoint for a given s3 storage. (needed for MINIO)
func WithS3StorageBucketEndpoint(endpoint string) S3StorageOption {
	return func(s *S3Storage) {
		s.bucketEndpoint = endpoint
	}
}

// WithS3ArchiveFormat configures the module archive format (zip, tar, tgz, etc.)
func WithS3ArchiveFormat(archiveFormat string) S3StorageOption {
	return func(s *S3Storage) {
		s.moduleArchiveFormat = archiveFormat
	}
}

// WithS3StoragePathStyle configures if Path Style is used for a given s3 storage. (needed for MINIO)
func WithS3StoragePathStyle(forcePathStyle bool) S3StorageOption {
	return func(s *S3Storage) {
		s.forcePathStyle = forcePathStyle
	}
}

// WithS3StorageSignedUrlExpiry configures the duration until the signed url expires
func WithS3StorageSignedUrlExpiry(t time.Duration) S3StorageOption {
	return func(s *S3Storage) {
		s.signedURLExpiry = t
	}
}

// NewS3Storage returns a fully initialized S3 storage.
func NewS3Storage(ctx context.Context, bucket string, options ...S3StorageOption) (Storage, error) {
	// Required- and default-values should be set here
	s := &S3Storage{
		bucket: bucket,
	}

	for _, option := range options {
		option(s)
	}

	// The EndpointResolver is used for compatibility with MinIO
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if s.bucketEndpoint != "" {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               s.bucketEndpoint,
				HostnameImmutable: true, // Needs to be true for MinIO
			}, nil
		}

		// returning EndpointNotFoundError will allow the service to fall back to its default resolution
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	// Create the S3 client
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s.bucketRegion), config.WithEndpointResolverWithOptions(customResolver))
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)
	s.client = client
	s.presignClient = s3.NewPresignClient(client)
	s.uploader = s3manager.NewUploader(client)
	s.downloader = s3manager.NewDownloader(client)

	if s.bucketRegion == "" {
		region, err := s3manager.GetBucketRegion(ctx, client, s.bucket)
		if err != nil {
			return nil, fmt.Errorf("failed to determine bucket region: %w", err)
		}
		s.bucketRegion = region
	}

	return s, nil
}
