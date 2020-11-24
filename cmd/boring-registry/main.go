package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/TierMobility/boring-registry/internal/cmd/rootcmd"
	"github.com/TierMobility/boring-registry/internal/cmd/servercmd"
	"github.com/TierMobility/boring-registry/internal/cmd/ui"
	"github.com/TierMobility/boring-registry/internal/cmd/uploadcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/peterbourgon/ff/v3/ffcli"
)

const (
	apiVersion = "v1"
)

var (
	prefix             = fmt.Sprintf(`/%s`, apiVersion)
	moduleSpecFileName = "boring-registry.hcl"
)

func main() {
	root, config := rootcmd.New()

	root.Subcommands = []*ffcli.Command{
		servercmd.New(config, os.Stdout),
		uploadcmd.New(config, os.Stdout),
	}

	if err := root.Parse(os.Args[1:]); err != nil {
		abort(err)
	}

	ctx := context.Background()
	ui := ui.NewUI(ctx, ui.WithColors(!config.NoColor), ui.WithOutput(os.Stdout))
	config.UI = ui

	var logger log.Logger
	{
		logLevel := level.AllowInfo()
		if config.Debug {
			logLevel = level.AllowAll()
		}
		logger = log.With(
			log.NewJSONLogger(os.Stdout),
			"timestamp", log.DefaultTimestampUTC,
			"caller", log.Caller(5),
		)
		logger = level.NewFilter(logger, logLevel)
	}

	hostname, err := os.Hostname()
	if err != nil {
		abort(err)
	}
	logger = log.With(logger, "hostname", hostname)
	config.Logger = logger

	switch config.Type {
	case "s3":
		registry, err := module.NewS3Registry(config.S3Bucket,
			module.WithS3RegistryBucketPrefix(config.S3Prefix),
			module.WithS3RegistryBucketRegion(config.S3Region),
		)
		if err != nil {
			abort(err)
		}
		config.Registry = registry
	default:
		abort(fmt.Errorf("Invalid registry type '%s'", config.Type))
	}

	service := module.NewService(config.Registry)
	{
		service = module.LoggingMiddleware(config.Logger)(service)
	}
	config.Service = service

	if err := root.Run(ctx); err != nil {
		if err != flag.ErrHelp {
			abort(err)
		}
	}
}

func abort(err error) {
	if err == nil {
		return
	}

	// fmt.Println(help.Error(fmt.Sprintf("Error: '%s'", err)))
	os.Exit(1)
}
