package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/pkg/errors"

	"golang.org/x/sync/errgroup"

	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spf13/cobra"
)

const (
	apiVersion = "v1"
)

var (
	prefix = fmt.Sprintf(`/%s`, apiVersion)
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
		storage, err := setupModuleStorage()
		if err != nil {
			return errors.Wrap(err, "failed to setup storage")
		}

		service := module.NewService(storage)
		mux := serveMux(service)
		ctx, cancel := context.WithCancel(context.Background())
		group, ctx := errgroup.WithContext(ctx)

		server := &http.Server{
			Addr:         flagListenAddr,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
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

func serveMux(service module.Service) *http.ServeMux {
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

	mux.HandleFunc("/.well-known/terraform.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"modules.v1": "%s/modules"}`, prefix)))
	})

	opts := []httptransport.ServerOption{
		httptransport.ServerErrorHandler(
			transport.NewLogErrorHandler(logger),
		),
		httptransport.ServerErrorEncoder(module.ErrorEncoder),
		httptransport.ServerBefore(
			httptransport.PopulateRequestContext,
		),
	}

	var apiKeys []string
	if flagAPIKey != "" {
		apiKeys = strings.Split(flagAPIKey, ",")
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

	return mux
}
