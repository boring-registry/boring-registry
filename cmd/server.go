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
	"reflect"
	"strconv"
	"strings"
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
	flagAuthOidc         []string
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
	serverCmd.Flags().StringSliceVar(&flagAuthOidc, "auth-oidc", []string{}, "Enable multiple OIDC authentication methods. Format: client_id=...;issuer=...;scopes=...;login_grants=...;login_ports=...")
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
}

func serveMux(ctx context.Context) (*http.ServeMux, error) {
	mux := http.NewServeMux()

	authMiddleware, logins, err := authMiddleware(ctx)
	if err != nil {
		return nil, err
	}

	metrics := o11y.NewMetrics(nil)
	instrumentation := o11y.NewMiddleware(metrics.Http)

	registerMetrics(mux)
	registerDiscovery(mux, logins)

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
			svc = mirror.NewPullThroughMirror(s, copier)
		} else {
			svc = mirror.NewMirror(s)
		}

		if err := registerMirror(mux, s, svc, authMiddleware, metrics.Mirror, instrumentation); err != nil {
			return nil, err
		}
	}

	return mux, nil
}

func setFieldByKey(config *auth.OidcConfig, key string, value interface{}) error {
    keyToField := map[string]string{
        "client_id": "ClientID",
        "issuer":    "Issuer",
        "scopes":    "Scopes",
        "login_grants": "LoginGrants",
        "login_ports":  "LoginPorts",
    }

    fieldName, ok := keyToField[key]
	if !ok {
		return fmt.Errorf("no mapping found for key: %s", key)
	}

	v := reflect.ValueOf(config).Elem()
	field := v.FieldByName(fieldName)

	if !field.IsValid() {
		return fmt.Errorf("no such field: %s in struct", fieldName)
	}

	if !field.CanSet() {
		return fmt.Errorf("cannot set field: %s", fieldName)
	}

	fieldValue := reflect.ValueOf(value)
	if field.Type() != fieldValue.Type() {
		return fmt.Errorf("provided value type doesn't match field type")
	}

	field.Set(fieldValue)
	return nil
}


func parseOidc(ctx context.Context) ([]auth.OidcConfig, error) {
    parsedList := []auth.OidcConfig{}

    if len(flagAuthOidc) != 0 {
        fmt.Printf("flagAuthOidc: %v\n", flagAuthOidc)
        for _, oidcConfig := range flagAuthOidc {
            parsed := &auth.OidcConfig{
                ClientID:    "",
                Issuer:      "",
                Scopes:      []string{},
                LoginGrants: flagLoginGrantTypes,
                LoginPorts:  flagLoginPorts,
            }

            pairs := strings.Split(oidcConfig, ";")

            for _, pair := range pairs {
                if pair == "" {
                    continue
                }
                kv := strings.SplitN(pair, "=", 2)
                if len(kv) != 2 {
                    return nil, fmt.Errorf("invalid key-value pair: %s", pair)
                }
                key := strings.TrimSpace(kv[0])
                value := strings.TrimSpace(kv[1])

                if key == "scopes" || key == "login_grants" {
                    err := setFieldByKey(parsed, key, strings.Split(value, ","))
                    if err != nil {
                        return nil, fmt.Errorf("invalid OIDC configuration %s: %w", key, err)
                    }
                } else if key == "login_ports" {
                    ports := strings.Split(value, ",")
                    intPorts := []int{}
                    for _, port := range ports {
                        intPort, err := strconv.Atoi(strings.TrimSpace(port))
                        if err != nil {
                            return nil, fmt.Errorf("invalid port value: %s", port)
                        }
                        intPorts = append(intPorts, intPort)
                    }
                    err := setFieldByKey(parsed, key, intPorts)
                    if err != nil {
                        return nil, fmt.Errorf("invalid OIDC configuration %s: %w", key, err)
                    }
                } else {
                    err := setFieldByKey(parsed, key, value)
                    if err != nil {
                        return nil, fmt.Errorf("invalid OIDC configuration %s: %w", key, err)
                    }
                }
            }
            parsedList = append(parsedList, *parsed)
        }
	} else {
		slog.Debug("setting up oidc auth",
            slog.String("client-id", flagAuthOidcClientId),
            slog.String("issuer", flagAuthOidcIssuer),
            slog.Any("login-grant", flagLoginGrantTypes),
            slog.Any("login-ports", flagLoginPorts),
            slog.Any("scopes", flagAuthOidcScopes),
        )

	    parsed :=  &auth.OidcConfig{
	        ClientID: flagAuthOidcClientId,
	        Issuer:    flagAuthOidcIssuer,
	        Scopes:    flagAuthOidcScopes,
	        LoginGrants: flagLoginGrantTypes,
	        LoginPorts:  flagLoginPorts,
	    }
	    parsedList = append(parsedList, *parsed)
	}

	return parsedList, nil
}

func setupOidc(ctx context.Context) ([]auth.Provider, []*discovery.LoginV1, error) {
	authCtx, cancelAuthCtx := context.WithTimeout(ctx, 15*time.Second)
	defer cancelAuthCtx()

	oidcConfigs, error := parseOidc(ctx)
	if error != nil {
        return nil, nil, fmt.Errorf("failed to parse OIDC configuration: %w", error)
    }

	providers := []auth.Provider{}
	logins := []*discovery.LoginV1{}

	for _, config := range oidcConfigs {
        slog.Debug("setting up oidc auth", slog.Any("config", config))
        provider, err := auth.NewOidcProvider(authCtx, config.Issuer, config.ClientID)
        if err != nil {
            return nil, nil, fmt.Errorf("failed to set up oidc provider: %w", err)
        }

        login := &discovery.LoginV1{
            Client:     config.ClientID,
            GrantTypes: config.LoginGrants,
            Authz:      provider.AuthURL(),
            Token:      provider.TokenURL(),
            Ports:      config.LoginPorts,
            Scopes:     config.Scopes,
        }
        providers = append(providers, provider)
        logins = append(logins, login)
    }

	return providers, logins, nil
}

func setupOkta() ([]auth.Provider, []*discovery.LoginV1) {
	slog.Debug("setting up okta auth", slog.String("issuer", flagAuthOktaIssuer), slog.String("client-id", flagAuthOktaClientId))
	slog.Warn("Okta auth is deprecated, please migrate to OIDC auth")
	p := []auth.Provider{auth.NewOktaProvider(flagAuthOktaIssuer, flagAuthOktaClaims...)}
	login := []*discovery.LoginV1{&discovery.LoginV1{
            Client:     flagAuthOktaClientId,
            GrantTypes: flagLoginGrantTypes,
            Authz:      flagAuthOktaAuthz,
            Token:      flagAuthOktaToken,
            Ports:      flagLoginPorts,
            Scopes:     flagLoginScopes,
        },
	}

	return p, login
}

func authMiddleware(ctx context.Context) (endpoint.Middleware, []*discovery.LoginV1, error) {
	providers := []auth.Provider{}

	if flagAuthStaticTokens != nil {
		providers = append(providers, auth.NewStaticProvider(flagAuthStaticTokens...))
	}

	// Check if OIDC or Okta are configured, we only want to allow one at a time.
	// OIDC is recommended, we want to deprecate our Okta-specific implementation and use our OIDC implementation instead, which Okta also supports.
	if (flagAuthOidcIssuer != "" || len(flagAuthOidc) > 0) && flagAuthOktaIssuer != "" {
		return nil, nil, errors.New("both OIDC and Okta are configured, only one is allowed at a time")
	}

	// We construct the discovery.LoginV1 on this level, as we need the OIDC provider to look up the
	// authorization and token endpoints dynamically to populate the LoginV1
	var logins []*discovery.LoginV1
	if flagAuthOidcIssuer != "" || len(flagAuthOidc) > 0 || flagAuthOktaIssuer != "" {
		var ps []auth.Provider
		if flagAuthOidcIssuer != "" || len(flagAuthOidc) > 0 {
			var err error
			ps, logins, err = setupOidc(ctx)
			if err != nil {
				return nil, nil, err
			}
		} else if flagAuthOktaIssuer != "" {
			ps, logins = setupOkta()
		}
		providers = append(providers, ps...)
	}

    for _, login := range logins {
        if login != nil { // Can be nil if neither Oidc, Okta, nor API token are configured
            if err := login.Validate(); err != nil {
                return nil, nil, err
            }
        }
	}

	return auth.Middleware(providers...), logins, nil
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

func registerDiscovery(mux *http.ServeMux, logins []*discovery.LoginV1) error {
	options := []discovery.Option{
		discovery.WithModulesV1(fmt.Sprintf("%s/", prefixModules)),
		discovery.WithProvidersV1(fmt.Sprintf("%s/", prefixProviders)),
	}

	for _, login := range logins {
	    options = append(options, discovery.WithLoginV1(login))
	}

	terraformJSON, err := json.Marshal(discovery.NewDiscovery(options...))
	if err != nil {
		return err
	}

	mux.HandleFunc("/.well-known/terraform.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "application/json")
		w.Write(terraformJSON)
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
