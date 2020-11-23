package module

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

type muxVar string

const (
	varNamespace muxVar = "namespace"
	varName      muxVar = "name"
	varProvider  muxVar = "provider"
	varVersion   muxVar = "version"
)

type header string

const (
	headerAuthorization header = "Authorization"
)

// MakeHandler returns a fully initialized http.Handler.
func MakeHandler(svc Service, auth endpoint.Middleware, options ...httptransport.ServerOption) http.Handler {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path(`/modules/{namespace}/{name}/{provider}/versions`).Handler(
		httptransport.NewServer(
			auth(listEndpoint(svc)),
			decodeListRequest,
			httptransport.EncodeJSONResponse,
			append(
				options,
				httptransport.ServerBefore(extractMuxVars(varNamespace, varName, varProvider)),
				httptransport.ServerBefore(extractHeaders("Authorization")),
			)...,
		),
	)

	r.Methods("GET").Path(`/modules/{namespace}/{name}/{provider}/{version}/download`).Handler(
		httptransport.NewServer(
			auth(downloadEndpoint(svc)),
			decodeDownloadRequest,
			encodeDownloadResponse,
			append(
				options,
				httptransport.ServerBefore(extractMuxVars(varNamespace, varName, varProvider, varVersion)),
				httptransport.ServerBefore(extractHeaders("Authorization")),
			)...,
		),
	)

	return r
}

func decodeListRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var req listRequest

	if v, ok := ctx.Value(varNamespace).(string); ok {
		req.namespace = v
	}

	if v, ok := ctx.Value(varName).(string); ok {
		req.name = v
	}

	if v, ok := ctx.Value(varProvider).(string); ok {
		req.provider = v
	}

	return req, nil
}

func decodeDownloadRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var req downloadRequest

	if v, ok := ctx.Value(varNamespace).(string); ok {
		req.namespace = v
	}

	if v, ok := ctx.Value(varName).(string); ok {
		req.name = v
	}

	if v, ok := ctx.Value(varProvider).(string); ok {
		req.provider = v
	}

	if v, ok := ctx.Value(varVersion).(string); ok {
		req.version = v
	}

	return req, nil
}

// ErrorEncoder translates domain specific errors to HTTP status codes.
func ErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	switch errors.Cause(err) {
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

func encodeDownloadResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	res := response.(downloadResponse)
	w.Header().Set("X-Terraform-Get", res.url)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
