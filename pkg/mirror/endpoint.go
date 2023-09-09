package mirror

import (
	"context"
	"errors"
	"fmt"

	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/kit/endpoint"
)

type listVersionsRequest struct {
	Hostname  string `json:"hostname,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

type listVersionsResponse struct {
	Versions map[string]EmptyObject `json:"versions"`
}

//func (l listVersionsResponse) Headers() http.Header {
//	return map[string][]string{http.CanonicalHeaderKey("content-type"): {"application/json"}}
//}

func listProviderVersionsEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(listVersionsRequest)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for listVersionsRequest")
		}

		provider := &core.Provider{
			Hostname:  req.Hostname,
			Namespace: req.Namespace,
			Name:      req.Name,
		}

		if provider.Hostname == "" || provider.Namespace == "" || provider.Name == "" {
			return nil, ErrVarMissing
		}

		versions, err := svc.ListProviderVersions(ctx, provider)
		if err != nil {
			return nil, err
		}

		return listVersionsResponse{
			Versions: versions.Versions,
		}, nil
	}
}

type listProviderInstallationRequest struct {
	Hostname  string `json:"hostname,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
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

		archives, err := svc.ListProviderInstallation(ctx, provider)
		if err != nil {
			return nil, err
		}

		return archives, nil
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

// Copied from provider module
type downloadResponse struct {
	OS                  string `json:"os"`
	Arch                string `json:"arch"`
	Filename            string `json:"filename"`
	DownloadURL         string `json:"download_url"`
	Shasum              string `json:"shasum"`
	ShasumsURL          string `json:"shasums_url"`
	ShasumsSignatureURL string `json:"shasums_signature_url"`
	//SigningKeys         SigningKeys `json:"signing_keys"`
}
