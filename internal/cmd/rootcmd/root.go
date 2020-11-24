package rootcmd

import (
	"context"
	"flag"

	"github.com/TierMobility/boring-registry/internal/cmd/help"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type Config struct {
	Debug   bool
	NoColor bool

	Type     string
	S3Bucket string
	S3Prefix string
	S3Region string

	Logger   log.Logger
	Service  module.Service
	Registry module.Registry
}

func New() (*ffcli.Command, *Config) {
	var cfg Config
	fs := flag.NewFlagSet("boring-registry", flag.ExitOnError)
	cfg.RegisterFlags(fs)

	return &ffcli.Command{
		Name:      "boring-registry",
		UsageFunc: help.UsageFunc,
		Options: []ff.Option{
			ff.WithEnvVarPrefix("BORING_REGISTRY"),
		},
		ShortUsage: "boring-registry [flags] <subcommand> [flags] [<arg>...]",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}, &cfg
}

func (c *Config) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.Debug, "debug", false, "log debug output")
	fs.BoolVar(&c.NoColor, "no-color", false, "disable color output")
	fs.StringVar(&c.Type, "type", "s3", "registry type")
	fs.StringVar(&c.S3Bucket, "s3-bucket", "", "s3 bucket to use for the S3 registry")
	fs.StringVar(&c.S3Prefix, "s3-prefix", "", "s3 prefix to use for the S3 registry")
	fs.StringVar(&c.S3Region, "s3-region", "", "s3 region to use for the S3 registry")
}

func (c *Config) Exec(ctx context.Context, args []string) error { return flag.ErrHelp }

func (c *Config) Info(v string) string {
	return help.Info(v, c.NoColor)
}

func (c *Config) Error(v string) string {
	return help.Error(v, c.NoColor)
}

func (c *Config) Warn(v string) string {
	return help.Warn(v, c.NoColor)
}

func (c *Config) Success(v string) string {
	return help.Success(v, c.NoColor)
}
