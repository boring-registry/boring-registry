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
			return c.doUpload(path, registry)
		})
	} else {
		err = c.doUpload(filepath.Join(root, moduleSpecFileName), registry)
	}
	return err
}

func (c *Config) doUpload(path string, registry module.Registry) error {
	spec, err := module.ParseFile(path)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("%s/%s/%s/%s",
		spec.Metadata.Namespace, spec.Metadata.Name,
		spec.Metadata.Provider, spec.Metadata.Version,
	)

	level.Debug(c.Logger).Log(
		"msg", "parsed module spec",
		"path", path,
		"name", name,
	)

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
