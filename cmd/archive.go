package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TierMobility/boring-registry/pkg/module"

	"github.com/go-kit/kit/log/level"
	"github.com/hashicorp/go-version"
)

const (
	moduleSpecFileName = "boring-registry.hcl"
)

func archiveModules(root string, storage module.Storage) error {
	var err error
	if flagRecursive {
		err = filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if fi.Name() != moduleSpecFileName {
				return nil
			}
			return processModule(path, storage)
		})
	} else {
		err = processModule(filepath.Join(root, moduleSpecFileName), storage)
	}
	return err
}

func processModule(path string, storage module.Storage) error {
	spec, err := module.ParseFile(path)
	if err != nil {
		return fmt.Errorf("parse module file failed, error: %w", err)
	}

	_ = level.Debug(logger).Log(
		"msg", "parsed module spec",
		"path", path,
		"name", spec.Name(),
	)

	// Check if the module meets version constraints
	if versionConstraintsSemver != nil {
		ok, err := meetsSemverConstraints(spec)
		if err != nil {
			return err
		} else if !ok {
			// Skip the module, as it didn't pass the version constraints
			_ = level.Info(logger).Log("msg", "module doesn't meet semver version constraints, skipped", "name", spec.Name())
			return nil
		}
	}

	if versionConstraintsRegex != nil {
		if !meetsRegexConstraints(spec) {
			// Skip the module, as it didn't pass the regex version constraints
			_ = level.Info(logger).Log("msg", "module doesn't meet regex version constraints, skipped", "name", spec.Name())
			return nil
		}
	}

	ctx := context.Background()
	if res, err := storage.GetModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version); err == nil {
		if flagIgnoreExistingModule {
			_ = level.Info(logger).Log(
				"msg", "module already exists",
				"download_url", res.DownloadURL,
			)
			return nil
		} else {
			_ = level.Error(logger).Log(
				"msg", "module already exists",
				"download_url", res.DownloadURL,
			)
			return fmt.Errorf("module already exists")
		}
	}

	moduleRoot := filepath.Dir(path)
	buf, err := archiveModule(moduleRoot)
	if err != nil {
		return err
	}

	res, err := storage.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, buf)
	if err != nil {
		return err
	}

	_ = level.Info(logger).Log(
		"msg", "module successfully uploaded",
		"download_url", res.DownloadURL,
	)

	return nil

}

func archiveModule(root string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(root); err != nil {
		return buf, fmt.Errorf("unable to tar files - %v", err.Error())
	}

	gw := gzip.NewWriter(buf)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

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
		header.Name = strings.TrimPrefix(strings.Replace(path, root, "", -1), string(filepath.Separator))

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
		data.Close()

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
