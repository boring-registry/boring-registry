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
	// Proxy options
	flagProxy bool

	// General server options
	flagTLSCertFile         string
	flagTLSKeyFile          string
	flagListenAddr          string
	flagTelemetryListenAddr string
	flagModuleArchiveFormat string

	// Login options
	flagLoginGrantTypes []string
	flagLoginPorts      []int

	// Static auth
	flagAuthStaticTokens []string

	// OIDC auth
	flagAuthOidcIssuer   string
	flagAuthOidcClientId string
	flagAuthOidcScopes   []string

	// Okta auth
	flagAuthOktaIssuer   string
	flagAuthOktaClientId string
	flagAuthOktaClaims   []string
	flagAuthOktaAuthz    string
	flagAuthOktaToken    string
	flagLoginScopes      []string

	// Provider Network Mirror
	flagProviderNetworkMirrorEnabled            bool
	flagProviderNetworkMirrorPullThroughEnabled bool

	// Provider Network Mirror Cache options
	flagProviderNetworkMirrorPullThroughCacheEnabled bool
	flagProviderNetworkMirrorPullThroughCacheTTL     time.Duration
	flagProviderNetworkMirrorPullThroughCacheSize    int
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
	serverCmd.Flags().StringSliceVar(&flagLoginScopes, "login-scopes", nil, "List of scopes")

	// OIDC auth options
	serverCmd.Flags().StringVar(&flagAuthOidcIssuer, "auth-oidc-issuer", "", "OIDC issuer URL")
	serverCmd.Flags().StringVar(&flagAuthOidcClientId, "auth-oidc-clientid", "", "OIDC client identifier")
	serverCmd.Flags().StringSliceVar(&flagAuthOidcScopes, "auth-oidc-scopes", nil, "List of OAuth2 scopes")

	// Terraform Login Protocol options.
	serverCmd.Flags().StringVar(&flagAuthOktaClientId, "login-client", "", "The client_id value to use when making requests")
	serverCmd.Flags().StringSliceVar(&flagLoginGrantTypes, "login-grant-types", []string{"authz_code"}, "An array describing a set of OAuth 2.0 grant types")
	serverCmd.Flags().StringVar(&flagAuthOktaAuthz, "login-authz", "", "The server's authorization endpoint")
	serverCmd.Flags().StringVar(&flagAuthOktaToken, "login-token", "", "The server's token endpoint")
	serverCmd.Flags().IntSliceVar(&flagLoginPorts, "login-ports", []int{10000, 10010}, "Inclusive range of TCP ports that Terraform/OpenTofu CLI may use")

	// Provider Network Mirror options
	serverCmd.Flags().BoolVar(&flagProviderNetworkMirrorEnabled, "network-mirror", true, "Enable the provider network mirror")
	serverCmd.Flags().BoolVar(&flagProviderNetworkMirrorPullThroughEnabled, "network-mirror-pull-through", false, "Enable the pull-through provider network mirror. This setting takes no effect if network-mirror is disabled")
	serverCmd.Flags().BoolVar(&flagProviderNetworkMirrorPullThroughCacheEnabled, "network-mirror-pull-through-cache-enabled", false, "Enable in-memory cache for pull-through mirror")
	serverCmd.Flags().DurationVar(&flagProviderNetworkMirrorPullThroughCacheTTL, "network-mirror-pull-through-cache-ttl", 24*time.Hour, "Cache TTL in hours")
	serverCmd.Flags().IntVar(&flagProviderNetworkMirrorPullThroughCacheSize, "network-mirror-pull-through-cache-size", 16, "Cache maximum size in MB by upstream registry")
}

func serveMux(ctx context.Context) (*http.ServeMux, error) {
	mux := http.NewServeMux()

	authMiddleware, login, err := authMiddleware(ctx)
	if err != nil {
		return nil, err
	}

	metrics := o11y.NewMetrics(nil)
	instrumentation := o11y.NewMiddleware(metrics.Http)

	registerMetrics(mux)
	if err := registerDiscovery(mux, login); err != nil {
		return nil, fmt.Errorf("failed to register discovery: %w", err)
	}

	s, err := setupStorage(ctx)
	if err != nil {
		return nil, err
	}

	proxyUrlService := core.NewProxyUrlService(flagProxy, prefixProxy)

	if err := registerModule(mux, s, authMiddleware, metrics.Module, instrumentation, proxyUrlService); err != nil {
		return nil, err
	}

	if err := registerProvider(mux, s, authMiddleware, metrics.Provider, instrumentation, proxyUrlService); err != nil {
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

			// Prepare cache configuration
			cacheConfig := mirror.CacheConfig{
				Enabled:   flagProviderNetworkMirrorPullThroughCacheEnabled,
				TTL:       time.Duration(flagProviderNetworkMirrorPullThroughCacheTTL) * time.Hour,
				MaxSizeMB: flagProviderNetworkMirrorPullThroughCacheSize,
			}

			svc, err = mirror.NewPullThroughMirror(s, copier, cacheConfig, metrics.Mirror)
			if err != nil {
				return nil, err
			}
		} else {
			svc = mirror.NewMirror(s)
		}

		if err := registerMirror(mux, s, svc, authMiddleware, metrics.Mirror, instrumentation); err != nil {
			return nil, err
		}
	}

	return mux, nil
}

func setupOidc(ctx context.Context) (auth.Provider, *discovery.LoginV1, error) {
	authCtx, cancelAuthCtx := context.WithTimeout(ctx, 15*time.Second)
	defer cancelAuthCtx()

	slog.Debug("setting up oidc auth",
		slog.String("client-id", flagAuthOidcClientId),
		slog.String("issuer", flagAuthOidcIssuer),
		slog.String("client-id", flagAuthOidcClientId),
		slog.Any("ports", flagLoginPorts),
		slog.Any("scopes", flagAuthOidcScopes),
	)

	provider, err := auth.NewOidcProvider(authCtx, flagAuthOidcIssuer, flagAuthOidcClientId)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to set up oidc provider: %w", err)
	}

	login := &discovery.LoginV1{
		Client:     flagAuthOidcClientId,
		GrantTypes: flagLoginGrantTypes,
		Authz:      provider.AuthURL(),
		Token:      provider.TokenURL(),
		Ports:      flagLoginPorts,
		Scopes:     flagAuthOidcScopes,
	}

	return provider, login, nil
}

func setupOkta() (auth.Provider, *discovery.LoginV1) {
	slog.Debug("setting up okta auth", slog.String("issuer", flagAuthOktaIssuer), slog.String("client-id", flagAuthOktaClientId))
	slog.Warn("Okta auth is deprecated, please migrate to OIDC auth")
	p := auth.NewOktaProvider(flagAuthOktaIssuer, flagAuthOktaClaims...)
	login := &discovery.LoginV1{
		Client:     flagAuthOktaClientId,
		GrantTypes: flagLoginGrantTypes,
		Authz:      flagAuthOktaAuthz,
		Token:      flagAuthOktaToken,
		Ports:      flagLoginPorts,
		Scopes:     flagLoginScopes,
	}

	return p, login
}

func authMiddleware(ctx context.Context) (endpoint.Middleware, *discovery.LoginV1, error) {
	providers := []auth.Provider{}

	if flagAuthStaticTokens != nil {
		providers = append(providers, auth.NewStaticProvider(flagAuthStaticTokens...))
	}

	// Check if OIDC or Okta are configured, we only want to allow one at a time.
	// OIDC is recommended, we want to deprecate our Okta-specific implementation and use our OIDC implementation instead, which Okta also supports.
	if flagAuthOidcIssuer != "" && flagAuthOktaIssuer != "" {
		return nil, nil, errors.New("both OIDC and Okta are configured, only one is allowed at a time")
	}

	// We construct the discovery.LoginV1 on this level, as we need the OIDC provider to look up the
	// authorization and token endpoints dynamically to populate the LoginV1
	var login *discovery.LoginV1
	if flagAuthOidcIssuer != "" || flagAuthOktaIssuer != "" {
		var p auth.Provider
		if flagAuthOidcIssuer != "" {
			var err error
			p, login, err = setupOidc(ctx)
			if err != nil {
				return nil, nil, err
			}
		} else if flagAuthOktaIssuer != "" {
			p, login = setupOkta()
		}
		providers = append(providers, p)
	}

	if login != nil { // Can be nil if neither Oidc, Okta, or API token are configured
		if err := login.Validate(); err != nil {
			return nil, nil, err
		}
	}

	return auth.Middleware(providers...), login, nil
}

func registerMetrics(mux *http.ServeMux) {
	mux.Handle("/metrics", promhttp.Handler())
	if flagDebug {
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
}

func registerDiscovery(mux *http.ServeMux, login *discovery.LoginV1) error {
	options := []discovery.Option{
		discovery.WithModulesV1(fmt.Sprintf("%s/", prefixModules)),
		discovery.WithProvidersV1(fmt.Sprintf("%s/", prefixProviders)),
		discovery.WithLoginV1(login),
	}

	terraformJSON, err := json.Marshal(discovery.NewDiscovery(options...))
	if err != nil {
		return err
	}

	mux.HandleFunc("/.well-known/terraform.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "application/json")
		if _, err := w.Write(terraformJSON); err != nil {
			slog.Error("failed to write http response in discovery handler", slog.String("error", err.Error()))
		}
	})

	return nil
}

func registerModule(mux *http.ServeMux, s storage.Storage, auth endpoint.Middleware, metrics *o11y.ModuleMetrics, instrumentation o11y.Middleware, proxyUrlService core.ProxyUrlService) error {
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
				auth,
				metrics,
				instrumentation,
				opts...,
			),
		),
	)

	return nil
}

func registerProvider(mux *http.ServeMux, s storage.Storage, authMiddleware endpoint.Middleware, metrics *o11y.ProviderMetrics, instrumentation o11y.Middleware, proxyUrlService core.ProxyUrlService) error {
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
				authMiddleware,
				metrics,
				instrumentation,
				opts...,
			),
		),
	)

	return nil
}

func registerMirror(mux *http.ServeMux, _ storage.Storage, svc mirror.Service, authMiddleware endpoint.Middleware, metrics *o11y.MirrorMetrics, instrumentation o11y.Middleware) error {
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
				authMiddleware,
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
