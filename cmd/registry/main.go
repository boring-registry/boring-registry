package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	apiVersion = "v1"
)

var (
	prefix = fmt.Sprintf(`/%s`, apiVersion)
)

func main() {
	var (
		flagAddr = flag.String("web.listen-address", func() string {
			if v := os.Getenv("REGISTRY_WEB_LISTEN_ADDRESS"); v != "" {
				return v
			}

			return ":5601"
		}(), "Address to listen on")

		flagTelemetryAddress = flag.String("web.telemetry-address", func() string {
			if v := os.Getenv("REGISTRY_WEB_TELEMETRY_ADDRESS"); v != "" {
				return v
			}

			return ":8701"
		}(), "Address to listen on for telemetry")

		flagTelemetryPath = flag.String("web.telemetry-path", func() string {
			if v := os.Getenv("REGISTRY_WEB_TELEMETRY_PATH"); v != "" {
				return v
			}

			return "/metrics"
		}(), "Path under which to expose metrics")

		flagDebug = flag.Bool("debug", func() bool {
			if v := os.Getenv("REGISTRY_DEBUG"); v != "" {
				val, err := strconv.ParseBool(v)
				if err != nil {
					return false
				}

				return val
			}

			return false
		}(), "Enables debug logging")

		flagRegistry = flag.String("registry", func() string {
			if v := os.Getenv("REGISTRY_TYPE"); v != "" {
				return v
			}

			return "s3"
		}(), "Registry type to use")

		flagRegistryS3Bucket       = flag.String("registry.s3.bucket", os.Getenv("REGISTRY_S3_BUCKET"), "Bucket to use for the S3 registry type")
		flagRegistryS3BucketPrefix = flag.String("registry.s3.bucket-prefix", os.Getenv("REGISTRY_S3_BUCKET_PREFIX"), "Bucket prefix to  use for the S3 registry type")
		flagRegistryS3BucketRegion = flag.String("registry.s3.bucket-region", os.Getenv("REGISTRY_S3_BUCKET_REGION"), "Region of the bucket to use for the S3 registry type")
	)

	flag.Parse()

	var logger log.Logger
	{
		logLevel := level.AllowInfo()
		if *flagDebug {
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

	go func(logger log.Logger, addr string) {
		mux := http.NewServeMux()
		mux.Handle(*flagTelemetryPath, promhttp.Handler())
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
		abort(logger, http.ListenAndServe(addr, mux))
	}(logger, *flagTelemetryAddress)

	var registry module.Registry

	switch *flagRegistry {
	case "s3":
		registry, err = module.NewS3Registry(*flagRegistryS3Bucket, module.WithS3RegistryBucketPrefix(*flagRegistryS3BucketPrefix), module.WithS3RegistryBucketRegion(*flagRegistryS3BucketRegion))
		if err != nil {
			abort(logger, err)
		}
	default:
		abort(logger, fmt.Errorf("invalid registry type '%s'", *flagRegistry))
	}

	service := module.NewService(registry)
	{
		service = module.LoggingMiddleware(logger)(service)
	}

	level.Info(logger).Log("msg", "starting server")

	mux := http.NewServeMux()
	opts := []httptransport.ServerOption{
		httptransport.ServerErrorHandler(
			transport.NewLogErrorHandler(logger),
		),
		httptransport.ServerErrorEncoder(module.ErrorEncoder),
		httptransport.ServerBefore(
			httptransport.PopulateRequestContext,
		),
	}

	mux.HandleFunc("/.well-known/terraform.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"modules.v1": "%s/modules"}`, prefix)))
	})

	mux.Handle(
		fmt.Sprintf(`%s/`, prefix),
		http.StripPrefix(
			prefix,
			module.MakeHandler(
				service,
				module.AuthMiddleware(collectAPIKeys(logger, os.Environ())...),
				opts...,
			),
		),
	)

	srv := &http.Server{
		Addr:         *flagAddr,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      mux,
	}

	abort(logger, srv.ListenAndServe())
}

func abort(logger log.Logger, err error) {
	if err == nil {
		return
	}

	level.Error(logger).Log("err", err)
	os.Exit(1)
}

func collectAPIKeys(logger log.Logger, in []string) []string {
	var keys []string

	for _, e := range in {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		if strings.HasPrefix(key, "REGISTRY_API_KEY_") {
			level.Debug(logger).Log(
				"msg", fmt.Sprintf("loading API key from env %s", key),
			)
			keys = append(keys, value)
		}
	}

	if len(keys) < 1 {
		level.Warn(logger).Log("msg", "no API key defined, consider setting one or multiple using env variables (REGISTRY_API_KEY_<key>=<value>)")
	}

	return keys
}
