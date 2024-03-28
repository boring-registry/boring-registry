package mirror

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
)

type muxVar string

const (
	varHostname     muxVar = "hostname"
	varNamespace    muxVar = "namespace"
	varName         muxVar = "name"
	varVersion      muxVar = "version"
	varOS           muxVar = "os"
	varArchitecture muxVar = "architecture"
)

// MakeHandler returns a fully initialized http.Handler.
func MakeHandler(svc Service, auth endpoint.Middleware, metrics *o11y.MirrorMetrics, instrumentation o11y.Middleware, options ...httptransport.ServerOption) http.Handler {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path(`/{hostname}/{namespace}/{name}/index.json`).Handler(
		instrumentation.WrapHandler(
			httptransport.NewServer(
				auth(listProviderVersionsEndpoint(svc, metrics)),
				decodeListVersionsRequest,
				httptransport.EncodeJSONResponse,
				append(
					options,
					httptransport.ServerBefore(extractMuxVars(varHostname, varNamespace, varName)),
					httptransport.ServerBefore(jwt.HTTPToContext()),
				)...,
			),
		),
	)

	r.Methods("GET").Path(`/{hostname}/{namespace}/{name}/{version}.json`).Handler(
		instrumentation.WrapHandler(
			httptransport.NewServer(
				auth(listProviderInstallationEndpoint(svc, metrics)),
				decodeListInstallationRequest,
				addAuthToken,
				append(
					options,
					httptransport.ServerBefore(extractMuxVars(varHostname, varNamespace, varName, varVersion)),
					httptransport.ServerBefore(jwt.HTTPToContext()),
				)...,
			),
		),
	)

	// If static auth is
	r.Methods("GET").Path(`/{hostname}/{namespace}/{name}/terraform-provider-{nameplaceholder}_{version}_{os}_{architecture}.zip`).Handler(
		instrumentation.WrapHandler(
			httptransport.NewServer(
				auth(retrieveProviderArchiveEndpoint(svc, metrics)),
				decodeRetrieveProviderArchiveRequest,
				encodeMirroredResponse,
				append(
					options,
					httptransport.ServerBefore(extractMuxVars(varHostname, varNamespace, varName, varVersion, varOS, varArchitecture)),
					httptransport.ServerBefore(tokenQueryParamToContext()),
				)...,
			),
		),
	)
	return r
}

func pathPortions(ctx context.Context) (string, string, string, error) {
	hostname, ok := ctx.Value(varHostname).(string)
	if !ok {
		return "", "", "", fmt.Errorf("hostname path portion missing")
	}

	namespace, ok := ctx.Value(varNamespace).(string)
	if !ok {
		return "", "", "", fmt.Errorf("namespace path portion missing")
	}

	name, ok := ctx.Value(varName).(string)
	if !ok {
		return "", "", "", fmt.Errorf("name path portion missing")
	}

	return hostname, namespace, name, nil
}

func decodeListVersionsRequest(ctx context.Context, _ *http.Request) (interface{}, error) {
	hostname, namespace, name, err := pathPortions(ctx)
	return listProviderVersionsRequest{
		Hostname:  hostname,
		Namespace: namespace,
		Name:      name,
	}, err
}

func decodeListInstallationRequest(ctx context.Context, _ *http.Request) (interface{}, error) {
	hostname, namespace, name, err := pathPortions(ctx)
	if err != nil {
		return nil, err
	}

	version, ok := ctx.Value(varVersion).(string)
	if !ok {
		return nil, fmt.Errorf("version path portion missing")
	}

	return listProviderInstallationRequest{
		Hostname:  hostname,
		Namespace: namespace,
		Name:      name,
		Version:   version,
	}, nil
}

func decodeRetrieveProviderArchiveRequest(ctx context.Context, _ *http.Request) (interface{}, error) {
	hostname, namespace, name, err := pathPortions(ctx)
	if err != nil {
		return nil, err
	}

	version, ok := ctx.Value(varVersion).(string)
	if !ok {
		return nil, fmt.Errorf("%s path portion missing", string(varVersion))
	}

	os, ok := ctx.Value(varOS).(string)
	if !ok {
		return nil, fmt.Errorf("%s path portion missing", string(varOS))
	}

	architecture, ok := ctx.Value(varArchitecture).(string)
	if !ok {
		return nil, fmt.Errorf("%s path portion missing", string(varArchitecture))
	}

	return retrieveProviderArchiveRequest{
		Hostname:     hostname,
		Namespace:    namespace,
		Name:         name,
		Version:      version,
		OS:           os,
		Architecture: architecture,
	}, nil

}

func addAuthToken(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	listResponse, ok := response.(*ListProviderInstallationResponse)
	if !ok {
		return errors.New("failed to type assert to listProviderInstallationResponse")
	}

	if !listResponse.isMirror {
		t := ctx.Value(jwt.JWTContextKey)
		token, ok := t.(string)
		if !ok {
			return errors.New("failed to type assert to string")
		}

		for k, a := range listResponse.Archives {
			parsed, err := url.Parse(a.Url)
			if err != nil {
				return err
			}
			parsed.RawQuery = fmt.Sprintf("token=%s", token)
			a.Url = parsed.String()
			listResponse.Archives[k] = a
		}
	}

	return httptransport.EncodeJSONResponse(ctx, w, listResponse)
}

// tokenQueryParamToContext extracts the `token` query parameter in case it exists
func tokenQueryParamToContext() httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		token := r.URL.Query().Get("token")
		if token == "" {
			return ctx
		}

		return context.WithValue(ctx, jwt.JWTContextKey, token)
	}
}

func encodeMirroredResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	archiveResponse, ok := response.(*retrieveProviderArchiveResponse)
	if !ok {
		return errors.New("failed to type assert to retrieveProviderArchiveResponse")
	}

	w.Header().Set("Location", archiveResponse.location)
	w.WriteHeader(http.StatusTemporaryRedirect)
	return nil
}

// ErrorEncoder translates domain specific errors to HTTP status codes
func ErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	var providerErr *core.ProviderError
	if errors.As(err, &providerErr) {
		w.WriteHeader(providerErr.StatusCode)
	} else if errors.Is(err, ErrUpstreamNotFound) {
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

func decodeUpstreamProviderResponse(r *http.Response) (*core.Provider, error) {
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status code is %d instead of 200", ErrUpstreamNotFound, r.StatusCode)
	}

	var response core.Provider
	if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}

func decodeUpstreamListProviderVersionsResponse(r *http.Response) (*core.ProviderVersions, error) {
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status code is %d instead of 200", ErrUpstreamNotFound, r.StatusCode)
	}

	var response core.ProviderVersions
	if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}
