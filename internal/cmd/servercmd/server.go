package servercmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/TierMobility/boring-registry/internal/cmd/help"
	"github.com/TierMobility/boring-registry/internal/cmd/rootcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
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
	fmt.Println(c.rootConfig.Info("==> Boring Registry server configuration:"))
	fmt.Println()
	fmt.Println(c.rootConfig.Info(fmt.Sprintf("    Listen Address: %s", c.ListenAddress)))
	fmt.Println(c.rootConfig.Info(fmt.Sprintf("    Registry: %s", c.rootConfig.Type)))

	if c.rootConfig.Type == "s3" {
		fmt.Println(c.rootConfig.Info(fmt.Sprintf("    Bucket: %s", c.rootConfig.S3Bucket)))
		if c.rootConfig.S3Prefix != "" {
			fmt.Println(c.rootConfig.Info(fmt.Sprintf("    Prefix: %s", c.rootConfig.S3Prefix)))
		} else {
			fmt.Println(c.rootConfig.Info("    Prefix: /"))
		}
	}

	fmt.Println()
	fmt.Println(c.rootConfig.Info("==> Boring Registry server started! Log data will stream below:"))
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	c.printConfig()

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
	}(c.TelemetryListenAddress)

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

	mux.Handle(
		fmt.Sprintf(`%s/`, prefix),
		http.StripPrefix(
			prefix,
			module.MakeHandler(
				c.rootConfig.Service,
				module.AuthMiddleware(strings.Split(c.APIKey, ",")...),
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

	return srv.ListenAndServe()
}
