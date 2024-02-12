package provider

import (
	"context"
	"io"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// Storage represents the Storage of Terraform providers.
type Storage interface {
	GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error)
	ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error)

	// UploadProviderReleaseFiles is used to upload all artifacts which make up a provider release
	// https://developer.hashicorp.com/terraform/registry/providers/publishing#manually-preparing-a-release
	UploadProviderReleaseFiles(ctx context.Context, namespace, name, filename string, file io.Reader) error

	// SigningKeys downloads and returns the keys for a given namespace from the configured storage backend
	SigningKeys(ctx context.Context, namespace string) (*core.SigningKeys, error)

	// MigrateProviders is needed for the migration from 0.7.0 to 0.8.0
	MigrateProviders(ctx context.Context, dryRun bool) error
}
