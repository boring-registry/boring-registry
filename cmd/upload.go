package cmd

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-kit/log/level"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/TierMobility/boring-registry/pkg/provider"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	flagFileSha256SumsName    = "filename-sha256sums"
	flagProviderNamespaceName = "namespace"
)

var (
	flagRecursive                bool
	flagIgnoreExistingModule     bool
	flagVersionConstraintsRegex  string
	flagVersionConstraintsSemver string

	// upload provider flags
	flagFileSha256Sums       string
	flagProviderArchivePaths []string
	flagProviderNamespace    string
)

var (
	versionConstraintsRegex  *regexp.Regexp
	versionConstraintsSemver version.Constraints
)

func init() {
	rootCmd.AddCommand(uploadCmd)

	uploadProviderCmd.Flags().StringVar(&flagFileSha256Sums, flagFileSha256SumsName, "", "The absolute path to the *_SHA256SUMS file")
	uploadProviderCmd.Flags().StringSliceVar(&flagProviderArchivePaths, "filenames-provider-archives", []string{}, "A list of file paths to provider ZIP archives")
	uploadProviderCmd.Flags().StringVar(&flagProviderNamespace, flagProviderNamespaceName, "", "The namespace under which the provider will be uploaded")
	for _, f := range []string{flagFileSha256SumsName, flagProviderNamespaceName} {
		if err := uploadProviderCmd.MarkFlagRequired(f); err != nil {
			panic(fmt.Errorf("failed to mark flag %s as required: %w", f, err))
		}
	}
	uploadCmd.AddCommand(uploadModuleCmd, uploadProviderCmd)

	uploadCmd.Flags().BoolVar(&flagRecursive, "recursive", true, "Recursively traverse <dir> and upload all modules in subdirectories")
	uploadCmd.Flags().BoolVar(&flagIgnoreExistingModule, "ignore-existing", true, "Ignore already existing modules. If set to false upload will fail immediately if a module already exists in that version")
	uploadCmd.Flags().StringVar(&flagVersionConstraintsRegex, "version-constraints-regex", "", `Limit the module versions that are eligible for upload with a regex that a version has to match.
Can be combined with the -version-constraints-semver flag`)
	uploadCmd.Flags().StringVar(&flagVersionConstraintsSemver, "version-constraints-semver", "", `Limit the module versions that are eligible for upload with version constraints.
The version string has to be formatted as a string literal containing one or more conditions, which are separated by commas.
Can be combined with the -version-constrained-regex flag`)
}

// uploadCmd uploads modules for legacy reasons.
// It is recommended to use `upload module` instead.
// This will eventually be deprecated and replaced.
var uploadCmd = &cobra.Command{
	Use:          "upload [flags] MODULE",
	Short:        "Upload modules and providers",
	SilenceUsage: true,
	RunE:         uploadModule,
}

var uploadModuleCmd = &cobra.Command{
	Use:          "module MODULE",
	SilenceUsage: true,
	RunE:         uploadModule,
}

var uploadProviderCmd = &cobra.Command{
	Use:          "provider PROVIDER",
	SilenceUsage: true,
	RunE:         uploadProvider,
}

func uploadModule(cmd *cobra.Command, args []string) error {
	storageBackend, err := setupStorage(context.Background(), "upload")
	if err != nil {
		return fmt.Errorf("failed to set up storage: %w", err)
	}

	if len(args) == 0 {
		return fmt.Errorf("missing argument")
	}

	if _, err := os.Stat(args[0]); errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Validate the semver version constraints
	if flagVersionConstraintsSemver != "" {
		constraints, err := version.NewConstraint(flagVersionConstraintsSemver)
		if err != nil {
			return err
		}
		versionConstraintsSemver = constraints
	}

	// Validate the regex version constraints
	if flagVersionConstraintsRegex != "" {
		constraints, err := regexp.Compile(flagVersionConstraintsRegex)
		if err != nil {
			return fmt.Errorf("invalid regex given: %v", err)
		}
		versionConstraintsRegex = constraints
	}

	return archiveModules(args[0], storageBackend)
}

func uploadProvider(cmd *cobra.Command, args []string) error {
	if !filepath.IsAbs(flagFileSha256Sums) {
		return fmt.Errorf("file path is not absolute: %s", flagFileSha256Sums)
	}

	f, err := os.Open(flagFileSha256Sums)
	defer f.Close()
	if err != nil {
		return fmt.Errorf("failed to open file at path %s: %w", flagFileSha256Sums, err)
	}

	sums, err := core.NewSha256Sums(filepath.Base(flagFileSha256Sums), f)
	if err != nil {
		return err
	}

	if err := validateShaSums(sums); err != nil {
		return err
	}

	ctx := context.Background()
	setupCtx, cancelSetupCtx := context.WithTimeout(ctx, 15*time.Second)
	defer cancelSetupCtx()
	storageBackend, err := setupStorage(setupCtx, "upload")
	if err != nil {
		return fmt.Errorf("failed to set up storage: %w", err)
	}

	validateCtx, cancelValidateCtx := context.WithTimeout(ctx, 15*time.Second)
	defer cancelValidateCtx()
	signingKeys, err := storageBackend.SigningKeys(validateCtx, flagProviderNamespace)
	if err != nil {
		return err
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

	// Upload provider binary .zip archives
	if len(flagProviderArchivePaths) > 0 {
		for _, archivePath := range flagProviderArchivePaths {
			if err := uploadProviderReleaseFile(ctx, storageBackend, archivePath, flagProviderNamespace, providerName); err != nil {
				return err
			}
			_ = level.Info(logger).Log(
				"msg", "successfully published provider binary",
				"name", filepath.Base(archivePath),
			)
		}
	} else {
		baseDir := filepath.Dir(flagFileSha256Sums)
		for _, entry := range sums.Entries {
			archivePath := filepath.Join(baseDir, entry.FileName)
			if err := uploadProviderReleaseFile(ctx, storageBackend, archivePath, flagProviderNamespace, providerName); err != nil {
				return err
			}
			_ = level.Info(logger).Log(
				"msg", "successfully published provider binary",
				"name", entry.FileName,
			)
		}
	}

	// Upload *_SHA256SUMS file
	if err = uploadProviderReleaseFile(ctx, storageBackend, flagFileSha256Sums, flagProviderNamespace, providerName); err != nil {
		return err
	}
	_ = level.Info(logger).Log(
		"msg", "successfully published provider SHA256SUMS file",
		"name", filepath.Base(flagFileSha256Sums),
	)

	// Upload *_SHA256SUMS.sig file
	signaturePath := fmt.Sprintf("%s.sig", flagFileSha256Sums)
	if err = uploadProviderReleaseFile(ctx, storageBackend, signaturePath, flagProviderNamespace, providerName); err != nil {
		return err
	}
	_ = level.Info(logger).Log(
		"msg", "successfully published provider SHA256SUMS.sig file",
		"name", filepath.Base(signaturePath),
	)

	return nil
}

func validateShaSums(sums *core.Sha256Sums) error {
	// Check whether the user has given archive paths to upload on the command line as flags.
	// If not, we try to determine the locations of the provider zip archives based on the path of the *_SHA256SUMS file and the filenames in that file
	if len(flagProviderArchivePaths) != 0 {
		if len(sums.Entries) != len(flagProviderArchivePaths) {
			return fmt.Errorf("the number of provided archive paths doesn't match the number of entries in %s", flagFileSha256Sums)
		}

		for _, archivePath := range flagProviderArchivePaths {
			fileName := filepath.Base(archivePath)
			for _, l := range sums.Entries {
				if fileName == l.FileName {
					if err := validateShaSumsEntry(archivePath, l.Sum); err != nil {
						return fmt.Errorf("failed to validate checksum for file %s", l.FileName)
					}

					break
				}
			}
			return fmt.Errorf("failed to find entry for %s in file %s", fileName, flagFileSha256Sums)
		}
	} else {
		baseDir := filepath.Dir(flagFileSha256Sums)
		for _, l := range sums.Entries {
			if err := validateShaSumsEntry(filepath.Join(baseDir, l.FileName), l.Sum); err != nil {
				return fmt.Errorf("failed to validate checksum for file %s", l.FileName)
			}
		}
	}

	return nil
}

func validateShaSumsEntry(path string, checksum []byte) error {
	binaryName := filepath.Base(path)
	if !regexp.MustCompile("^terraform-provider-.+_.+_.+.zip$").MatchString(binaryName) {
		return fmt.Errorf("provider binary %s file name is invalid", binaryName)
	}

	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		return fmt.Errorf("failed to open provided archive file: %s", path)
	}

	c, err := core.Sha256Checksum(f)
	if err != nil {
		return err
	}

	if !bytes.Equal(checksum, c) {
		return fmt.Errorf("checksums don't match")
	}

	return nil
}

func uploadProviderReleaseFile(ctx context.Context, storage provider.Storage, path, namespace, name string) error {
	archiveFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer archiveFile.Close()

	uploadCtx, uploadCtxCancel := context.WithTimeout(ctx, 120*time.Second)
	defer uploadCtxCancel()

	fileName := filepath.Base(path)
	return storage.UploadProviderReleaseFiles(uploadCtx, namespace, name, fileName, archiveFile)
}
