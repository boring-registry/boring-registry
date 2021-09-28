package provider

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/TierMobility/boring-registry/pkg/auth"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

type muxVar string

const (
	varNamespace muxVar = "namespace"
	varName      muxVar = "name"
	varOS        muxVar = "os"
	varArch      muxVar = "arch"
	varVersion   muxVar = "version"
)

type header string

// MakeHandler returns a fully initialized http.Handler.
func MakeHandler(svc Service, auth endpoint.Middleware, options ...httptransport.ServerOption) http.Handler {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path(`/{namespace}/{name}/versions`).Handler(
		httptransport.NewServer(
			auth(listEndpoint(svc)),
			decodeListRequest,
			httptransport.EncodeJSONResponse,
			append(
				options,
				httptransport.ServerBefore(extractMuxVars(varNamespace, varName)),
				httptransport.ServerBefore(extractHeaders("Authorization")),
			)...,
		),
	)

	r.Methods("GET").Path(`/{namespace}/{name}/{version}/download/{os}/{arch}`).Handler(
		httptransport.NewServer(
			auth(downloadEndpoint(svc)),
			decodeDownloadRequest,
			httptransport.EncodeJSONResponse,
			append(
				options,
				httptransport.ServerBefore(extractMuxVars(varNamespace, varName, varOS, varArch, varVersion)),
				httptransport.ServerBefore(extractHeaders("Authorization")),
			)...,
		),
	)

	return r
}

func decodeListRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	namespace, ok := ctx.Value(varNamespace).(string)
	if !ok {
		return nil, errors.Wrap(ErrVarMissing, "namespace")
	}

	name, ok := ctx.Value(varName).(string)
	if !ok {
		return nil, errors.Wrap(ErrVarMissing, "name")
	}

	return listRequest{
		namespace: namespace,
		name:      name,
	}, nil
}

func decodeDownloadRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	namespace, ok := ctx.Value(varNamespace).(string)
	if !ok {
		return nil, errors.Wrap(ErrVarMissing, "namespace")
	}

	name, ok := ctx.Value(varName).(string)
	if !ok {
		return nil, errors.Wrap(ErrVarMissing, "name")
	}

	version, ok := ctx.Value(varVersion).(string)
	if !ok {
		return nil, errors.Wrap(ErrVarMissing, "version")
	}

	os, ok := ctx.Value(varOS).(string)
	if !ok {
		return nil, errors.Wrap(ErrVarMissing, "os")
	}

	arch, ok := ctx.Value(varArch).(string)
	if !ok {
		return nil, errors.Wrap(ErrVarMissing, "arch")
	}

	return downloadRequest{
		namespace: namespace,
		name:      name,
		version:   version,
		os:        os,
		arch:      arch,
	}, nil
}

// ErrorEncoder translates domain specific errors to HTTP status codes.
func ErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	switch errors.Cause(err) {
	case ErrVarMissing:
		w.WriteHeader(http.StatusBadRequest)
	case auth.ErrInvalidKey:
		w.WriteHeader(http.StatusUnauthorized)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{
		Error: err.Error(),
	})
}

func extractHeaders(keys ...header) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		for _, k := range keys {
			if v := r.Header.Get(string(k)); v != "" {
				ctx = context.WithValue(ctx, k, v)
			}
		}

		return ctx
	}
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
