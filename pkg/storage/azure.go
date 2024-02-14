package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/boring-registry/boring-registry/pkg/core"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/go-kit/log"
)

// AzureStorage is a Storage implementation backed by Azure Blob Storage.
// AzureStorage implements module.Storage and provider.Storage
type AzureStorage struct {
	client              *azblob.Client
	account             string
	container           string
	prefix              string
	moduleArchiveFormat string
}

// GetModule retrieves information about a module from the Azure Storage.
func (s *AzureStorage) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	panic("Implement me!!!")
}

func (s *AzureStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	panic("Implement me!!!")
}

// UploadModule uploads a module to the Azure Storage.
func (s *AzureStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
	panic("Implement me!!!")
}

// MigrateModules is only a temporary method needed for the migration from 0.7.0 to 0.8.0 and above
func (s *AzureStorage) MigrateModules(ctx context.Context, logger log.Logger, dryRun bool) error {
	panic("Implement me!!!")
}

// MigrateProviders is a temporary method needed for the migration from 0.7.0 to 0.8.0 and above
func (s *AzureStorage) MigrateProviders(ctx context.Context, logger log.Logger, dryRun bool) error {
	panic("Implement me!!!")
}

// GetProvider retrieves information about a provider from the Azure Storage.
// TODO:
func (s *AzureStorage) getProvider(ctx context.Context, pt providerType, provider *core.Provider) (*core.Provider, error) {
	panic("Implement me!!!")
}

func (s *AzureStorage) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error) {
	p, err := s.getProvider(ctx, internalProviderType, &core.Provider{
		Namespace: namespace,
		Name:      name,
		Version:   version,
		OS:        os,
		Arch:      arch,
	})
	return p, err
}

func (s *AzureStorage) GetMirroredProvider(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
	return s.getProvider(ctx, mirrorProviderType, provider)
}

// TODO:
func (s *AzureStorage) listProviderVersions(ctx context.Context, pt providerType, provider *core.Provider) ([]*core.Provider, error) {
	panic("Implement me!!!")
}

func (s *AzureStorage) ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error) {
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

func (s *AzureStorage) ListMirroredProviders(ctx context.Context, provider *core.Provider) ([]*core.Provider, error) {
	return s.listProviderVersions(ctx, mirrorProviderType, provider)
}

func (s *AzureStorage) UploadProviderReleaseFiles(ctx context.Context, namespace, name, filename string, file io.Reader) error {
	if namespace == "" {
		return fmt.Errorf("namespace argument is empty")
	}

	if name == "" {
		return fmt.Errorf("name argument is empty")
	}

	if filename == "" {
		return fmt.Errorf("name argument is empty")
	}

	prefix := providerStoragePrefix(s.prefix, internalProviderType, "", namespace, name)
	key := filepath.Join(prefix, filename)
	return s.upload(ctx, key, file, false)
}

func (s *AzureStorage) signingKeys(ctx context.Context, pt providerType, hostname, namespace string) (*core.SigningKeys, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace argument is empty")
	}
	key := signingKeysPath(s.prefix, pt, hostname, namespace)
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

// SigningKeys downloads the JSON placed in the namespace in Azure Blob Storage and unmarshals it into a core.SigningKeys
func (s *AzureStorage) SigningKeys(ctx context.Context, namespace string) (*core.SigningKeys, error) {
	return s.signingKeys(ctx, internalProviderType, "", namespace)
}

func (s *AzureStorage) MirroredSigningKeys(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error) {
	return s.signingKeys(ctx, mirrorProviderType, hostname, namespace)
}

func (s *AzureStorage) uploadSigningKeys(ctx context.Context, pt providerType, hostname, namespace string, signingKeys *core.SigningKeys) error {
	b, err := json.Marshal(signingKeys)
	if err != nil {
		return err
	}
	key := signingKeysPath(s.prefix, pt, hostname, namespace)
	return s.upload(ctx, key, bytes.NewReader(b), true)
}

func (s *AzureStorage) UploadMirroredSigningKeys(ctx context.Context, hostname, namespace string, signingKeys *core.SigningKeys) error {
	return s.uploadSigningKeys(ctx, mirrorProviderType, hostname, namespace, signingKeys)
}

func (s *AzureStorage) MirroredSha256Sum(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error) {
	prefix := providerStoragePrefix(s.prefix, mirrorProviderType, provider.Hostname, provider.Namespace, provider.Name)
	key := filepath.Join(prefix, provider.ShasumFileName())
	shaSumBytes, err := s.download(ctx, key)
	if err != nil {
		return nil, errors.New("failed to download SHA256SUMS")
	}

	return core.NewSha256Sums(provider.ShasumFileName(), bytes.NewReader(shaSumBytes))
}

func (s *AzureStorage) UploadMirroredFile(ctx context.Context, provider *core.Provider, fileName string, reader io.Reader) error {
	prefix := providerStoragePrefix(s.prefix, mirrorProviderType, provider.Hostname, provider.Namespace, provider.Name)
	key := filepath.Join(prefix, fileName)
	return s.upload(ctx, key, reader, true)
}

// func (s *AzureStorage) presignedURL(ctx context.Context, key string) (string, error) {
// 	panic("Implement me!!!")
// }

// TODO:
func (s *AzureStorage) objectExists(ctx context.Context, key string) (bool, error) {
	o := s.client.ServiceClient().NewContainerClient(s.container).NewBlobClient(key)
	_, err := o.GetProperties(ctx, nil)
	if errors.Is(err, err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func (s *AzureStorage) upload(ctx context.Context, key string, reader io.Reader, overwrite bool) error {
	if !overwrite {
		exists, err := s.objectExists(ctx, key)
		if err != nil {
			return err
		} else if exists {
			return fmt.Errorf("failed to upload key %s: %w", key, core.ErrObjectAlreadyExists)
		}
	}

	if _, err := s.client.UploadStream(ctx, s.container, key, reader, nil); err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}

	return nil
}

func (s *AzureStorage) download(ctx context.Context, key string) ([]byte, error) {
	r, err := s.client.DownloadStream(ctx, s.container, key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", key, err)
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// AzureStorageOption provides additional options for the AzureStorage.
type AzureStorageOption func(*AzureStorage)

// WithAzureStoragePrefix configures the azure storage to work under a given prefix.
func WithAzureStoragePrefix(prefix string) AzureStorageOption {
	return func(s *AzureStorage) {
		s.prefix = prefix
	}
}

// WithAzureStorageArchiveFormat configures the module archive format (zip, tar, tgz, etc.)
func WithAzureStorageArchiveFormat(archiveFormat string) AzureStorageOption {
	return func(s *AzureStorage) {
		s.moduleArchiveFormat = archiveFormat
	}
}

// NewAzureStorage returns a fully initialized Azure Storage.
func NewAzureStorage(account string, container string, options ...AzureStorageOption) (Storage, error) {
	s := &AzureStorage{
		account:   account,
		container: container,
	}

	for _, option := range options {
		option(s)
	}

	url := fmt.Sprintf("https://%s.blob.core.windows.net/", account)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}

	client, err := azblob.NewClient(url, cred, nil)
	if err != nil {
		return nil, err
	}

	s.client = client

	return s, nil
}
