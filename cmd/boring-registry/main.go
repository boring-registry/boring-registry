package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/TierMobility/boring-registry/internal/cmd/help"
	"github.com/TierMobility/boring-registry/internal/cmd/rootcmd"
	"github.com/TierMobility/boring-registry/internal/cmd/servercmd"
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
		fmt.Fprintf(os.Stderr, "error during parse: %v\n", err)
		os.Exit(1)
	}

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
		abort(logger, err)
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
			abort(logger, err)
		}
		config.Registry = registry
	default:
		fmt.Println(help.Error(fmt.Sprintf("Invalid registry type '%s'", config.Type)))
		os.Exit(1)
	}

	service := module.NewService(config.Registry)
	{
		service = module.LoggingMiddleware(config.Logger)(service)
	}
	config.Service = service

	if err := root.Run(context.Background()); err != nil {
		if err != flag.ErrHelp {
			fmt.Println(help.Error(fmt.Sprintf("Failed to run: '%s'", err)))
			os.Exit(1)
		}
	}
}

func abort(logger log.Logger, err error) {
	if err == nil {
		return
	}

	level.Error(logger).Log("err", err)
	os.Exit(1)
}
