package provider

import (
	"context"
	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/kit/endpoint"
)

type listRequest struct {
	namespace string
	name      string
}

type listResponse struct {
	Versions []listResponseVersion `json:"versions,omitempty"`
}

type listResponseVersion struct {
	Version   string          `json:"version,omitempty"`
	Platforms []core.Platform `json:"platforms,omitempty"`
}

func listEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(listRequest)

		res, err := svc.ListProviderVersions(ctx, req.namespace, req.name)
		if err != nil {
			return nil, err
		}

		var versions []listResponseVersion

		for _, provider := range res {
			versions = append(versions, listResponseVersion{
				Version:   provider.Version,
				Platforms: provider.Platforms,
			})
		}

		return listResponse{
			Versions: versions,
		}, nil
	}
}

type downloadRequest struct {
	namespace string
	name      string
	version   string
	os        string
	arch      string
}

type downloadResponse struct {
	OS                  string           `json:"os"`
	Arch                string           `json:"arch"`
	Filename            string           `json:"filename"`
	DownloadURL         string           `json:"download_url"`
	Shasum              string           `json:"shasum"`
	ShasumsURL          string           `json:"shasums_url"`
	ShasumsSignatureURL string           `json:"shasums_signature_url"`
	SigningKeys         core.SigningKeys `json:"signing_keys"`
}

func downloadEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(downloadRequest)

		res, err := svc.GetProvider(ctx, req.namespace, req.name, req.version, req.os, req.arch)
		if err != nil {
			return nil, err
		}

		return downloadResponse{
			OS:                  res.OS,
			Arch:                res.Arch,
			DownloadURL:         res.DownloadURL,
			Filename:            res.Filename,
			Shasum:              res.SHASum,
			SigningKeys:         res.SigningKeys,
			ShasumsURL:          res.SHASumsURL,
			ShasumsSignatureURL: res.SHASumsSignatureURL,
		}, nil
	}
}
