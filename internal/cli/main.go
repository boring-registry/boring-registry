package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/TierMobility/boring-registry/internal/cli/rootcmd"
	"github.com/TierMobility/boring-registry/internal/cli/servercmd"
	"github.com/TierMobility/boring-registry/internal/cli/uploadcmd"
	"github.com/TierMobility/boring-registry/internal/cli/versioncmd"
	"github.com/fatih/color"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/peterbourgon/ff/v3/ffcli"
)

const (
	logKeyCaller    = "caller"
	logKeyHostname  = "hostname"
	logKeyTimestamp = "timestamp"
)

func Run(args []string) int {
	var (
		ctx          = context.Background()
		root, config = rootcmd.New()
	)

	config.Output = os.Stdout
	root.Subcommands = []*ffcli.Command{
		servercmd.New(config),
		uploadcmd.New(config),
		versioncmd.New(config),
	}

	if err := root.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse args: %v\n", err)
		return 1
	}

	if err := preRun(config); err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare CLI: %v\n", err)
		return 1
	}

	if err := root.Run(ctx); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	}

	return 0
}

func preRun(config *rootcmd.Config) error {
	logger := log.NewLogfmtLogger(config.Output)

	if config.JSON {
		logger = log.NewJSONLogger(config.Output)
	}

	logger = log.With(logger,
		logKeyCaller, log.Caller(5),
		logKeyTimestamp, log.DefaultTimestampUTC,
	)

	logLevel := level.AllowInfo()
	{
		if config.Debug {
			logLevel = level.AllowDebug()
		}
		logger = level.NewFilter(logger, logLevel)
	}

	if hostname, err := os.Hostname(); err == nil {
		logger = log.With(logger, logKeyHostname, hostname)
	}
	config.Logger = logger

	if config.NoColor {
		color.NoColor = true
	}

	return nil
}
