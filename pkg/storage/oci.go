package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/module"
)

// ociClientAPI is used to mock the OCI APIs for testing
type ociClientAPI interface {
	ArtifactExists(ctx context.Context, reference string) (bool, error)
	DownloadArtifact(ctx context.Context, reference string) ([]byte, error)
	UploadArtifact(ctx context.Context, reference string, content io.Reader, overwrite bool) error
	ListTags(ctx context.Context, repository string, callback func(tags []string) error) error
	GenerateDownloadURL(ctx context.Context, reference string) (string, error)
}

// OCIStorage is a Storage implementation backed by OCI Registry.
// OCIStorage implements module.Storage, provider.Storage, and mirror.Storage
type OCIStorage struct {
	client              ociClientAPI
	registry            string
	repository          string
	repositoryPrefix    string
	username            string
	password            string
	token               string
	insecure            bool
	moduleArchiveFormat string
	signedURLExpiry     time.Duration
	httpClient          *http.Client
}

// GetModule retrieves information about a module from the OCI storage.
func (s *OCIStorage) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	ref := s.buildModuleReference(namespace, name, provider, version)
	
	exists, err := s.client.ArtifactExists(ctx, ref)
	if err != nil {
		return core.Module{}, err
	} else if !exists {
		return core.Module{}, module.ErrModuleNotFound
	}

	downloadURL, err := s.client.GenerateDownloadURL(ctx, ref)
	if err != nil {
		return core.Module{}, err
	}

	return core.Module{
		Namespace:   namespace,
		Name:        name,
		Provider:    provider,
		Version:     version,
		DownloadURL: downloadURL,
	}, nil
}

func (s *OCIStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	repoRef := s.buildModuleRepositoryReference(namespace, name, provider)
	
	var modules []core.Module
	
	err := s.client.ListTags(ctx, repoRef, func(tags []string) error {
		for _, tag := range tags {
			if strings.HasPrefix(tag, fmt.Sprintf("%s-%s-%s-", namespace, name, provider)) {
				version := strings.TrimPrefix(tag, fmt.Sprintf("%s-%s-%s-", namespace, name, provider))
				
				downloadURL, err := s.client.GenerateDownloadURL(ctx, fmt.Sprintf("%s:%s", repoRef, tag))
				if err != nil {
					return err
				}
				
				modules = append(modules, core.Module{
					Namespace:   namespace,
					Name:        name,
					Provider:    provider,
					Version:     version,
					DownloadURL: downloadURL,
				})
			}
		}
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("%v: %w", module.ErrModuleListFailed, err)
	}

	return modules, nil
}

// UploadModule uploads a module to the OCI storage.
func (s *OCIStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
	if namespace == "" {
		return core.Module{}, fmt.Errorf("namespace not defined")
	}

	if name == "" {
		return core.Module{}, fmt.Errorf("name not defined")
	}

	if provider == "" {
		return core.Module{}, fmt.Errorf("provider not defined")
	}

	if version == "" {
		return core.Module{}, fmt.Errorf("version not defined")
	}

	ref := s.buildModuleReference(namespace, name, provider, version)

	if _, err := s.GetModule(ctx, namespace, name, provider, version); err == nil {
		return core.Module{}, fmt.Errorf("%w: %s", module.ErrModuleAlreadyExists, ref)
	}

	if err := s.client.UploadArtifact(ctx, ref, body, false); err != nil {
		return core.Module{}, fmt.Errorf("%v: %w", module.ErrModuleUploadFailed, err)
	}

	return s.GetModule(ctx, namespace, name, provider, version)
}

// GetProvider retrieves information about a provider from the OCI storage.
func (s *OCIStorage) getProvider(ctx context.Context, pt providerType, provider *core.Provider) (*core.Provider, error) {
	var ref string
	if pt == internalProviderType {
		ref = s.buildInternalProviderReference(provider.Namespace, provider.Name, provider.Version, provider.OS, provider.Arch)
	} else if pt == mirrorProviderType {
		ref = s.buildMirrorProviderReference(provider.Hostname, provider.Namespace, provider.Name, provider.Version, provider.OS, provider.Arch)
	}

	exists, err := s.client.ArtifactExists(ctx, ref)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, noMatchingProviderFound(provider)
	}

	// Generate download URLs for provider artifacts
	provider.DownloadURL, err = s.client.GenerateDownloadURL(ctx, ref)
	if err != nil {
		return nil, err
	}
	
	shasumRef := s.buildShasumReference(pt, provider.Hostname, provider.Namespace, provider.Name, provider.Version)
	provider.SHASumsURL, err = s.client.GenerateDownloadURL(ctx, shasumRef)
	if err != nil {
		return nil, fmt.Errorf("failed to generate download url for %s: %w", shasumRef, err)
	}
	
	shasumSigRef := s.buildShasumSignatureReference(pt, provider.Hostname, provider.Namespace, provider.Name, provider.Version)
	provider.SHASumsSignatureURL, err = s.client.GenerateDownloadURL(ctx, shasumSigRef)
	if err != nil {
		return nil, err
	}

	// Download and parse shasum
	shasumBytes, err := s.client.DownloadArtifact(ctx, shasumRef)
	if err != nil {
		return nil, err
	}

	provider.Shasum, err = readSHASums(bytes.NewReader(shasumBytes), s.buildProviderArchiveFilename(provider.Name, provider.Version, provider.OS, provider.Arch))
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

	provider.Filename = s.buildProviderArchiveFilename(provider.Name, provider.Version, provider.OS, provider.Arch)
	provider.SigningKeys = *signingKeys
	return provider, nil
}

func (s *OCIStorage) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error) {
	p, err := s.getProvider(ctx, internalProviderType, &core.Provider{
		Namespace: namespace,
		Name:      name,
		Version:   version,
		OS:        os,
		Arch:      arch,
	})

	return p, err
}

func (s *OCIStorage) GetMirroredProvider(ctx context.Context, provider *core.Provider) (*core.Provider, error) {
	return s.getProvider(ctx, mirrorProviderType, provider)
}

func (s *OCIStorage) listProviderVersions(ctx context.Context, pt providerType, provider *core.Provider) ([]*core.Provider, error) {
	repoRef := s.buildProviderRepositoryReference(pt, provider.Hostname, provider.Namespace, provider.Name)
	
	var providers []*core.Provider
	
	err := s.client.ListTags(ctx, repoRef, func(tags []string) error {
		for _, tag := range tags {
			p, err := core.NewProviderFromArchive(tag)
			if err != nil {
				continue
			}

			if provider.Version != "" && provider.Version != p.Version {
				// The provider version doesn't match the requested version
				continue
			}

			p.Hostname = provider.Hostname
			p.Namespace = provider.Namespace
			
			archiveUrl, err := s.client.GenerateDownloadURL(ctx, fmt.Sprintf("%s:%s", repoRef, tag))
			if err != nil {
				return err
			}
			p.DownloadURL = archiveUrl

			providers = append(providers, &p)
		}
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to list provider versions: %w", err)
	}

	if len(providers) == 0 {
		return nil, noMatchingProviderFound(provider)
	}

	return providers, nil
}

func (s *OCIStorage) ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error) {
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

func (s *OCIStorage) ListMirroredProviders(ctx context.Context, provider *core.Provider) ([]*core.Provider, error) {
	return s.listProviderVersions(ctx, mirrorProviderType, provider)
}

func (s *OCIStorage) UploadProviderReleaseFiles(ctx context.Context, namespace, name, filename string, file io.Reader) error {
	if namespace == "" {
		return fmt.Errorf("namespace argument is empty")
	}

	if name == "" {
		return fmt.Errorf("name argument is empty")
	}

	if filename == "" {
		return fmt.Errorf("filename argument is empty")
	}

	ref := s.buildProviderReleaseFileReference(namespace, name, filename)
	
	// Check if artifact already exists when not overwriting
	exists, err := s.client.ArtifactExists(ctx, ref)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("failed to upload %s: %w", ref, core.ErrObjectAlreadyExists)
	}
	
	return s.client.UploadArtifact(ctx, ref, file, false)
}

func (s *OCIStorage) signingKeys(ctx context.Context, pt providerType, hostname, namespace string) (*core.SigningKeys, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace argument is empty")
	}
	
	ref := s.buildSigningKeysReference(pt, hostname, namespace)
	exists, err := s.client.ArtifactExists(ctx, ref)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, core.ErrObjectNotFound
	}

	signingKeysRaw, err := s.client.DownloadArtifact(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to download signing_keys.json for namespace %s: %w", namespace, err)
	}

	return unmarshalSigningKeys(signingKeysRaw)
}

// SigningKeys downloads the JSON placed in the namespace in OCI registry and unmarshals it into a core.SigningKeys
func (s *OCIStorage) SigningKeys(ctx context.Context, namespace string) (*core.SigningKeys, error) {
	return s.signingKeys(ctx, internalProviderType, "", namespace)
}

func (s *OCIStorage) MirroredSigningKeys(ctx context.Context, hostname, namespace string) (*core.SigningKeys, error) {
	return s.signingKeys(ctx, mirrorProviderType, hostname, namespace)
}

func (s *OCIStorage) uploadSigningKeys(ctx context.Context, pt providerType, hostname, namespace string, signingKeys *core.SigningKeys) error {
	b, err := json.Marshal(signingKeys)
	if err != nil {
		return err
	}
	
	ref := s.buildSigningKeysReference(pt, hostname, namespace)
	return s.client.UploadArtifact(ctx, ref, bytes.NewReader(b), true)
}

func (s *OCIStorage) UploadMirroredSigningKeys(ctx context.Context, hostname, namespace string, signingKeys *core.SigningKeys) error {
	return s.uploadSigningKeys(ctx, mirrorProviderType, hostname, namespace, signingKeys)
}

func (s *OCIStorage) MirroredSha256Sum(ctx context.Context, provider *core.Provider) (*core.Sha256Sums, error) {
	ref := s.buildShasumReference(mirrorProviderType, provider.Hostname, provider.Namespace, provider.Name, provider.Version)
	shaSumBytes, err := s.client.DownloadArtifact(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to download SHA256SUMS: %w", err)
	}

	return core.NewSha256Sums(provider.ShasumFileName(), bytes.NewReader(shaSumBytes))
}

func (s *OCIStorage) UploadMirroredFile(ctx context.Context, provider *core.Provider, fileName string, reader io.Reader) error {
	ref := s.buildMirroredFileReference(provider.Hostname, provider.Namespace, provider.Name, fileName)
	return s.client.UploadArtifact(ctx, ref, reader, true)
}

func (s *OCIStorage) GetDownloadUrl(ctx context.Context, url string) (string, error) {
	return url, nil
}

// Helper methods for building OCI references
func (s *OCIStorage) buildModuleReference(namespace, name, provider, version string) string {
	tag := fmt.Sprintf("%s-%s-%s-%s", namespace, name, provider, version)
	return fmt.Sprintf("%s/%s/modules/%s:%s", s.registry, s.buildRepositoryPath(), s.buildModulePath(namespace, name, provider), tag)
}

func (s *OCIStorage) buildModuleRepositoryReference(namespace, name, provider string) string {
	return fmt.Sprintf("%s/%s/modules/%s", s.registry, s.buildRepositoryPath(), s.buildModulePath(namespace, name, provider))
}

func (s *OCIStorage) buildModulePath(namespace, name, provider string) string {
	return fmt.Sprintf("%s/%s/%s", namespace, name, provider)
}

func (s *OCIStorage) buildInternalProviderReference(namespace, name, version, os, arch string) string {
	filename := s.buildProviderArchiveFilename(name, version, os, arch)
	return fmt.Sprintf("%s/%s/providers/%s/%s:%s", s.registry, s.buildRepositoryPath(), namespace, name, filename)
}

func (s *OCIStorage) buildMirrorProviderReference(hostname, namespace, name, version, os, arch string) string {
	filename := s.buildProviderArchiveFilename(name, version, os, arch)
	return fmt.Sprintf("%s/%s/mirror/providers/%s/%s/%s:%s", s.registry, s.buildRepositoryPath(), hostname, namespace, name, filename)
}

func (s *OCIStorage) buildProviderRepositoryReference(pt providerType, hostname, namespace, name string) string {
	if pt == internalProviderType {
		return fmt.Sprintf("%s/%s/providers/%s/%s", s.registry, s.buildRepositoryPath(), namespace, name)
	}
	return fmt.Sprintf("%s/%s/mirror/providers/%s/%s/%s", s.registry, s.buildRepositoryPath(), hostname, namespace, name)
}

func (s *OCIStorage) buildProviderReleaseFileReference(namespace, name, filename string) string {
	return fmt.Sprintf("%s/%s/providers/%s/%s:%s", s.registry, s.buildRepositoryPath(), namespace, name, filename)
}

func (s *OCIStorage) buildShasumReference(pt providerType, hostname, namespace, name, version string) string {
	filename := fmt.Sprintf("terraform-provider-%s_%s_SHA256SUMS", name, version)
	if pt == internalProviderType {
		return fmt.Sprintf("%s/%s/providers/%s/%s:%s", s.registry, s.buildRepositoryPath(), namespace, name, filename)
	}
	return fmt.Sprintf("%s/%s/mirror/providers/%s/%s/%s:%s", s.registry, s.buildRepositoryPath(), hostname, namespace, name, filename)
}

func (s *OCIStorage) buildShasumSignatureReference(pt providerType, hostname, namespace, name, version string) string {
	filename := fmt.Sprintf("terraform-provider-%s_%s_SHA256SUMS.sig", name, version)
	if pt == internalProviderType {
		return fmt.Sprintf("%s/%s/providers/%s/%s:%s", s.registry, s.buildRepositoryPath(), namespace, name, filename)
	}
	return fmt.Sprintf("%s/%s/mirror/providers/%s/%s/%s:%s", s.registry, s.buildRepositoryPath(), hostname, namespace, name, filename)
}

func (s *OCIStorage) buildSigningKeysReference(pt providerType, hostname, namespace string) string {
	if pt == internalProviderType {
		return fmt.Sprintf("%s/%s/providers/%s:signing-keys.json", s.registry, s.buildRepositoryPath(), namespace)
	}
	return fmt.Sprintf("%s/%s/mirror/providers/%s/%s:signing-keys.json", s.registry, s.buildRepositoryPath(), hostname, namespace)
}

func (s *OCIStorage) buildMirroredFileReference(hostname, namespace, name, filename string) string {
	return fmt.Sprintf("%s/%s/mirror/providers/%s/%s/%s:%s", s.registry, s.buildRepositoryPath(), hostname, namespace, name, filename)
}

func (s *OCIStorage) buildProviderArchiveFilename(name, version, os, arch string) string {
	return fmt.Sprintf("terraform-provider-%s_%s_%s_%s.zip", name, version, os, arch)
}

func (s *OCIStorage) buildRepositoryPath() string {
	if s.repositoryPrefix != "" {
		return fmt.Sprintf("%s/%s", s.repositoryPrefix, s.repository)
	}
	return s.repository
}

// OCIStorageOption provides additional options for the OCIStorage.
type OCIStorageOption func(*OCIStorage)

// WithOCIStorageRepositoryPrefix configures the OCI storage to work under a given repository prefix.
func WithOCIStorageRepositoryPrefix(prefix string) OCIStorageOption {
	return func(s *OCIStorage) {
		s.repositoryPrefix = prefix
	}
}

// WithOCIStorageCredentials configures the username and password for OCI registry authentication.
func WithOCIStorageCredentials(username, password string) OCIStorageOption {
	return func(s *OCIStorage) {
		s.username = username
		s.password = password
	}
}

// WithOCIArchiveFormat configures the module archive format (zip, tar, tgz, etc.)
func WithOCIArchiveFormat(archiveFormat string) OCIStorageOption {
	return func(s *OCIStorage) {
		s.moduleArchiveFormat = archiveFormat
	}
}

// WithOCIStorageSignedUrlExpiry configures the duration until the signed url expires
func WithOCIStorageSignedUrlExpiry(t time.Duration) OCIStorageOption {
	return func(s *OCIStorage) {
		s.signedURLExpiry = t
	}
}

// WithOCIStorageHTTPClient configures a custom HTTP client for the OCI storage
func WithOCIStorageHTTPClient(client *http.Client) OCIStorageOption {
	return func(s *OCIStorage) {
		s.httpClient = client
	}
}

// WithOCIUsername configures the OCI registry username for authentication.
func WithOCIUsername(username string) OCIStorageOption {
	return func(s *OCIStorage) {
		s.username = username
	}
}

// WithOCIPassword configures the OCI registry password for authentication.
func WithOCIPassword(password string) OCIStorageOption {
	return func(s *OCIStorage) {
		s.password = password
	}
}

// WithOCIToken configures the OCI registry token for authentication.
func WithOCIToken(token string) OCIStorageOption {
	return func(s *OCIStorage) {
		s.token = token
	}
}

// WithOCIInsecure configures whether to allow insecure connections.
func WithOCIInsecure(insecure bool) OCIStorageOption {
	return func(s *OCIStorage) {
		s.insecure = insecure
	}
}

// NewOCIStorage returns a fully initialized OCI storage.
func NewOCIStorage(ctx context.Context, registry, repository string, options ...OCIStorageOption) (Storage, error) {
	// Required- and default-values should be set here
	s := &OCIStorage{
		registry:            registry,
		repository:          repository,
		moduleArchiveFormat: DefaultModuleArchiveFormat,
		signedURLExpiry:     5 * time.Minute,
		httpClient:          &http.Client{Timeout: 30 * time.Second},
	}

	for _, option := range options {
		option(s)
	}

	// Create the OCI client
	client, err := s.createOCIClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI client: %w", err)
	}
	
	s.client = client

	return s, nil
}

// createOCIClient creates an OCI client implementation
func (s *OCIStorage) createOCIClient(ctx context.Context) (ociClientAPI, error) {
	return &ociClient{
		registry:   s.registry,
		httpClient: s.httpClient,
		username:   s.username,
		password:   s.password,
		token:      s.token,
		insecure:   s.insecure,
	}, nil
}

// ociClient is the concrete implementation of ociClientAPI
type ociClient struct {
	registry   string
	httpClient *http.Client
	username   string
	password   string
	token      string
	insecure   bool
}

func (c *ociClient) ArtifactExists(ctx context.Context, reference string) (bool, error) {
	// Parse the reference to get repository and tag
	repo, tag := c.parseReference(reference)
	
	// Check if manifest exists using OCI Distribution API
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", c.registry, repo, tag)
	
	req, err := http.NewRequestWithContext(ctx, "HEAD", manifestURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return true, nil
}

func (c *ociClient) DownloadArtifact(ctx context.Context, reference string) ([]byte, error) {
	// This is a simplified implementation
	// In a real implementation, you would:
	// 1. Get the manifest for the reference
	// 2. Extract blob references from the manifest
	// 3. Download the actual blob content
	return nil, fmt.Errorf("download not implemented in this demo")
}

func (c *ociClient) UploadArtifact(ctx context.Context, reference string, content io.Reader, overwrite bool) error {
	// This is a simplified implementation
	// In a real implementation, you would:
	// 1. Upload blob content
	// 2. Create and upload manifest
	return fmt.Errorf("upload not implemented in this demo")
}

func (c *ociClient) ListTags(ctx context.Context, repository string, callback func(tags []string) error) error {
	// List tags using OCI Distribution API
	tagsURL := fmt.Sprintf("https://%s/v2/%s/tags/list", c.registry, repository)
	
	req, err := http.NewRequestWithContext(ctx, "GET", tagsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	var result struct {
		Tags []string `json:"tags"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	return callback(result.Tags)
}

func (c *ociClient) GenerateDownloadURL(ctx context.Context, reference string) (string, error) {
	// For OCI registries, we typically return the reference itself or a proper download URL
	// This could be enhanced to generate proper registry download URLs based on the registry type
	return reference, nil
}

func (c *ociClient) parseReference(reference string) (repository, tag string) {
	// Simple reference parsing
	// Format: registry/repository:tag
	parts := strings.SplitN(reference, "/", 2)
	if len(parts) < 2 {
		return reference, "latest"
	}
	
	repoTag := parts[1]
	tagParts := strings.SplitN(repoTag, ":", 2)
	if len(tagParts) < 2 {
		return repoTag, "latest"
	}
	
	return tagParts[0], tagParts[1]
}
