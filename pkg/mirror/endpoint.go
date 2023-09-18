package mirror

import (
	"context"
	"errors"
	"fmt"

	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/kit/endpoint"
)

type listProviderVersionsRequest struct {
	Hostname  string `json:"hostname,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// EmptyObject exists to return an `{}` JSON object to match the protocol spec
type EmptyObject struct{}

// ListProviderVersionsResponse holds the response that is passed to the endpoint
type ListProviderVersionsResponse struct {
	Versions map[string]EmptyObject `json:"versions"`

	// embedded struct to determine if the response was composed of providers from the mirror
	mirrorSource
}

func listProviderVersionsEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(listProviderVersionsRequest)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for listProviderVersionsRequest")
		}

		provider := &core.Provider{
			Hostname:  req.Hostname,
			Namespace: req.Namespace,
			Name:      req.Name,
		}

		if provider.Hostname == "" || provider.Namespace == "" || provider.Name == "" {
			return nil, ErrVarMissing
		}

		return svc.ListProviderVersions(ctx, provider)
	}
}

type listProviderInstallationRequest struct {
	Hostname  string `json:"hostname,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
}

type ListProviderInstallationResponse struct {
	Archives map[string]Archive `json:"archives"`

	// embedded struct to determine if the response was composed of providers from the mirror
	mirrorSource
}

type Archive struct {
	Url    string   `json:"url"`
	Hashes []string `json:"hashes,omitempty"`
}

func listProviderInstallationEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(listProviderInstallationRequest)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for listProviderInstallationRequest")
		}

		provider := &core.Provider{
			Hostname:  req.Hostname,
			Namespace: req.Namespace,
			Name:      req.Name,
			Version:   req.Version,
		}

		if provider.Hostname == "" || provider.Namespace == "" || provider.Name == "" || provider.Version == "" {
			return nil, errors.New("invalid parameters")
		}

		return svc.ListProviderInstallation(ctx, provider)
	}
}

type retrieveProviderArchiveRequest struct {
	Hostname     string `json:"hostname,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	Name         string `json:"name,omitempty"`
	Version      string `json:"version,omitempty"`
	OS           string `json:"os,omitempty"`
	Architecture string `json:"architecture,omitempty"`
}

type retrieveProviderArchiveResponse struct {
	location string

	// embedded struct to determine if the response was composed of providers from the mirror
	mirrorSource
}

func retrieveProviderArchiveEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(retrieveProviderArchiveRequest)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for retrieveProviderArchiveRequest")
		}

		provider := &core.Provider{
			Hostname:  req.Hostname,
			Namespace: req.Namespace,
			Name:      req.Name,
			Version:   req.Version,
			OS:        req.OS,
			Arch:      req.Architecture,
		}

		if provider.Hostname == "" || provider.Namespace == "" || provider.Name == "" || provider.Version == "" || provider.OS == "" || provider.Arch == "" {
			return nil, ErrVarMissing
		}

		return svc.RetrieveProviderArchive(ctx, provider)
	}
}
