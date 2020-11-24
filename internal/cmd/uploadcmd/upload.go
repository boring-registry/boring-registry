package uploadcmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TierMobility/boring-registry/internal/cmd/help"
	"github.com/TierMobility/boring-registry/internal/cmd/rootcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log/level"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
)

const (
	moduleSpecFileName = "boring-registry.hcl"
)

type Config struct {
	rootConfig *rootcmd.Config
	out        io.Writer
}

func New(rootConfig *rootcmd.Config, out io.Writer) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
		out:        out,
	}

	fs := flag.NewFlagSet("boring-registry upload", flag.ExitOnError)
	rootConfig.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "upload [flags] <dir>",
		ShortHelp:  "uploads terraform modules to a registry",
		LongHelp: help.FormatHelp(`Upload modules to a registry.

The upload command expects some configuration, such as which registry type to use and which local directory to work in.
The default registry type is "s3" and is currently the only registry type available.
For more options see the available options below.

EXAMPLE USAGE

boring-registry upload \
  -registry=s3 \
  -registry.s3.bucket=my-bucket terraform/modules
		`),
		FlagSet: fs,
		Exec:    cfg.Exec,
	}
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("create requires at least 1 args")
	}

	level.Info(c.rootConfig.Logger).Log(
		"msg", "starting uploader",
	)

	err := filepath.Walk(args[0], func(path string, fi os.FileInfo, err error) error {
		if fi == nil {
			return errors.New("missing file or directory")
		}

		if fi.Name() == moduleSpecFileName {
			spec, err := module.ParseFile(path)
			if err != nil {
				return err
			}

			body, err := archive(filepath.Dir(path))
			if err != nil {
				return err
			}

			res, err := c.rootConfig.Registry.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, body)
			if err != nil {
				if errors.Cause(err) == module.ErrAlreadyExists {
					level.Debug(c.rootConfig.Logger).Log(
						"msg", "skipping already existing module",
						"err", err,
					)
				} else {
					return err
				}
			} else {
				level.Info(c.rootConfig.Logger).Log(
					"msg", "successfully uploaded module",
					"module", res,
				)
			}
		}
		return nil
	})

	return err
}

func archive(dir string) (io.Reader, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}

	var baseDir string
	if info.IsDir() && info.Name() != "." {
		baseDir = filepath.Base(dir)
	}

	buf := new(bytes.Buffer)

	gw := gzip.NewWriter(buf)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.Walk(dir, func(file string, fi os.FileInfo, err error) error {
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(file, dir))
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}

			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return buf, nil
}
