package versioncmd

import (
	"context"
	"fmt"

	"github.com/TierMobility/boring-registry/internal/cli/help"
	"github.com/TierMobility/boring-registry/internal/cli/rootcmd"
	"github.com/TierMobility/boring-registry/version"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type Config struct {
	*rootcmd.Config
}

func (c *Config) Exec(ctx context.Context, args []string) error {
	fmt.Println(version.String())
	return nil
}

func New(config *rootcmd.Config) *ffcli.Command {
	cfg := &Config{
		Config: config,
	}

	return &ffcli.Command{
		Name:       "version",
		UsageFunc:  help.UsageFunc,
		ShortUsage: "boring-registry version [flags]",
		Options:    []ff.Option{ff.WithEnvVarPrefix(help.EnvVarPrefix)},
		ShortHelp:  "Prints the version",
		Exec:       cfg.Exec,
	}
}
