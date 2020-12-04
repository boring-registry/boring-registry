package uploadcmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TierMobility/boring-registry/internal/cli/help"
	"github.com/TierMobility/boring-registry/internal/cli/rootcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/fatih/color"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/slok/gospinner"
)

const (
	moduleSpecFileName = "boring-registry.hcl"
)

type Config struct {
	*rootcmd.Config

	RegistryType string
	S3Bucket     string
	S3Prefix     string
	S3Region     string

	APIKey                 string
	ListenAddress          string
	TelemetryListenAddress string
}

func (c *Config) Exec(ctx context.Context, args []string) error {

	if len(args) < 1 {
		return errors.New("upload requires at least 1 args")
	}

	var registry module.Registry

	switch c.RegistryType {
	case "s3":
		if c.S3Bucket == "" {
			return errors.New("missing flag -s3-bucket")
		}

		reg, err := module.NewS3Registry(c.S3Bucket,
			module.WithS3RegistryBucketPrefix(c.S3Prefix),
			module.WithS3RegistryBucketRegion(c.S3Region),
		)
		if err != nil {
			return err
		}
		registry = reg
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

			name := fmt.Sprintf("%s/%s/%s/%s", spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version)

			spinner, _ := gospinner.NewSpinnerWithColor(gospinner.Dots, gospinner.FgHiCyan)
			spinner.Start(color.New(color.Bold).Sprintf("Processing module: %s", name))

			if res, err := registry.GetModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version); err == nil {
				spinner.FinishWithMessage(color.New(color.Bold).Sprint("⚠"), fmt.Sprintf("Module already exists: %s", res.DownloadURL))
				return nil
			}

			body, err := archive(filepath.Dir(path))
			if err != nil {
				spinner.Fail()
				return err
			}

			res, err := registry.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, body)
			if err != nil {
				spinner.Fail()
				return err
			}

			spinner.FinishWithMessage(color.New(color.Bold).Sprint("✔"), fmt.Sprintf("Module successfully uploaded to: %s", res.DownloadURL))
		}

		return nil
	})

	return err
}

func New(config *rootcmd.Config) *ffcli.Command {
	cfg := &Config{
		Config: config,
	}

	fs := flag.NewFlagSet("boring-registry upload", flag.ExitOnError)
	fs.StringVar(&cfg.RegistryType, "type", "", "Registry type to use (currently only \"s3\" is supported)")
	fs.StringVar(&cfg.S3Bucket, "s3-bucket", "", "Bucket to use when using the S3 registry type")
	fs.StringVar(&cfg.S3Prefix, "s3-prefix", "/", "Prefix to use when using the S3 registry type")
	fs.StringVar(&cfg.S3Region, "s3-region", "", "Region of the S3 bucket when using the S3 registry type")
	config.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		UsageFunc:  help.UsageFunc,
		ShortUsage: "boring-registry upload [flags] <dir>",
		ShortHelp:  "Uploads modules to a registry.",
		FlagSet:    fs,
		Options:    []ff.Option{ff.WithEnvVarPrefix(help.EnvVarPrefix)},
		LongHelp: fmt.Sprint(`  Uploads modules to a registry.

  This command requires some configuration, 
  such as which registry type to use and a directory to search for modules.

  The upload command walks the directory recursively and looks
  for modules with a boring-registry.hcl file in it. The file is then parsed
  to get the module metadata the module is then archived and uploaded to the given registry.

  Example Usage: boring-registry upload -type=s3 -s3-bucket=example-bucket modules/

  For more options see the available options below.`),
		Exec: cfg.Exec,
	}
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
