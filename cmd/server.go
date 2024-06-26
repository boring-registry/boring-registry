package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boring-registry/boring-registry/pkg/auth"
	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/discovery"
	"github.com/boring-registry/boring-registry/pkg/mirror"
	"github.com/boring-registry/boring-registry/pkg/module"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"
	"github.com/boring-registry/boring-registry/pkg/provider"
	"github.com/boring-registry/boring-registry/pkg/proxy"
	"github.com/boring-registry/boring-registry/pkg/storage"

	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

const (
	apiVersion = "v1"
)

var (
	prefix          = fmt.Sprintf("/%s", apiVersion)
	prefixModules   = fmt.Sprintf("%s/modules", prefix)
	prefixProviders = fmt.Sprintf("%s/providers", prefix)
	prefixMirror    = fmt.Sprintf("%s/mirror", prefix)
	prefixProxy     = fmt.Sprintf("%s/proxy", prefix)
)

var (
	// Proxy options.
	flagProxy bool

	// General server options.
	flagTLSCertFile         string
	flagTLSKeyFile          string
	flagListenAddr          string
	flagTelemetryListenAddr string
	flagModuleArchiveFormat string

	// Login options.
	flagLoginIssuer     string
	flagLoginClient     string
	flagLoginScopes     []string
	flagLoginGrantTypes []string
	flagLoginAuthz      string
	flagLoginToken      string
	flagLoginPorts      []int

	// Static auth.
	flagAuthStaticTokens []string

	// Okta auth.
	flagAuthOktaIssuer string
	flagAuthOktaClaims []string

	// Provider Network Mirror
	flagProviderNetworkMirrorEnabled            bool
	flagProviderNetworkMirrorPullThroughEnabled bool
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Starts the server component",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		group, ctx := errgroup.WithContext(ctx)

		mux, err := serveMux(ctx)
		if err != nil {
			return fmt.Errorf("failed to setup server: %w", err)
		}

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
					slog.Error("failed to terminate server", slog.String("error", err.Error()))
				}
			}

			if err := telemetryServer.Shutdown(ctx); err != nil {
				if err != context.Canceled {
					slog.Error("failed to terminate telemetry server", slog.String("error", err.Error()))
				}
			}

			return nil
		})

		// Main server.
		group.Go(func() error {
			logger := slog.Default().With(slog.String("listen", flagListenAddr))
			logger.Info("starting server")
			defer logger.Info("shutting down server")

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
			logger := slog.Default().With(slog.String("listen", flagTelemetryListenAddr))
			logger.Info("starting telemetry server")
			defer logger.Info("shutting down telemetry server")

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

func init() {
	rootCmd.AddCommand(serverCmd)

	// General options.
	serverCmd.Flags().StringVar(&flagTLSKeyFile, "tls-key-file", "", "TLS private key to serve")
	serverCmd.Flags().StringVar(&flagTLSCertFile, "tls-cert-file", "", "TLS certificate to serve")
	serverCmd.Flags().StringVar(&flagListenAddr, "listen-address", ":5601", "Address to listen on")
	serverCmd.Flags().StringVar(&flagTelemetryListenAddr, "listen-telemetry-address", ":7801", "Telemetry address to listen on")
	serverCmd.Flags().StringVar(&flagModuleArchiveFormat, "storage-module-archive-format", storage.DefaultModuleArchiveFormat, "Archive file format for modules, specified without the leading dot")

	// Proxy options.
	serverCmd.PersistentFlags().BoolVar(&flagProxy, "download-proxy", false, "Enable proxying download request to remote storage")

	// Static auth options.
	serverCmd.Flags().StringSliceVar(&flagAuthStaticTokens, "auth-static-token", nil, "Static API token to protect the boring-registry")

	// Okta auth options.
	serverCmd.Flags().StringVar(&flagAuthOktaIssuer, "auth-okta-issuer", "", "Okta issuer")
	serverCmd.Flags().StringSliceVar(&flagAuthOktaClaims, "auth-okta-claims", nil, "Okta claims to validate")

	// Terraform Login Protocol options.
	serverCmd.Flags().StringVar(&flagLoginClient, "login-client", "", "The client_id value to use when making requests")
	serverCmd.Flags().StringSliceVar(&flagLoginGrantTypes, "login-grant-types", []string{"authz_code"}, "An array describing a set of OAuth 2.0 grant types")
	serverCmd.Flags().StringVar(&flagLoginAuthz, "login-authz", "", "The server's authorization endpoint")
	serverCmd.Flags().StringVar(&flagLoginToken, "login-token", "", "The server's token endpoint")
	serverCmd.Flags().IntSliceVar(&flagLoginPorts, "login-ports", []int{10000, 10010}, "Inclusive range of TCP ports that Terraform may use")
	serverCmd.Flags().StringSliceVar(&flagLoginScopes, "login-scopes", nil, "List of scopes")

	// Provider Network Mirror options
	serverCmd.Flags().BoolVar(&flagProviderNetworkMirrorEnabled, "network-mirror", true, "Enable the provider network mirror")
	serverCmd.Flags().BoolVar(&flagProviderNetworkMirrorPullThroughEnabled, "network-mirror-pull-through", false, "Enable the pull-through provider network mirror. This setting takes no effect if network-mirror is disabled")
}

// TODO(oliviermichaelis): move to root, as the storage flags are defined in root?
func setupStorage(ctx context.Context) (storage.Storage, error) {
	switch {
	case flagS3Bucket != "":
		return storage.NewS3Storage(ctx,
			flagS3Bucket,
			storage.WithS3StorageBucketPrefix(flagS3Prefix),
			storage.WithS3StorageBucketRegion(flagS3Region),
			storage.WithS3StorageBucketEndpoint(flagS3Endpoint),
			storage.WithS3StoragePathStyle(flagS3PathStyle),
			storage.WithS3ArchiveFormat(flagModuleArchiveFormat),
			storage.WithS3StorageSignedUrlExpiry(flagS3SignedURLExpiry),
		)
	case flagGCSBucket != "":
		return storage.NewGCSStorage(flagGCSBucket,
			storage.WithGCSStorageBucketPrefix(flagGCSPrefix),
			storage.WithGCSServiceAccount(flagGCSServiceAccount),
			storage.WithGCSSignedUrlExpiry(flagGCSSignedURLExpiry),
			storage.WithGCSArchiveFormat(flagModuleArchiveFormat),
		)
	case flagAzureStorageContainer != "":
		return storage.NewAzureStorage(flagAzureStorageAccount,
			flagAzureStorageContainer,
			storage.WithAzureStoragePrefix(flagAzureStoragePrefix),
			storage.WithAzureStorageArchiveFormat(flagModuleArchiveFormat),
			storage.WithAzureStorageSignedUrlExpiry(flagAzureStorageSignedURLExpiry),
		)
	default:
		return nil, errors.New("storage provider is not specified")
	}
}

func serveMux(ctx context.Context) (*http.ServeMux, error) {
	mux := http.NewServeMux()

	options := []discovery.Option{
		discovery.WithModulesV1(fmt.Sprintf("%s/", prefixModules)),
		discovery.WithProvidersV1(fmt.Sprintf("%s/", prefixProviders)),
	}

	if flagLoginClient != "" {
		login := &discovery.LoginV1{
			Client: flagLoginClient,
		}

		if flagLoginGrantTypes != nil {
			login.GrantTypes = flagLoginGrantTypes
		}

		if flagLoginAuthz != "" {
			login.Authz = flagLoginAuthz
		}

		if flagLoginToken != "" {
			login.Token = flagLoginToken
		}

		if flagLoginPorts != nil {
			login.Ports = flagLoginPorts
		}

		if flagLoginScopes != nil {
			login.Scopes = flagLoginScopes
		}

		options = append(options, discovery.WithLoginV1(login))
	}

	terraformJSON, err := json.Marshal(discovery.New(options...))
	if err != nil {
		return nil, err
	}

	mux.HandleFunc("/.well-known/terraform.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "application/json")
		w.Write(terraformJSON)
	})

	metrics := o11y.NewMetrics(nil)
	instrumentation := o11y.NewMiddleware(metrics.Http)

	registerMetrics(mux)

	s, err := setupStorage(ctx)
	if err != nil {
		return nil, err
	}

	proxyUrlService := core.NewProxyUrlService(flagProxy, prefixProxy)

	if err := registerModule(mux, s, metrics.Module, instrumentation, proxyUrlService); err != nil {
		return nil, err
	}

	if err := registerProvider(mux, s, metrics.Provider, instrumentation, proxyUrlService); err != nil {
		return nil, err
	}

	if flagProxy {
		if err := registerProxy(mux, s, metrics.Proxy, instrumentation); err != nil {
			return nil, err
		}
	}

	if flagProviderNetworkMirrorEnabled {
		var svc mirror.Service
		if flagProviderNetworkMirrorPullThroughEnabled {
			copier := mirror.NewCopier(ctx, s)
			svc = mirror.NewPullThroughMirror(s, copier)
		} else {
			svc = mirror.NewMirror(s)
		}

		if err := registerMirror(mux, s, svc, metrics.Mirror, instrumentation); err != nil {
			return nil, err
		}
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

func registerModule(mux *http.ServeMux, s storage.Storage, metrics *o11y.ModuleMetrics, instrumentation o11y.Middleware, proxyUrlService core.ProxyUrlService) error {
	service := module.NewService(s, proxyUrlService)
	{
		service = module.LoggingMiddleware()(service)
	}

	opts := []httptransport.ServerOption{
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
				authMiddleware(),
				metrics,
				instrumentation,
				opts...,
			),
		),
	)

	return nil
}

func authMiddleware() endpoint.Middleware {
	var providers []auth.Provider

	if flagAuthStaticTokens != nil {
		providers = append(providers, auth.NewStaticProvider(flagAuthStaticTokens...))
	}

	if flagAuthOktaIssuer != "" {
		providers = append(providers, auth.NewOktaProvider(flagAuthOktaIssuer, flagAuthOktaClaims...))
	}

	return auth.Middleware(providers...)
}

func registerProvider(mux *http.ServeMux, s storage.Storage, metrics *o11y.ProviderMetrics, instrumentation o11y.Middleware, proxyUrlService core.ProxyUrlService) error {
	service := provider.NewService(s, proxyUrlService)
	{
		service = provider.LoggingMiddleware()(service)
	}

	opts := []httptransport.ServerOption{
		httptransport.ServerErrorEncoder(provider.ErrorEncoder),
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
				authMiddleware(),
				metrics,
				instrumentation,
				opts...,
			),
		),
	)

	return nil
}

func registerMirror(mux *http.ServeMux, s storage.Storage, svc mirror.Service, metrics *o11y.MirrorMetrics, instrumentation o11y.Middleware) error {
	service := mirror.LoggingMiddleware()(svc)

	opts := []httptransport.ServerOption{
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
				authMiddleware(),
				metrics,
				instrumentation,
				opts...,
			),
		),
	)

	return nil
}

func registerProxy(mux *http.ServeMux, storage storage.Storage, metrics *o11y.ProxyMetrics, instrumentation o11y.Middleware) error {
	opts := []httptransport.ServerOption{
		httptransport.ServerErrorEncoder(proxy.ErrorEncoder),
		httptransport.ServerBefore(
			httptransport.PopulateRequestContext,
		),
	}

	mux.Handle(
		fmt.Sprintf(`%s/`, prefixProxy),
		http.StripPrefix(
			prefixProxy,
			proxy.MakeHandler(
				storage,
				metrics,
				instrumentation,
				opts...,
			),
		),
	)

	return nil
}
