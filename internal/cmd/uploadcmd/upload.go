package uploadcmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TierMobility/boring-registry/internal/cmd/help"
	"github.com/TierMobility/boring-registry/internal/cmd/rootcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
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
		UsageFunc:  help.UsageFunc,
		ShortUsage: "upload [flags] <dir>",
		ShortHelp:  "uploads terraform modules to a registry",
		LongHelp: help.Format(`Upload modules to a registry.

The upload command expects some configuration, such as which registry type to use and which local directory to work in.
The default registry type is "s3" and is currently the only registry type available.
For more options see the available options below.

EXAMPLE USAGE

boring-registry upload -type=s3 -s3-bucket=my-bucket terraform/modules
		`),
		FlagSet: fs,
		Exec:    cfg.Exec,
	}
}

func (c *Config) printConfig() {
	c.rootConfig.UI.Output("==> Boring Registry upload configuration:")
	c.rootConfig.UI.Output("")
	c.rootConfig.UI.Output(fmt.Sprintf("    Registry: %s", c.rootConfig.Type))

	if c.rootConfig.Type == "s3" {
		c.rootConfig.UI.Output(fmt.Sprintf("    Bucket: %s", c.rootConfig.S3Bucket))
		if c.rootConfig.S3Prefix != "" {
			c.rootConfig.UI.Output(fmt.Sprintf("    Prefix: %s", c.rootConfig.S3Prefix))
		} else {
			c.rootConfig.UI.Output("    Prefix: /")
		}
	}

	c.rootConfig.UI.Output("")
	c.rootConfig.UI.Output("==> Boring Registry upload started! Log data will stream below:")
	c.rootConfig.UI.Output("")
	c.rootConfig.UI.Output("")
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	c.printConfig()

	if len(args) < 1 {
		return errors.New("create requires at least 1 args")
	}

	err := filepath.Walk(args[0], func(path string, fi os.FileInfo, err error) error {
		if fi == nil {
			return errors.New("missing file or directory")
		}

		if fi.Name() == moduleSpecFileName {
			spec, err := module.ParseFile(path)
			if err != nil {
				return err
			}

			// id := fmt.Sprintf("%s/%s/%s/%s", spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version)

			body, err := archive(filepath.Dir(path))
			if err != nil {
				return err
			}

			if _, err := c.rootConfig.Registry.GetModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version); err == nil {
				// fmt.Println(help.Info(fmt.Sprintf("Skipping already uploaded module: %s", res.DownloadURL)))
				return nil
			}

			_, err = c.rootConfig.Registry.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, body)
			if err != nil {
				return err
			}

			// fmt.Println(help.Success(fmt.Sprintf("Successfully uploaded module: %s", res.DownloadURL)))
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
