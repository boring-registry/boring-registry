package module

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
)

type muxVar string

const (
	varNamespace muxVar = "namespace"
	varName      muxVar = "name"
	varProvider  muxVar = "provider"
	varVersion   muxVar = "version"
)

// MakeHandler returns a fully initialized http.Handler.
func MakeHandler(svc Service, auth endpoint.Middleware, metrics *o11y.ModuleMetrics, instrumentation o11y.Middleware, options ...httptransport.ServerOption) http.Handler {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path(`/{namespace}/{name}/{provider}/versions`).Handler(
		instrumentation.WrapHandler(
			httptransport.NewServer(
				auth(listEndpoint(svc, metrics)),
				decodeListRequest,
				httptransport.EncodeJSONResponse,
				append(
					options,
					httptransport.ServerBefore(extractMuxVars(varNamespace, varName, varProvider)),
					httptransport.ServerBefore(jwt.HTTPToContext()),
				)...,
			),
		),
	)

	r.Methods("GET").Path(`/{namespace}/{name}/{provider}/{version}/download`).Handler(
		instrumentation.WrapHandler(
			httptransport.NewServer(
				auth(downloadEndpoint(svc, metrics)),
				decodeDownloadRequest,
				encodeDownloadResponse,
				append(
					options,
					httptransport.ServerBefore(extractMuxVars(varNamespace, varName, varProvider, varVersion)),
					httptransport.ServerBefore(jwt.HTTPToContext()),
				)...,
			),
		),
	)

	return r
}

func decodeListRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	namespace, ok := ctx.Value(varNamespace).(string)
	if !ok {
		return nil, fmt.Errorf("%w: namespace", core.ErrVarMissing)
	}

	name, ok := ctx.Value(varName).(string)
	if !ok {
		return nil, fmt.Errorf("%w: name", core.ErrVarMissing)
	}

	provider, ok := ctx.Value(varProvider).(string)
	if !ok {
		return nil, fmt.Errorf("%w: provider", core.ErrVarMissing)
	}

	return listRequest{
		namespace: namespace,
		name:      name,
		provider:  provider,
	}, nil
}

func decodeDownloadRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	namespace, ok := ctx.Value(varNamespace).(string)
	if !ok {
		return nil, fmt.Errorf("%w: namespace", core.ErrVarMissing)
	}

	name, ok := ctx.Value(varName).(string)
	if !ok {
		return nil, fmt.Errorf("%w: names", core.ErrVarMissing)
	}

	provider, ok := ctx.Value(varProvider).(string)
	if !ok {
		return nil, fmt.Errorf("%w: provider", core.ErrVarMissing)
	}

	version, ok := ctx.Value(varVersion).(string)
	if !ok {
		return nil, fmt.Errorf("%w: version", core.ErrVarMissing)
	}

	proxyUrl := strings.Replace(r.URL.String(), "/download", "/proxy", 1)

	return downloadRequest{
		namespace: namespace,
		name:      name,
		provider:  provider,
		version:   version,
		proxyUrl:  proxyUrl,
	}, nil
}

// ErrorEncoder translates domain specific errors to HTTP status codes
func ErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {

	if errors.Is(err, ErrModuleNotFound) {
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(core.GenericError(err))
	}

	core.HandleErrorResponse(err, w)
}

func extractMuxVars(keys ...muxVar) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		for _, k := range keys {
			if v, ok := mux.Vars(r)[string(k)]; ok {
				ctx = context.WithValue(ctx, k, v)
			}
		}

		return ctx
	}
}

func encodeDownloadResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	res := response.(downloadResponse)
	w.Header().Set("X-Terraform-Get", res.url)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
