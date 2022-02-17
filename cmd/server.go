package cmd

import (
	"context"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/mirror"
	"github.com/TierMobility/boring-registry/pkg/storage"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	httptransport "github.com/go-kit/kit/transport/http"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/transport"
	"github.com/pkg/errors"

	"golang.org/x/sync/errgroup"

	"github.com/TierMobility/boring-registry/pkg/auth"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/TierMobility/boring-registry/pkg/provider"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spf13/cobra"
)

const (
	apiVersion = "v1"
)

var (
	prefix          = fmt.Sprintf("/%s", apiVersion)
	prefixModules   = fmt.Sprintf("%s/modules", prefix)
	prefixProviders = fmt.Sprintf("%s/providers", prefix)
	prefixMirror = fmt.Sprintf("%s/mirror", prefix)
)

var (
	// General server options.
	flagAPIKey              string
	flagTLSCertFile         string
	flagTLSKeyFile          string
	flagListenAddr          string
	flagTelemetryListenAddr string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Starts the server component",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		group, ctx := errgroup.WithContext(ctx)

		mux, err := serveMux()
		if err != nil {
			return errors.Wrap(err, "failed to setup server")
		}

		server := &http.Server{
			Addr:         flagListenAddr,
			ReadTimeout:  5 * time.Second,
			// WriteTimeout is specifically set to 0 in order to disable the timeout. Downloading large binaries
			// through the pull-through cache from slow upstream servers might exceed any timeout set here.
			// This is definitely not ideal, as it impacts robustness.
			WriteTimeout: 0,
			Handler:      mux,
		}

		telemetryServer := &http.Server{
			Addr:         flagTelemetryListenAddr,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			Handler:      mux,
		}

		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

		// Signal handler.
		group.Go(func() error {
			select {
			case <-sigint:
				cancel()
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})

		// Server handler.
		group.Go(func() error {
			<-ctx.Done()

			if err := server.Shutdown(ctx); err != nil {
				if err != context.Canceled {
					level.Error(logger).Log(
						"msg", "failed to terminate server",
						"err", err,
					)
				}
			}

			if err := telemetryServer.Shutdown(ctx); err != nil {
				if err != context.Canceled {
					level.Error(logger).Log(
						"msg", "failed to terminate telemetry server",
						"err", err,
					)
				}
			}

			return nil
		})

		// Main server.
		group.Go(func() error {
			logger := log.With(logger, "listen", flagListenAddr)
			level.Info(logger).Log("msg", "starting server")
			defer level.Info(logger).Log("msg", "shutting down server")

			if flagTLSCertFile != "" || flagTLSKeyFile != "" {
				if err := server.ListenAndServeTLS(flagTLSCertFile, flagTLSKeyFile); err != nil {
					if err != http.ErrServerClosed {
						return err
					}
				}
			} else {
				if err := server.ListenAndServe(); err != nil {
					if err != http.ErrServerClosed {
						return err
					}
				}
			}
			return nil
		})

		// Telemetry server.
		group.Go(func() error {
			logger := log.With(logger, "listen", flagTelemetryListenAddr)
			level.Info(logger).Log("msg", "starting telemetry server")
			defer level.Info(logger).Log("msg", "shutting down telemetry server")

			if err := telemetryServer.ListenAndServe(); err != nil {
				if err != http.ErrServerClosed {
					return err
				}
			}
			return nil
		})

		return group.Wait()
	},
}

func setupModuleStorage() (module.Storage, error) {
	switch {
	case flagS3Bucket != "":
		return setupS3ModuleStorage()
	case flagGCSBucket != "":
		return setupGCSModuleStorage()
	case flagDirectoryPath != "":
		return setupDirectoryModuleStorage()
	default:
		return nil, errors.New("please specify a valid storage provider")
	}
}

func setupProviderStorage() (provider.Storage, error) {
	switch {
	case flagS3Bucket != "":
		return setupS3ProviderStorage()
	case flagGCSBucket != "":
		return setupGCSProviderStorage()
	case flagDirectoryPath != "":
		return setupDirectoryProviderStorage()
	default:
		return nil, errors.New("please specify a valid storage provider")
	}
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "Comma-separated string of static API keys to protect the server with")
	serverCmd.Flags().StringVar(&flagTLSKeyFile, "tls-key-file", "", "TLS private key to serve")
	serverCmd.Flags().StringVar(&flagTLSCertFile, "tls-cert-file", "", "TLS certificate to serve")
	serverCmd.Flags().StringVar(&flagListenAddr, "listen-address", ":5601", "Address to listen on")
	serverCmd.Flags().StringVar(&flagTelemetryListenAddr, "listen-telemetry-address", ":7801", "Telemetry address to listen on")
}

func serveMux() (*http.ServeMux, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/terraform.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"modules.v1": "%s/", "providers.v1": "%s/"}`, prefixModules, prefixProviders)))
	})

	registerMetrics(mux)

	// TODO(oliviermichaelis): instantiate storage backend here and pass reference to modules
	if err := registerModule(mux); err != nil {
		return nil, err
	}

	if err := registerProvider(mux); err != nil {
		return nil, err
	}

	if err := registerMirror(mux); err != nil {
		return nil, err
	}


	return mux, nil
}

func registerMetrics(mux *http.ServeMux) {
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
}

func registerModule(mux *http.ServeMux) error {
	s, err := setupModuleStorage()
	if err != nil {
		return errors.Wrap(err, "failed to setup module storage")
	}

	service := module.NewService(s)
	{
		service = module.LoggingMiddleware(logger)(service)
	}

	opts := []httptransport.ServerOption{
		httptransport.ServerErrorHandler(
			transport.NewLogErrorHandler(logger),
		),
		httptransport.ServerErrorEncoder(module.ErrorEncoder),
		httptransport.ServerBefore(
			httptransport.PopulateRequestContext,
		),
	}

	mux.Handle(
		fmt.Sprintf(`%s/`, prefixModules),
		http.StripPrefix(
			prefixModules,
			module.MakeHandler(
				service,
				auth.Middleware(splitKeys(flagAPIKey)...),
				opts...,
			),
		),
	)

	return nil
}

func registerProvider(mux *http.ServeMux) error {
	s, err := setupProviderStorage()
	if err != nil {
		return errors.Wrap(err, "failed to setup provider storage")
	}

	service := provider.NewService(s)
	{
		service = provider.LoggingMiddleware(logger)(service)
	}

	opts := []httptransport.ServerOption{
		httptransport.ServerErrorHandler(
			transport.NewLogErrorHandler(logger),
		),
		httptransport.ServerErrorEncoder(module.ErrorEncoder),
		httptransport.ServerBefore(
			httptransport.PopulateRequestContext,
		),
	}

	mux.Handle(
		fmt.Sprintf(`%s/`, prefixProviders),
		http.StripPrefix(
			prefixProviders,
			provider.MakeHandler(
				service,
				auth.Middleware(splitKeys(flagAPIKey)...),
				opts...,
			),
		),
	)

	return nil
}

func registerMirror(mux *http.ServeMux) error {
	directoryStorage, err := storage.NewDirectoryStorage(flagDirectoryPath)
	if err != nil {
		return err
	}

	service := mirror.NewService(directoryStorage)
	{
		service = mirror.ProxyingMiddleware(logger)(service)
		service = mirror.LoggingMiddleware(logger)(service)
	}

	opts := []httptransport.ServerOption{
		httptransport.ServerErrorHandler(
			transport.NewLogErrorHandler(logger),
		),
		httptransport.ServerErrorEncoder(mirror.ErrorEncoder),
		httptransport.ServerBefore(
			httptransport.PopulateRequestContext,
		),
	}

	mux.Handle(
		fmt.Sprintf(`%s/`, prefixMirror),
		http.StripPrefix(
			prefixMirror,
			mirror.MakeHandler(
				service,
				auth.Middleware(splitKeys(flagAPIKey)...),
				opts...,
			),
		),
	)

	return nil
}

func splitKeys(in string) []string {
	var keys []string

	if in != "" {
		keys = strings.Split(in, ",")
	}

	return keys
}
