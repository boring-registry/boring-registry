package servercmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TierMobility/boring-registry/internal/cli/help"
	"github.com/TierMobility/boring-registry/internal/cli/rootcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/oklog/run"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	apiVersion = "v1"
)

var (
	prefix = fmt.Sprintf(`/%s`, apiVersion)
)

type Config struct {
	*rootcmd.Config

	RegistryType string
	S3Bucket     string
	S3Prefix     string
	S3Region     string
	S3Endpoint   string
	S3PathStyle  bool

	GCSBucket          string
	GCSPrefix          string
	GCSSignedURL       bool
	GCSServiceAccount  string
	GCSSignedURLExpiry int64

	APIKey                 string
	ListenAddress          string
	TelemetryListenAddress string

	Keyfile  string
	Certfile string
}

func (c *Config) Exec(ctx context.Context, args []string) error {

	var (
		g                       run.Group
		server, telemetryServer *http.Server
	)

	{
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

		telemetryServer = &http.Server{
			Addr:         c.TelemetryListenAddress,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			Handler:      mux,
		}

		g.Add(func() error {
			level.Info(c.Logger).Log(
				"msg", "starting server",
				"service", "telemetry",
				"listen", c.TelemetryListenAddress,
			)

			if err := telemetryServer.ListenAndServe(); err != nil {
				if err == http.ErrServerClosed {
					level.Debug(c.Logger).Log(
						"msg", "shutting down telemetry server gracefully",
					)
				} else {
					return err
				}
			}

			return nil
		}, func(err error) {
			if err := telemetryServer.Close(); err != nil {
				level.Error(c.Logger).Log(
					"msg", "failed to shutdown telemetry server gracefully",
					"err", err,
				)
			}
		})
	}

	var registry module.Registry

	switch c.RegistryType {
	case "s3":
		if c.S3Bucket == "" {
			return errors.Wrap(flag.ErrHelp, "missing flag -s3-bucket")
		}
		// Check for endpoint and path style here
		// TODO

		reg, err := module.NewS3Registry(c.S3Bucket,
			module.WithS3RegistryBucketPrefix(c.S3Prefix),
			module.WithS3RegistryBucketRegion(c.S3Region),
			module.WithS3RegistryBucketEndpoint(c.S3Endpoint),
			module.WithS3RegistryPathStyle(c.S3PathStyle),
		)
		if err != nil {
			return errors.Wrap(err, "failed to set up registry")
		}

		registry = reg
	case "gcs":
		if c.GCSBucket == "" {
			return errors.Wrap(flag.ErrHelp, "missing flag -gcs-bucket")
		}

		reg, err := module.NewGCSRegistry(c.GCSBucket,
			module.WithGCSRegistryBucketPrefix(c.GCSPrefix),
			module.WithGCSRegistrySignedURL(c.GCSSignedURL),
			module.WithGCSServiceAccount(c.GCSServiceAccount),
			module.WithGCSSignedUrlExpiry(c.GCSSignedURLExpiry),
		)
		if err != nil {
			return errors.Wrap(err, "failed to set up registry")
		}
		registry = reg
	default:
		return flag.ErrHelp
	}

	service := module.NewService(registry)
	{
		service = module.LoggingMiddleware(c.Logger)(service)
	}

	{
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/terraform.json", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-type", "application/json")
			w.Write([]byte(fmt.Sprintf(`{"modules.v1": "%s/modules"}`, prefix)))
		})

		opts := []httptransport.ServerOption{
			httptransport.ServerErrorHandler(
				transport.NewLogErrorHandler(c.Logger),
			),
			httptransport.ServerErrorEncoder(module.ErrorEncoder),
			httptransport.ServerBefore(
				httptransport.PopulateRequestContext,
			),
		}

		var apiKeys []string
		if c.APIKey != "" {
			apiKeys = strings.Split(c.APIKey, ",")
		}

		mux.Handle(
			fmt.Sprintf(`%s/`, prefix),
			http.StripPrefix(
				prefix,
				module.MakeHandler(
					service,
					module.AuthMiddleware(apiKeys...),
					opts...,
				),
			),
		)

		server = &http.Server{
			Addr:         c.ListenAddress,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			Handler:      mux,
		}

		g.Add(func() error {
			tls := "disabled"
			if c.Certfile != "" && c.Keyfile != "" {
				tls = "enabled"
			}

			level.Info(c.Logger).Log(
				"msg", "starting server",
				"service", "api",
				"listen", c.ListenAddress,
				"tls", tls,
			)

			if c.Certfile != "" && c.Keyfile != "" {
				if err := server.ListenAndServeTLS(c.Certfile, c.Keyfile); err != nil {
					if err == http.ErrServerClosed {
						level.Debug(c.Logger).Log(
							"msg", "shutting down server gracefully",
						)
					} else {
						return err
					}
				}
			} else {
				if err := server.ListenAndServe(); err != nil {
					if err == http.ErrServerClosed {
						level.Debug(c.Logger).Log(
							"msg", "shutting down server gracefully",
						)
					} else {
						return err
					}
				}
			}

			return nil
		}, func(err error) {
			if err := server.Close(); err != nil {
				level.Error(c.Logger).Log(
					"msg", "failed to shutdown server gracefully",
					"err", err,
				)
			}
		})
	}

	{
		g.Add(func() error {
			sigint := make(chan os.Signal, 1)
			signal.Notify(sigint, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
			<-sigint

			if err := server.Shutdown(ctx); err != nil {
				return err
			}

			return telemetryServer.Shutdown(ctx)
		}, func(err error) {})
	}

	return g.Run()
}

func New(config *rootcmd.Config) *ffcli.Command {
	cfg := &Config{
		Config: config,
	}

	fs := flag.NewFlagSet("boring-registry server", flag.ExitOnError)
	fs.StringVar(&cfg.ListenAddress, "listen-address", ":5601", "Listen address for the registry api")
	fs.StringVar(&cfg.TelemetryListenAddress, "telemetry-listen-address", ":7801", "Listen address for telemetry")
	fs.StringVar(&cfg.APIKey, "api-key", "", "Comma-separated string of static API keys to protect the server with")
	fs.StringVar(&cfg.RegistryType, "type", "", "Registry type to use (currently only \"s3\" and \"gcs\" is supported)")
	fs.StringVar(&cfg.S3Bucket, "s3-bucket", "", "Bucket to use when using the S3 registry type")
	fs.StringVar(&cfg.S3Prefix, "s3-prefix", "", "Prefix to use when using the S3 registry type")
	fs.StringVar(&cfg.S3Region, "s3-region", "", "Region of the S3 bucket when using the S3 registry type")
	fs.StringVar(&cfg.S3Endpoint, "s3-endpoint", "", "Endpoint of the S3 bucket when using the S3 registry type")
	fs.BoolVar(&cfg.S3PathStyle, "s3-pathstyle", false, "Use PathStyle for S3 bucket when using the S3 registry type")
	fs.StringVar(&cfg.GCSBucket, "gcs-bucket", "", "Bucket to use when using the GCS registry type")
	fs.StringVar(&cfg.GCSPrefix, "gcs-prefix", "", "Prefix to use when using the GCS registry type")
	fs.StringVar(&cfg.Keyfile, "key-file", "", "TLS private key to serve")
	fs.StringVar(&cfg.Certfile, "cert-file", "", "TLS certificate to serve")
	fs.BoolVar(&cfg.GCSSignedURL, "gcs-signedurl", false, "Generate GCS signedURL (public) instead of relying on GCP credentials being set on terraform init. WARNING: only use in combination with `api-key` option")
	fs.Int64Var(&cfg.GCSSignedURLExpiry, "gcs-signedurl-expiry", 30, "Generate GCS signed URL valid for X seconds. Only meaningful if used in combination with `gcs-signedurl`")
	fs.StringVar(&cfg.GCSServiceAccount, "gcs-sa-email", "", "Google service account email to be used for Application Default Credentials (ADC). GOOGLE_APPLICATION_CREDENTIALS environment variable might be used as alternative. For GCS presigned URLs this SA needs the `iam.serviceAccountTokenCreator` role")
	config.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "server",
		UsageFunc:  help.UsageFunc,
		ShortUsage: "boring-registry server -type=<type> [flags]",
		ShortHelp:  "Runs the server component",
		FlagSet:    fs,
		Options:    []ff.Option{ff.WithEnvVarPrefix(help.EnvVarPrefix)},
		LongHelp: fmt.Sprint(`  Runs the server component.

  This command requires some configuration, such as which registry type to use.

  The server starts two servers (one for serving the API and one for Telemetry).
  
  Example Usage: boring-registry server -type=s3 -s3-bucket=example-bucket
  
  For more options see the available options below.`),
		Exec: cfg.Exec,
	}
}
