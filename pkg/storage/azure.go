package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

// AzureStorage is a Storage implementation backed by Azure Blob Storage.
// AzureStorage implements module.Storage, provider.Storage, and mirror.Storage
type AzureStorage struct {
	client              *azblob.Client
	account             string
	container           string
	prefix              string
	moduleArchiveFormat string
	signedURLExpiry     time.Duration
}

// GetModule retrieves information about a module from the Azure Storage.
func (s *AzureStorage) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	key := modulePath(s.prefix, namespace, name, provider, version, s.moduleArchiveFormat)

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

func (s *AzureStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	prefix := modulePathPrefix(s.prefix, namespace, name, provider)

	var modules []core.Module
	pager := s.client.NewListBlobsFlatPager(s.container, &azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("%v: %w", module.ErrModuleListFailed, err)
		}

		for _, obj := range page.Segment.BlobItems {
			m, err := moduleFromObject(*obj.Name, s.moduleArchiveFormat)
			if err != nil {
				continue
			}

			m.DownloadURL, err = s.presignedURL(ctx, modulePath(prefix, m.Namespace, m.Name, m.Provider, m.Version, s.moduleArchiveFormat))
			if err != nil {
				return []core.Module{}, err
			}

			modules = append(modules, *m)
		}
	}

	return modules, nil
}

// UploadModule uploads a module to the Azure Storage.

func (s *AzureStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
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

	key := modulePath(s.prefix, namespace, name, provider, version, DefaultModuleArchiveFormat)

	if _, err := s.GetModule(ctx, namespace, name, provider, version); err == nil {
		return core.Module{}, fmt.Errorf("%w: %s", module.ErrModuleAlreadyExists, key)
	}

	if _, err := s.client.UploadStream(ctx, s.container, key, body, nil); err != nil {
		return core.Module{}, fmt.Errorf("%v: %w", module.ErrModuleUploadFailed, err)
	}

	return s.GetModule(ctx, namespace, name, provider, version)
}

// GetProvider retrieves information about a provider from the Azure Storage.
func (s *AzureStorage) getProvider(ctx context.Context, pt providerType, provider *core.Provider) (*core.Provider, error) {
	var archivePath, shasumPath, shasumSigPath string
	if pt == internalProviderType {
		archivePath, shasumPath, shasumSigPath = internalProviderPath(s.prefix, provider.Namespace, provider.Name, provider.Version, provider.OS, provider.Arch)
	} else if pt == mirrorProviderType {
		archivePath, shasumPath, shasumSigPath = mirrorProviderPath(s.prefix, provider.Hostname, provider.Namespace, provider.Name, provider.Version, provider.OS, provider.Arch)
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

func (s *AzureStorage) listProviderVersions(ctx context.Context, pt providerType, provider *core.Provider) ([]*core.Provider, error) {
	prefix := providerStoragePrefix(s.prefix, pt, provider.Hostname, provider.Namespace, provider.Name)

	var providers []*core.Provider
	pager := s.client.NewListBlobsFlatPager(s.container, &azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to page next page: %w", err)
		}

		for _, obj := range page.Segment.BlobItems {
			p, err := core.NewProviderFromArchive(filepath.Base(*obj.Name))
			if err != nil {
				continue
			}

			if provider.Version != "" && provider.Version != p.Version {
				continue
			}

			p.Hostname = provider.Hostname
			p.Namespace = provider.Namespace
			archiveUrl, err := s.presignedURL(ctx, *obj.Name)
			if err != nil {
				return nil, err
			}
			p.DownloadURL = archiveUrl

			providers = append(providers, &p)
		}
	}

	return providers, nil
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
		return fmt.Errorf("filename argument is empty")
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

func (s *AzureStorage) presignedURL(ctx context.Context, key string) (string, error) {
	info := service.KeyInfo{
		Start:  to.Ptr(time.Now().UTC().Format(sas.TimeFormat)),
		Expiry: to.Ptr(time.Now().UTC().Add(4 * time.Hour).Format(sas.TimeFormat)),
	}

	udc, err := s.client.ServiceClient().GetUserDelegationCredential(ctx, info, nil)
	if err != nil {
		return "", err
	}

	params, err := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		ExpiryTime:    time.Now().Add(s.signedURLExpiry),
		Permissions:   to.Ptr(sas.BlobPermissions{Read: true}).String(),
		ContainerName: s.container,
		BlobName:      key,
	}.SignWithUserDelegation(udc)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s?%s", s.client.ServiceClient().NewContainerClient(s.container).NewBlobClient(key).URL(), params.Encode())

	return url, nil
}

func (s *AzureStorage) objectExists(ctx context.Context, key string) (bool, error) {
	o := s.client.ServiceClient().NewContainerClient(s.container).NewBlobClient(key)
	_, err := o.GetProperties(ctx, nil)

	if bloberror.HasCode(err, bloberror.BlobNotFound) {
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

func (s *AzureStorage) GetDownloadUrl(ctx context.Context, url string) (string, error) {
	return fmt.Sprintf("%s/%s", s.client.URL(), url), nil
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

// WithAzureStorageSignedUrlExpiry configures the duration until the signed url expires
func WithAzureStorageSignedUrlExpiry(t time.Duration) AzureStorageOption {
	return func(s *AzureStorage) {
		s.signedURLExpiry = t
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
