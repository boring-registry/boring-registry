package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	"github.com/go-kit/kit/auth/jwt"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
)

type muxVar string

const (
	varUrl muxVar = "url"
)

// MakeHandler returns a fully initialized http.Handler.
func MakeHandler(storage Storage, metrics *o11y.ProxyMetrics, instrumentation o11y.Middleware, options ...httptransport.ServerOption) http.Handler {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path(`/{url:.*}`).Handler(
		instrumentation.WrapHandler(
			httptransport.NewServer(
				proxyEndpoint(storage, metrics),
				decodeProxyRequest,
				copyHeadersAndBody,
				append(
					options,
					httptransport.ServerBefore(extractMuxVars(varUrl)),
					httptransport.ServerBefore(jwt.HTTPToContext()),
				)...,
			),
		),
	)

	return r
}

// decodeProxyRequest translates request's paths into an object representing the request
func decodeProxyRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	downloadUrl, ok := ctx.Value(varUrl).(string)
	if !ok {
		return nil, fmt.Errorf("%w: url", core.ErrVarMissing)
	}

	completeUrl := downloadUrl + "?" + r.URL.RawQuery

	return proxyRequest{
		url: completeUrl,
	}, nil
}

// copyHeadersAndBody copy status codes, headers and body from the downloadUrl's HTTP response into the our response
func copyHeadersAndBody(_ context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(proxyResponse)

	// Copy headers into reponse
	for k, v := range resp.Header {
		w.Header()[k] = v
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// And then copy the body
	_, err := io.Copy(w, resp.Body)
	if err != nil {
		return nil
	}

	// Close the body reader
	return resp.Body.Close()
}

// ErrorEncoder translates domain specific errors to HTTP status codes
func ErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	if errors.Is(err, ErrInvalidRequestUrl) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	} else if errors.Is(err, ErrCannotDownloadFile) {
		w.WriteHeader(http.StatusBadGateway)
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
