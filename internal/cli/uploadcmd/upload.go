package uploadcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/TierMobility/boring-registry/internal/cli/help"
	"github.com/TierMobility/boring-registry/internal/cli/rootcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
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
	case "gcs":
		if c.S3Bucket == "" {
			return errors.New("missing flag -s3-bucket")
		}

		reg, err := module.NewGCSRegistry(c.S3Bucket,
			module.WithS3RegistryBucketPrefix(c.S3Prefix),
			module.WithS3RegistryBucketRegion(c.S3Region),
		)
		if err != nil {
			return err
		}
		registry = reg
	default:
		return flag.ErrHelp
	}

	if _, err := os.Stat(args[0]); os.IsNotExist(err) {
		return err
	}

	return c.archiveModules(args[0], registry)
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
