package servercmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TierMobility/boring-registry/internal/cmd/help"
	"github.com/TierMobility/boring-registry/internal/cmd/rootcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/oklog/run"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	apiVersion = "v1"
)

var (
	prefix = fmt.Sprintf(`/%s`, apiVersion)
)

type Config struct {
	rootConfig *rootcmd.Config
	out        io.Writer

	APIKey                 string
	ListenAddress          string
	TelemetryListenAddress string
}

func New(rootConfig *rootcmd.Config, out io.Writer) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
		out:        out,
	}

	fs := flag.NewFlagSet("boring-registry server", flag.ExitOnError)
	fs.StringVar(&cfg.ListenAddress, "listen-address", ":5601", "listen address for the registry api")
	fs.StringVar(&cfg.APIKey, "api-key", "", "comma-delimited list of api keys")
	fs.StringVar(&cfg.TelemetryListenAddress, "telemetry-listen-address", ":7801", "listen address for telemetry")

	rootConfig.RegisterFlags(fs)

	return &ffcli.Command{
		Name:      "server",
		UsageFunc: help.UsageFunc,
		ShortHelp: "run the registry api server",
		LongHelp: help.Format(`Run the registry API server.

The server command expects some configuration, such as which registry type to use.
The default registry type is "s3" and is currently the only registry type available.
For more options see the available options below.

EXAMPLE USAGE

boring-registry server -type=s3 -s3-bucket=my-bucket`),
		ShortUsage: "server [flags]",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}
}

func (c *Config) printConfig() {
	c.rootConfig.UI.Output("==> Boring Registry server configuration:")
	c.rootConfig.UI.Output("")
	c.rootConfig.UI.Output(fmt.Sprintf("    Listen Address: %s", c.ListenAddress))
	c.rootConfig.UI.Output(fmt.Sprintf("    Registry: %s", c.rootConfig.Type))

	if c.rootConfig.Type == "s3" {
		c.rootConfig.UI.Output(fmt.Sprintf("    Bucket: %s", c.rootConfig.S3Bucket))
		if c.rootConfig.S3Prefix != "" {
			c.rootConfig.UI.Output(fmt.Sprintf("    Prefix: %s", c.rootConfig.S3Prefix))
		} else {
			c.rootConfig.UI.Output("    Prefix: /")
		}
	}

	c.rootConfig.UI.Output("")
	c.rootConfig.UI.Output("==> Boring Registry server started! Log data will stream below:")
	c.rootConfig.UI.Output("")
	c.rootConfig.UI.Output("")
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	c.printConfig()

	var g run.Group

	telemetryMux := http.NewServeMux()
	telemetryMux.Handle("/metrics", promhttp.Handler())
	telemetryMux.HandleFunc("/debug/pprof/", pprof.Index)
	telemetryMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	telemetryMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	telemetryMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	telemetryMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	telemetryMux.Handle("/debug/pprof/block", pprof.Handler("block"))
	telemetryMux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	telemetryMux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	telemetryMux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	telemetryMux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))

	telemetrySrv := &http.Server{
		Addr:         c.TelemetryListenAddress,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      telemetryMux,
	}

	g.Add(func() error {
		if err := telemetrySrv.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				level.Info(c.rootConfig.Logger).Log(
					"msg", "shutting down telemetry server gracefully",
				)
			} else {
				return err
			}
		}

		return nil
	}, func(err error) {
		if err := telemetrySrv.Close(); err != nil {
			level.Error(c.rootConfig.Logger).Log(
				"msg", "failed to shutdown telemetry server gracefully",
				"err", err,
			)
		}
	})

	mux := http.NewServeMux()

	opts := []httptransport.ServerOption{
		httptransport.ServerErrorHandler(
			transport.NewLogErrorHandler(c.rootConfig.Logger),
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

	var apiKeys []string

	if c.APIKey != "" {
		apiKeys = strings.Split(c.APIKey, ",")
	}

	mux.Handle(
		fmt.Sprintf(`%s/`, prefix),
		http.StripPrefix(
			prefix,
			module.MakeHandler(
				c.rootConfig.Service,
				module.AuthMiddleware(apiKeys...),
				opts...,
			),
		),
	)

	srv := &http.Server{
		Addr:         c.ListenAddress,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      mux,
	}

	g.Add(func() error {
		if err := srv.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				level.Info(c.rootConfig.Logger).Log(
					"msg", "shutting down server gracefully",
				)
			} else {
				return err
			}
		}

		return nil
	}, func(err error) {
		if err := srv.Close(); err != nil {
			level.Error(c.rootConfig.Logger).Log(
				"msg", "failed to shutdown telemetry server cleanly",
				"err", err,
			)
		}
	})

	g.Add(func() error {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
		<-sigint

		if err := srv.Shutdown(ctx); err != nil {
			return err
		}

		return telemetrySrv.Shutdown(ctx)
	}, func(err error) {
		level.Info(c.rootConfig.Logger).Log(
			"msg", "shutting down server",
		)
	})

	return g.Run()
}
