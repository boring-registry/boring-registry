package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/TierMobility/boring-registry/internal/cmd/rootcmd"
	"github.com/TierMobility/boring-registry/internal/cmd/servercmd"
	"github.com/TierMobility/boring-registry/internal/cmd/uploadcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	go func(addr string) {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		mux.Handle("/debug/pprof/block", pprof.Handler("block"))
		mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
		mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		http.ListenAndServe(addr, mux)
	}(config.TelemetryListenAddress)

	switch config.RegistryType {
	case "s3":
		registry, err := module.NewS3Registry(config.RegistryS3Bucket,
			module.WithS3RegistryBucketPrefix(config.RegistryS3Prefix),
			module.WithS3RegistryBucketRegion(config.RegistryS3Region),
		)
		if err != nil {
			abort(logger, err)
		}
		config.Registry = registry
	default:
		abort(logger, fmt.Errorf("invalid registry type '%s'", config.RegistryType))
	}

	service := module.NewService(config.Registry)
	{
		service = module.LoggingMiddleware(logger)(service)
	}
	config.Service = service

	abort(logger, root.Run(context.Background()))
}

func abort(logger log.Logger, err error) {
	if err == nil {
		return
	}

	level.Error(logger).Log("err", err)
	os.Exit(1)
}
