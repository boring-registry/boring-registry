package cmd

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/provider"
	"github.com/boring-registry/boring-registry/pkg/storage"

	"github.com/spf13/cobra"
)

const (
	flagFileSha256SumsName    = "filename-sha256sums"
	flagProviderNamespaceName = "namespace"
)

var (
	// upload provider flags
	flagFileSha256Sums       string
	flagProviderArchivePaths []string
	flagProviderNamespace    string
)

var (
	uploadProvider    = &uploadProviderRunner{}
	uploadProviderCmd = &cobra.Command{
		Use:          "provider PROVIDER",
		SilenceUsage: true,
		PreRunE:      uploadProvider.preRun,
		RunE:         uploadProvider.run,
	}
)

// The main idea of uploadProviderRunner is to have a struct that can be mocked more easily in tests
type uploadProviderRunner struct {
	storage          provider.Storage
	sha256Sums       func(string) (*core.Sha256Sums, error)
	uploader         func(context.Context, *core.Sha256Sums, string, string, string) error
	artifactUploader func(context.Context, string, string, string) error
}

// preRun sets up the storage backend before running the upload command
func (u *uploadProviderRunner) preRun(cmd *cobra.Command, args []string) error {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelCtx()
	storage, err := setupStorage(ctx)
	if err != nil {
		return fmt.Errorf("failed to set up storage: %w", err)
	}

	u.storage = storage
	u.sha256Sums = u.createSha256Sums
	u.uploader = u.uploadArtifacts
	u.artifactUploader = u.uploadProviderReleaseFile

	return nil
}

// Executes the provider upload process.
func (u *uploadProviderRunner) run(cmd *cobra.Command, args []string) error {
	sums, err := u.sha256Sums(flagFileSha256Sums)
	if err != nil {
		return err
	}

	ctx := context.Background()
	validateCtx, cancelValidateCtx := context.WithTimeout(ctx, 15*time.Second)
	defer cancelValidateCtx()
	signingKeys, err := u.storage.SigningKeys(validateCtx, flagProviderNamespace)
	if err != nil {
		return fmt.Errorf("failed to retrieve %s: %w", storage.SigningKeyFileName, err)
	}

	sumsBytes, err := os.ReadFile(flagFileSha256Sums)
	if err != nil {
		return fmt.Errorf("failed to read file at path %s: %w", flagFileSha256Sums, err)
	}

	// We expect the signature to be suffixed with the .sig extension
	// https://developer.hashicorp.com/terraform/registry/providers/publishing#manually-preparing-a-release
	p := fmt.Sprintf("%s.sig", flagFileSha256Sums)
	sumsSigBytes, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("failed to read file at path %s: %w", p, err)
	}

	if err := signingKeys.IsValidSha256Sums(sumsBytes, sumsSigBytes); err != nil {
		return err
	}

	providerName, err := sums.Name()
	if err != nil {
		return fmt.Errorf("failed to parse provider name: %v", err)
	}

	return u.uploader(ctx, sums, flagFileSha256Sums, flagProviderNamespace, providerName)
}

func (u *uploadProviderRunner) createSha256Sums(path string) (*core.Sha256Sums, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("file path is not absolute: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file at path %s: %w", path, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("failed to close file", slog.String("path", path), slog.String("error", err.Error()))
		}
	}()

	sums, err := core.NewSha256Sums(filepath.Base(path), f)
	if err != nil {
		return nil, err
	}

	if err := u.validateShaSums(sums); err != nil {
		return nil, err
	}

	return sums, nil
}

func (u *uploadProviderRunner) validateShaSums(sums *core.Sha256Sums) error {
	// Check whether the user has given archive paths to upload on the command line as flags.
	// If not, we try to determine the locations of the provider zip archives based on the path of the *_SHA256SUMS file and the filenames in that file
	if len(flagProviderArchivePaths) != 0 {
		if len(sums.Entries) != len(flagProviderArchivePaths) {
			return fmt.Errorf("the number of provided archive paths doesn't match the number of entries in %s", flagFileSha256Sums)
		}

		for _, archivePath := range flagProviderArchivePaths {
			fileName := filepath.Base(archivePath)
			checksum, exists := sums.Entries[fileName]
			if !exists {
				return fmt.Errorf("checksum for file %s is missing", fileName)
			}
			if err := u.validateShaSumsEntry(archivePath, checksum); err != nil {
				return fmt.Errorf("failed to validate checksum for file %s", fileName)
			}
		}
	} else {
		baseDir := filepath.Dir(flagFileSha256Sums)
		for fileName, checksum := range sums.Entries {
			if err := u.validateShaSumsEntry(filepath.Join(baseDir, fileName), checksum); err != nil {
				return fmt.Errorf("failed to validate checksum for file %s", fileName)
			}
		}
	}

	return nil
}

func (u *uploadProviderRunner) validateShaSumsEntry(path string, checksum []byte) error {
	binaryName := filepath.Base(path)
	if !regexp.MustCompile("^terraform-provider-.+_.+_.+.(zip|json)$").MatchString(binaryName) {
		return fmt.Errorf("provider binary %s file name is invalid", binaryName)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open provided archive file: %s", path)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("failed to close file", slog.String("path", path), slog.String("error", err.Error()))
		}
	}()

	c, err := core.Sha256Checksum(f)
	if err != nil {
		return err
	}

	if !bytes.Equal(checksum, c) {
		return fmt.Errorf("checksums don't match")
	}

	return nil
}

func (u *uploadProviderRunner) uploadProviderReleaseFile(ctx context.Context, path, namespace, name string) error {
	archiveFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := archiveFile.Close(); err != nil {
			slog.Warn("failed to close file", slog.String("path", path), slog.String("error", err.Error()))
		}
	}()

	uploadCtx, uploadCtxCancel := context.WithTimeout(ctx, 120*time.Second)
	defer uploadCtxCancel()

	fileName := filepath.Base(path)
	return u.storage.UploadProviderReleaseFiles(uploadCtx, namespace, name, fileName, archiveFile)
}

func (u *uploadProviderRunner) uploadArtifacts(ctx context.Context, sums *core.Sha256Sums, archivePath, namespace, name string) error {
	// Upload provider binary .zip archives
	if len(flagProviderArchivePaths) > 0 {
		for _, archivePath := range flagProviderArchivePaths {
			if err := u.artifactUploader(ctx, archivePath, flagProviderNamespace, name); err != nil {
				return err
			}
			slog.Info("successfully published provider binary", slog.String("name", filepath.Base(archivePath)))
		}
	} else {
		baseDir := filepath.Dir(flagFileSha256Sums)
		for fileName := range sums.Entries {
			archivePath := filepath.Join(baseDir, fileName)
			if err := u.artifactUploader(ctx, archivePath, flagProviderNamespace, name); err != nil {
				return err
			}
			slog.Info("successfully published provider binary", slog.String("name", fileName))
		}
	}

	// Upload *_SHA256SUMS file
	if err := u.artifactUploader(ctx, flagFileSha256Sums, flagProviderNamespace, name); err != nil {
		return err
	}
	slog.Info("successfully published provider SHA256SUMS file", slog.String("name", filepath.Base(flagFileSha256Sums)))

	// Upload *_SHA256SUMS.sig file
	signaturePath := fmt.Sprintf("%s.sig", flagFileSha256Sums)
	if err := u.artifactUploader(ctx, signaturePath, flagProviderNamespace, name); err != nil {
		return err
	}
	slog.Info("successfully published provider SHA256SUMS.sig file", slog.String("name", filepath.Base(signaturePath)))

	return nil
}
