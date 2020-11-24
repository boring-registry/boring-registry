package servercmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TierMobility/boring-registry/internal/cmd/help"
	"github.com/TierMobility/boring-registry/internal/cmd/rootcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/peterbourgon/ff/v3/ffcli"
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

	ListenAddress string
}

func New(rootConfig *rootcmd.Config, out io.Writer) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
		out:        out,
	}

	fs := flag.NewFlagSet("boring-registry server", flag.ExitOnError)
	fs.StringVar(&cfg.ListenAddress, "listen-address", ":5601", "listen address for the registry api")
	rootConfig.RegisterFlags(fs)

	return &ffcli.Command{
		Name:      "server",
		ShortHelp: "run the registry api server",
		LongHelp: help.FormatHelp(`Run the registry API server.

The server command expects some configuration, such as which registry type to use.
The default registry type is "s3" and is currently the only registry type available.
For more options see the available options below.

EXAMPLE USAGE

boring-registry server \
  -registry=s3 \
  -registry.s3.bucket=my-bucket
`),
		ShortUsage: "server [flags]",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	level.Info(c.rootConfig.Logger).Log("msg", "starting server")

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
				module.AuthMiddleware(collectAPIKeys(c.rootConfig.Logger, os.Environ())...),
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
