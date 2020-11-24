package rootcmd

import (
	"context"
	"flag"

	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type Config struct {
	Debug                  bool
	RegistryType           string
	RegistryS3Bucket       string
	RegistryS3Prefix       string
	RegistryS3Region       string
	TelemetryListenAddress string

	Logger   log.Logger
	Service  module.Service
	Registry module.Registry
}

func New() (*ffcli.Command, *Config) {
	var cfg Config
	fs := flag.NewFlagSet("boring-registry", flag.ExitOnError)
	cfg.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "boring-registry",
		ShortUsage: "boring-registry [flags] <subcommand> [flags] [<arg>...]",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}, &cfg
}

func (c *Config) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.Debug, "debug", false, "log debug output")
	fs.StringVar(&c.RegistryType, "registry", "s3", "registry type")
	fs.StringVar(&c.RegistryS3Bucket, "registry.s3.bucket", "", "s3 bucket to use for the s3 registry")
	fs.StringVar(&c.RegistryS3Prefix, "registry.s3.prefix", "", "s3 prefix to use for the s3 registry")
	fs.StringVar(&c.RegistryS3Prefix, "registry.s3.region", "", "s3 region to use for the s3 registry")
	fs.StringVar(&c.TelemetryListenAddress, "telemetry-listen-address", ":7801", "listen address for telemetry")
}

func (c *Config) Exec(ctx context.Context, args []string) error { return flag.ErrHelp }
