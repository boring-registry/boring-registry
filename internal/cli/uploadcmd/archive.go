package uploadcmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/pkg/errors"
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

func (c *Config) archiveModules(root string, registry module.Registry) error {
	var err error
	if c.UploadRecursive == true {
		err = filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if fi.Name() != moduleSpecFileName {
				return nil
			}
			return c.processModule(path, registry)
		})
	} else {
		err = c.processModule(filepath.Join(root, moduleSpecFileName), registry)
	}
	return err
}

func (c *Config) processModule(path string, registry module.Registry) error {
	spec, err := module.ParseFile(path)
	if err != nil {
		return err
	}

	level.Debug(c.Logger).Log(
		"msg", "parsed module spec",
		"path", path,
		"name", spec.Name(),
	)

	// Check if the module meets version constraints
	ok, err := c.meetsConstraints(spec)
	if err != nil {
		return err
	} else if !ok {
		// Skip the module, as it didn't pass the version constraints
		level.Info(c.Logger).Log("msg", "skipped as module doesn't meet version constraints", "name", spec.Name())

		return nil
	}

	ctx := context.Background()
	if res, err := registry.GetModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version); err == nil {
		if c.IgnoreExistingModule == true {
			level.Info(c.Logger).Log(
				"msg", "module already exists",
				"download_url", res.DownloadURL,
			)
			return nil
		} else {
			level.Error(c.Logger).Log(
				"msg", "module already exists",
				"download_url", res.DownloadURL,
			)
			return errors.New("module already exists")
		}
	}

	moduleRoot := filepath.Dir(path)

	buf, err := archiveModule(moduleRoot)
	if err != nil {
		return err
	}

	res, err := registry.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, buf)
	if err != nil {
		return err
	}

	level.Info(c.Logger).Log(
		"msg", "module successfully uploaded",
		"download_url", res.DownloadURL,
	)

	return nil

}

func archiveModule(root string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	// ensure the src actually exists before trying to tar it
	if _, err := os.Stat(root); err != nil {
		return buf, fmt.Errorf("Unable to tar files - %v", err.Error())
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

		// manually close here after each file operation; defering would cause each file close
		// to wait until all operations have completed.
		data.Close()

		return nil
	})

	return buf, err
}

// meetsConstraints checks whether a module version matches the version constraints - if there are any.
// Returns an unrecoverable error if there's an internal error. Otherwise it returns a boolean indicating if the module meets the constraints
func (c *Config) meetsConstraints(spec *module.Spec) (bool, error) {
	if c.VersionConstraints == nil {
		return true, nil
	}

	v, err := version.NewVersion(spec.Metadata.Version)
	if err != nil {
		return false, err
	}

	return c.VersionConstraints.Check(v), nil
}
