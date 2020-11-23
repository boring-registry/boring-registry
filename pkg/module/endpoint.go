package module

import (
	"context"

	"github.com/go-kit/kit/endpoint"
)

type listRequest struct {
	namespace string
	name      string
	provider  string
}

type listResponseVersion struct {
	Version string `json:"version,omitempty"`
}

type listResponseModule struct {
	Versions []listResponseVersion `json:"versions,omitempty"`
}

type listResponse struct {
	Modules []listResponseModule `json:"modules,omitempty"`
}

func listEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(listRequest)

		res, err := svc.ListModuleVersions(ctx, req.namespace, req.name, req.provider)
		if err != nil {
			return nil, err
		}

		var versions []listResponseVersion

		for _, module := range res {
			versions = append(versions, listResponseVersion{
				Version: module.Version,
			})
		}

		return listResponse{
			Modules: []listResponseModule{
				{
					Versions: versions,
				},
			},
		}, nil
	}
}

type downloadRequest struct {
	namespace string
	name      string
	provider  string
	version   string
}

type downloadResponse struct{ url string }

func downloadEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(downloadRequest)

		res, err := svc.GetModule(ctx, req.namespace, req.name, req.provider, req.version)
		if err != nil {
			return nil, err
		}

		return downloadResponse{
			url: res.DownloadURL,
		}, nil
	}
}
