package uploadcmd

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
)

const (
	moduleSpecFileName = "boring-registry.hcl"
)

func (c *Config) archiveModules(root string, registry module.Registry) error {
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if fi.Name() != moduleSpecFileName {
			return nil
		}

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
			level.Info(c.Logger).Log(
				"msg", "module already exists",
				"download_url", res.DownloadURL,
			)

			return nil
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
	})

	return err
}

func archiveModule(root string) (io.Reader, error) {
	buf := new(bytes.Buffer)

	gw := gzip.NewWriter(buf)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, path)
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(path, root+"/")

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

		return nil
	})

	return buf, err
}
