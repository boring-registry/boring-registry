package proxy

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	"github.com/go-kit/kit/auth/jwt"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
)

type muxVar string

const (
	RootUrlContextKey muxVar = "rootUrl"

	varSignature muxVar = "signature"
	varExpiry    muxVar = "expiry"
	varUrl       muxVar = "url"
)

// MakeHandler returns a fully initialized http.Handler.
func MakeHandler(proxy ProxyUrlService, metrics *o11y.ProxyMetrics, instrumentation o11y.Middleware, options ...httptransport.ServerOption) http.Handler {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path(`/{signature}/{expiry}/{url}`).Handler(
		instrumentation.WrapHandler(
			httptransport.NewServer(
				proxyEndpoint(proxy, metrics),
				decodeProxyRequest,
				copyHeadersAndBody,
				append(
					options,
					httptransport.ServerBefore(extractMuxVars(varSignature, varExpiry, varUrl)),
					httptransport.ServerBefore(jwt.HTTPToContext()),
				)...,
			),
		),
	)

	return r
}

// decodeProxyRequest translates request's paths into an object representing the request
func decodeProxyRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	signature, ok := ctx.Value(varSignature).(string)
	if !ok {
		return nil, fmt.Errorf("%w: signature", core.ErrVarMissing)
	}

	expiryValue, ok := ctx.Value(varExpiry).(string)
	if !ok {
		return nil, fmt.Errorf("%w: expiry", core.ErrVarMissing)
	}

	expiry, err := strconv.ParseInt(expiryValue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: expiry", core.ErrVarType)
	}

	encodedUrl, ok := ctx.Value(varUrl).(string)
	if !ok {
		return nil, fmt.Errorf("%w: url", core.ErrVarMissing)
	}

	decodedUrl, err := base64.StdEncoding.DecodeString(encodedUrl)
	if err != nil {
		return nil, err
	}

	return proxyRequest{
		signature: signature,
		expiry:    expiry,
		url:       string(decodedUrl),
	}, nil
}

// copyHeadersAndBody copy status codes, headers and body from the downloadUrl's HTTP response into the our response
func copyHeadersAndBody(_ context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(proxyResponse)

	// Copy headers into reponse
	for k, v := range resp.Header {
		w.Header()[k] = v
	}

	// Copy  status code
	w.WriteHeader(resp.StatusCode)

	// And the copy the body
	_, err := io.Copy(w, resp.Body)

	// Close the body reader
	resp.Body.Close()

	return err
}

// ErrorEncoder translates domain specific errors to HTTP status codes
func ErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	// var proxyError *core.ProxyError
	if errors.Is(err, ErrExpiredUrl) {
		w.WriteHeader(http.StatusGone)
	} else if errors.Is(err, ErrInvalidSignature) {
		w.WriteHeader(http.StatusBadRequest)
	} else if errors.Is(err, ErrInvalidRequestUrl) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	} else if errors.Is(err, ErrCantDownloadFile) {
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

// ExtractRootUrl return an URl composed of the scheme (http or https) and the host of the incoming request
func ExtractRootUrl() httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		rootUrl := getRootURLFromRequest(r)

		// Add the rootUrl to the context
		ctx = context.WithValue(ctx, RootUrlContextKey, rootUrl)

		return ctx
	}
}

// Get the root part of the URL of the request
func getRootURLFromRequest(r *http.Request) string {
	// Find the protocol (http ou https)
	var protocol string
	if r.TLS != nil {
		protocol = "https://"
	} else {
		protocol = "http://"
	}

	// Build root URL
	rootUrl := protocol + r.Host
	return rootUrl
}
