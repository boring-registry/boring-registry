package mirror

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/TierMobility/boring-registry/pkg/auth"
	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
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
func MakeHandler(svc Service, auth endpoint.Middleware, options ...httptransport.ServerOption) http.Handler {
	r := mux.NewRouter().StrictSlash(true)

	r.Methods("GET").Path(`/{hostname}/{namespace}/{name}/index.json`).Handler(
		httptransport.NewServer(
			auth(listProviderVersionsEndpoint(svc)),
			decodeListVersionsRequest,
			EncodeJSONResponse,
			append(
				options,
				httptransport.ServerBefore(extractMuxVars(varHostname, varNamespace, varName)),
				httptransport.ServerBefore(jwt.HTTPToContext()),
			)...,
		),
	)

	r.Methods("GET").Path(`/{hostname}/{namespace}/{name}/{version}.json`).Handler(
		httptransport.NewServer(
			auth(listProviderInstallationEndpoint(svc)),
			decodeListInstallationRequest,
			EncodeJSONResponse,
			append(
				options,
				httptransport.ServerBefore(extractMuxVars(varHostname, varNamespace, varName, varVersion)),
				httptransport.ServerBefore(jwt.HTTPToContext()),
			)...,
		),
	)

	r.Methods("GET").Path(`/{hostname}/{namespace}/{name}/terraform-provider-{nameplaceholder}_{version}_{os}_{architecture}.zip`).Handler(
		httptransport.NewServer(
			auth(retrieveProviderArchiveEndpoint(svc)),
			decodeRetrieveProviderArchiveRequest,
			encodeMirroredResponse,
			append(
				options,
				httptransport.ServerBefore(extractMuxVars(varHostname, varNamespace, varName, varVersion, varOS, varArchitecture)),
			)...,
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

// EncodeJSONResponse is a duplicate of httptransport.EncodeJSONResponse but uses the content-type expected by Terraform
func EncodeJSONResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	if headerer, ok := response.(httptransport.Headerer); ok {
		for k, values := range headerer.Headers() {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
	}
	code := http.StatusOK
	if sc, ok := response.(httptransport.StatusCoder); ok {
		code = sc.StatusCode()
	}
	w.WriteHeader(code)
	if code == http.StatusNoContent {
		return nil
	}
	return json.NewEncoder(w).Encode(response)
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

// ErrorEncoder translates domain specific errors to HTTP status codes.
func ErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	var providerErr *core.ProviderError
	if errors.As(err, &providerErr) {
		w.WriteHeader(providerErr.StatusCode)
	} else if errors.Is(err, ErrVarMissing) {
		w.WriteHeader(http.StatusBadRequest)
	} else if errors.Is(err, auth.ErrInvalidToken) {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	_ = json.NewEncoder(w).Encode(struct {
		Errors []string `json:"errors"`
	}{
		Errors: []string{
			err.Error(),
		},
	})
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
		return nil, fmt.Errorf("status code is %d instead of 200", r.StatusCode)
	}

	var response core.Provider
	if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}

func decodeUpstreamListProviderVersionsResponse(r *http.Response) (*core.ProviderVersions, error) {
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code is %d instead of 200", r.StatusCode)
	}

	var response core.ProviderVersions
	if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}
