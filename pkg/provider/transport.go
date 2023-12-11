package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
)

type muxVar string

const (
	varNamespace muxVar = "namespace"
	varName      muxVar = "name"
	varOS        muxVar = "os"
	varArch      muxVar = "arch"
	varVersion   muxVar = "version"
)

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
				httptransport.ServerBefore(jwt.HTTPToContext()),
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
				httptransport.ServerBefore(jwt.HTTPToContext()),
			)...,
		),
	)

	return r
}

func decodeListRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	namespace, ok := ctx.Value(varNamespace).(string)
	if !ok {
		return nil, fmt.Errorf("%w: namespace", ErrVarMissing)
	}

	name, ok := ctx.Value(varName).(string)
	if !ok {
		return nil, fmt.Errorf("%w: name", ErrVarMissing)
	}

	return listRequest{
		namespace: namespace,
		name:      name,
	}, nil
}

func decodeDownloadRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	namespace, ok := ctx.Value(varNamespace).(string)
	if !ok {
		return nil, fmt.Errorf("%w: namespace", ErrVarMissing)
	}

	name, ok := ctx.Value(varName).(string)
	if !ok {
		return nil, fmt.Errorf("%w: name", ErrVarMissing)
	}

	version, ok := ctx.Value(varVersion).(string)
	if !ok {
		return nil, fmt.Errorf("%w: version", ErrVarMissing)
	}

	os, ok := ctx.Value(varOS).(string)
	if !ok {
		return nil, fmt.Errorf("%w: os", ErrVarMissing)
	}

	arch, ok := ctx.Value(varArch).(string)
	if !ok {
		return nil, fmt.Errorf("%w: arch", ErrVarMissing)
	}

	return downloadRequest{
		namespace: namespace,
		name:      name,
		version:   version,
		os:        os,
		arch:      arch,
	}, nil
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
