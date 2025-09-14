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

	"github.com/spf13/cobra"

	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/hashicorp/go-version"
)

const (
	moduleSpecFileName = "boring-registry.hcl"
)

var (
	flagRecursive                bool
	flagIgnoreExistingModule     bool
	flagVersionConstraintsRegex  string
	flagVersionConstraintsSemver string
)

var (
	versionConstraintsRegex  *regexp.Regexp
	versionConstraintsSemver version.Constraints
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

// The main idea of moduleUploadRunner is to have a struct that can be mocked more easily in tests
type moduleUploadRunner struct {
	storage  module.Storage
	discover func(string, module.Storage) error
	archive  func(string) (io.Reader, error)
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
	return nil
}

func (m *moduleUploadRunner) run(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing path to module directory")
	} else if len(args) > 1 {
		return fmt.Errorf("only a single module is supported at a time")
	}

	if _, err := os.Stat(args[0]); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to locate module directory: %w", err)
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

	return m.discover(args[0], m.storage)
}

func (m *moduleUploadRunner) walkModules(root string, storage module.Storage) error {
	if flagRecursive {
		err := filepath.Walk(root, func(path string, fi os.FileInfo, _ error) error {
			// FYI we conciously ignore all walk-related errors

			if fi.Name() != moduleSpecFileName {
				return nil
			}
			if processErr := m.processModule(path, storage); processErr != nil {
				return fmt.Errorf("failed to process module at %s:\n%w", path, processErr)
			}

			return nil
		})
		return err
	}

	path := filepath.Join(root, moduleSpecFileName)
	if processErr := m.processModule(path, storage); processErr != nil {
		return fmt.Errorf("failed to process module at %s:\n%w", path, processErr)
	}
	return nil
}

func (m *moduleUploadRunner) processModule(path string, storage module.Storage) error {
	spec, err := module.ParseFile(path)
	if err != nil {
		return err
	}

	slog.Debug("parsed module spec", slog.String("path", path), slog.String("name", spec.Name()))

	// Check if the module meets version constraints
	if versionConstraintsSemver != nil {
		ok, err := meetsSemverConstraints(spec)
		if err != nil {
			return err
		} else if !ok {
			// Skip the module, as it didn't pass the version constraints
			slog.Info("module doesn't meet semver version constraints, skipped", slog.String("name", spec.Name()))
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
	if _, err := storage.GetModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version); err == nil {
		if flagIgnoreExistingModule {
			slog.Info("module already exists", providerAttrs...)
			return nil
		} else {
			slog.Error("module already exists", providerAttrs...)
			return errors.New("module already exists")
		}
	}

	moduleRoot := filepath.Dir(path)

	buf, err := m.archive(moduleRoot)
	if err != nil {
		return err
	}

	if _, err := storage.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, buf); err != nil {
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
