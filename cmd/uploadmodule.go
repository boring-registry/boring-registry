package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
)

const (
	moduleSpecFileName = "boring-registry.hcl"
)

var (
	flagRecursive                bool
	flagIgnoreExistingModule     bool
	flagVersionConstraintsRegex  string
	flagVersionConstraintsSemver string
	flagModuleVersion            string
)

var (
	versionConstraintsRegex  *regexp.Regexp
	versionConstraintsSemver version.Constraints
	moduleVersion            *version.Version
)

var (
	moduleUploader  = &moduleUploadRunner{}
	uploadModuleCmd = &cobra.Command{
		Use:          "module MODULE",
		SilenceUsage: true,
		PreRunE:      moduleUploader.preRun,
		RunE:         moduleUploader.run,
	}
)

func init() {
	uploadModuleCmd.PersistentFlags().StringVar(&flagModuleVersion, "version", "", "Specify the version of the module to upload. Mutually exclusive with --recursive module discovery.")
}

// The main idea of moduleUploadRunner is to have a struct that can be mocked more easily in tests
type moduleUploadRunner struct {
	storage  module.Storage
	discover func(string) error
	archive  func(string) (io.Reader, error)
	process  func(string) error
}

// preRun sets up the storage backend before running the upload command
func (m *moduleUploadRunner) preRun(cmd *cobra.Command, args []string) error {
	storage, err := setupStorage(context.Background())
	if err != nil {
		return fmt.Errorf("failed to set up storage: %w", err)
	}
	m.storage = storage
	m.discover = m.walkModules
	m.archive = archiveModule
	m.process = m.processModule
	return nil
}

func (m *moduleUploadRunner) run(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing path argument to module directory")
	} else if len(args) > 1 {
		return fmt.Errorf("only a single module path argument is supported at a time")
	}

	if _, err := os.Stat(args[0]); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to locate module directory: %w", err)
	}

	// Validate the semver version constraints
	if flagVersionConstraintsSemver != "" {
		constraints, err := version.NewConstraint(flagVersionConstraintsSemver)
		if err != nil {
			return fmt.Errorf("failed to upload module: %w", err)
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

	if flagModuleVersion != "" {
		var err error
		moduleVersion, err = version.NewSemver(flagModuleVersion)
		if err != nil {
			return fmt.Errorf("failed to validate version %v: %w", flagModuleVersion, err)
		}
	}

	if flagRecursive && moduleVersion != nil {
		return errors.New("providing a module version is not supported when traversing recursively. You can only provide one of the two options")
	}

	return m.discover(args[0])
}

func (m *moduleUploadRunner) walkModules(root string) error {
	modulePaths := []string{} // holds paths to all discovered module spec files
	if flagRecursive {
		if err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("walk-related error: %w", err)
			}

			if fi.Name() != moduleSpecFileName {
				return nil
			}
			modulePaths = append(modulePaths, path)
			return nil
		}); err != nil {
			return err
		}
	} else {
		modulePaths = append(modulePaths, filepath.Join(root, moduleSpecFileName))
	}

	for _, path := range modulePaths {
		if processErr := m.process(path); processErr != nil {
			return fmt.Errorf("failed to process module at %s: %w", path, processErr)
		}
	}
	return nil
}

func (m *moduleUploadRunner) processModule(path string) error {
	spec, err := module.ParseFile(path)
	if err != nil {
		return err
	}

	if moduleVersion == nil {
		err = spec.ValidateWithVersion()
	} else {
		err = spec.ValidateWithoutVersion()
	}
	if err != nil {
		return fmt.Errorf("module specification at path %s failed validation: %w", path, err)
	}

	// The user can pass a flag that sets the version of the module.
	// In that case, recursive traversal/discovery is not allowed and the boring-registry.hcl file does not contain
	// the metadata.version attribute.
	if moduleVersion != nil {
		spec.Metadata.Version = moduleVersion.String()
	}

	slog.Debug("parsed module spec", slog.String("path", path), slog.String("name", spec.Name()))

	// Check if the module meets version constraints
	if versionConstraintsSemver != nil {
		ok, err := meetsSemverConstraints(spec)
		if err != nil {
			return err
		} else if !ok {
			// Skip the module, as it didn't pass the version constraints
			slog.Info("module doesn't meet semver version constraints, skipped", slog.String("name", spec.Name()), slog.String("version", spec.Metadata.Version))
			return nil
		}
	}

	if versionConstraintsRegex != nil {
		if !meetsRegexConstraints(spec) {
			// Skip the module, as it didn't pass the regex version constraints
			slog.Info("module doesn't meet regex version constraints, skipped", slog.String("name", spec.Name()))
			return nil
		}
	}

	ctx := context.Background()
	providerAttrs := []any{
		slog.String("namespace", spec.Metadata.Namespace),
		slog.String("name", spec.Metadata.Name),
		slog.String("provider", spec.Metadata.Provider),
		slog.String("version", spec.Metadata.Version),
	}
	if _, err := m.storage.GetModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version); err != nil {
		if !errors.Is(err, module.ErrModuleNotFound) {
			slog.Error("failed to check if module exists in storage provider", append(providerAttrs, slog.Any("error", err))...)
			return fmt.Errorf("failed to check if module exists: %w", err)
		}
	} else {
		if flagIgnoreExistingModule {
			// We ignore the ErrModuleNotFound error, as it means the module version doesn't exist yet and we can proceed to upload it
			slog.Info("module already exists", providerAttrs...)
			return nil
		} else {
			slog.Error("module already exists", providerAttrs...)
			return fmt.Errorf("module version %s already exists", spec.Metadata.Version)
		}
	}

	moduleRoot := filepath.Dir(path)

	buf, err := m.archive(moduleRoot)
	if err != nil {
		return err
	}

	if _, err := m.storage.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, buf); err != nil {
		return err
	}

	slog.Info("module successfully uploaded", providerAttrs...)
	return nil

}

func archiveModule(root string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(root); err != nil {
		return buf, fmt.Errorf("unable to tar files: %w", err)
	}

	gw := gzip.NewWriter(buf)
	defer func() {
		if err := gw.Close(); err != nil {
			slog.Error("failed to close gzip writer", slog.String("module-root", root), slog.Any("error", err))
		}
	}()

	tw := tar.NewWriter(gw)
	defer func() {
		if err := tw.Close(); err != nil {
			slog.Error("failed to close tar writer", slog.String("module-root", root), slog.Any("error", err))
		}
	}()

	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		// return on any error
		if err != nil {
			return err
		}

		// return on non-regular files
		if !fi.Mode().IsRegular() {
			return nil
		}

		// create a new dir/file header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// update the name to correctly reflect the desired destination when untaring
		header.Name = archiveFileHeaderName(path, root)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		data, err := os.Open(path)
		if err != nil {
			return err
		}

		if _, err := io.Copy(tw, data); err != nil {
			return err
		}

		// manually close here after each file operation; deferring would cause each file close
		// to wait until all operations have completed.
		if err := data.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %w", path, err)
		}

		return nil
	})

	return buf, err
}

// meetsSemverConstraints checks whether a module version matches the semver version constraints.
// Returns an unrecoverable error if there's an internal error.
// Otherwise, it returns a boolean indicating if the module meets the constraints
func meetsSemverConstraints(spec *module.Spec) (bool, error) {
	v, err := version.NewSemver(spec.Metadata.Version)
	if err != nil {
		return false, err
	}

	return versionConstraintsSemver.Check(v), nil
}

// meetsRegexConstraints checks whether a module version matches the regex.
// Returns a boolean indicating if the module meets the constraints
func meetsRegexConstraints(spec *module.Spec) bool {
	return versionConstraintsRegex.MatchString(spec.Metadata.Version)
}

func archiveFileHeaderName(path, root string) string {
	// Check if the module is uploaded non-recursively from the current directory
	if root == "." {
		return path
	}

	// Remove the root prefix from the path
	if strings.HasPrefix(path, root) {
		relativePath := strings.TrimPrefix(path, root)

		// the leading slash needs to be removed
		return strings.TrimPrefix(relativePath, "/")
	}

	return path
}
