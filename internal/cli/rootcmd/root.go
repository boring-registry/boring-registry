package rootcmd

import (
	"context"
	"flag"
	"io"

	"github.com/TierMobility/boring-registry/internal/cli/help"
	"github.com/go-kit/kit/log"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type Config struct {
	Logger log.Logger
	Output io.Writer

	JSON  bool
	Debug bool

	NoColor bool
}

func (c *Config) Run(ctx context.Context, args []string) error {
	return flag.ErrHelp
}

func New() (*ffcli.Command, *Config) {
	cfg := Config{}

	fs := flag.NewFlagSet("boring-registry", flag.ExitOnError)
	cfg.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "boring-registry",
		UsageFunc:  help.UsageFunc,
		ShortUsage: "boring-registry [flags] <subcommand> [flags] [<arg>...]",
		FlagSet:    fs,
		Options:    []ff.Option{ff.WithEnvVarPrefix(help.EnvVarPrefix)},
		Exec:       cfg.Run,
	}, &cfg
}

func (c *Config) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.Debug, "debug", false, "Enable debug output")
	fs.BoolVar(&c.JSON, "json", false, "Output logs in JSON format")
	fs.BoolVar(&c.NoColor, "no-color", false, "Disables colored output")
}
