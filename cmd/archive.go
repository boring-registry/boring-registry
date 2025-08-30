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
	"strings"

	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/hashicorp/go-version"
)

const (
	moduleSpecFileName = "boring-registry.hcl"
)

func archiveModules(root string, storage module.Storage) error {
	if flagRecursive {
		err := filepath.Walk(root, func(path string, fi os.FileInfo, _ error) error {
			// FYI we conciously ignore all walk-related errors

			if fi.Name() != moduleSpecFileName {
				return nil
			}
			if processErr := processModule(path, storage); processErr != nil {
				return fmt.Errorf("failed to process module at %s:\n%w", path, processErr)
			}

			return nil
		})
		return err
	}

	path := filepath.Join(root, moduleSpecFileName)
	if processErr := processModule(path, storage); processErr != nil {
		return fmt.Errorf("failed to process module at %s:\n%w", path, processErr)
	}
	return nil
}

func processModule(path string, storage module.Storage) error {
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
	res, err := storage.GetModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version)
	providerAttrs := []any{
		slog.String("namespace", res.Namespace),
		slog.String("name", res.Name),
		slog.String("provider", res.Provider),
		slog.String("version", res.Version),
	}
	if err == nil {
		if flagIgnoreExistingModule {
			slog.Info("module already exists", providerAttrs...)
			return nil
		} else {
			slog.Error("module already exists", providerAttrs...)
			return errors.New("module already exists")
		}
	}

	moduleRoot := filepath.Dir(path)

	buf, err := archiveModule(moduleRoot)
	if err != nil {
		return err
	}

	res, err = storage.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, buf)
	if err != nil {
		return err
	}

	slog.Info("module successfully uploaded", providerAttrs...)
	return nil

}

func archiveModule(root string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(root); err != nil {
		return buf, fmt.Errorf("unable to tar files - %v", err.Error())
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
