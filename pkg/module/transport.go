package module

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/TierMobility/boring-registry/pkg/core"

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
func MakeHandler(svc Service, auth endpoint.Middleware, options ...httptransport.ServerOption) http.Handler {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path(`/{namespace}/{name}/{provider}/versions`).Handler(
		httptransport.NewServer(
			auth(listEndpoint(svc)),
			decodeListRequest,
			httptransport.EncodeJSONResponse,
			append(
				options,
				httptransport.ServerBefore(extractMuxVars(varNamespace, varName, varProvider)),
				httptransport.ServerBefore(jwt.HTTPToContext()),
			)...,
		),
	)

	r.Methods("GET").Path(`/{namespace}/{name}/{provider}/{version}/download`).Handler(
		httptransport.NewServer(
			auth(downloadEndpoint(svc)),
			decodeDownloadRequest,
			encodeDownloadResponse,
			append(
				options,
				httptransport.ServerBefore(extractMuxVars(varNamespace, varName, varProvider, varVersion)),
				httptransport.ServerBefore(jwt.HTTPToContext()),
			)...,
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

	return downloadRequest{
		namespace: namespace,
		name:      name,
		provider:  provider,
		version:   version,
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
