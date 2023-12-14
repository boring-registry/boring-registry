package storage

import (
	"net/http"

	"github.com/boring-registry/boring-registry/pkg/core"
)

func noMatchingProviderFound(provider *core.Provider) error {
	return &core.ProviderError{
		Reason:     "failed to find matching providers",
		Provider:   provider,
		StatusCode: http.StatusNotFound,
	}
}
