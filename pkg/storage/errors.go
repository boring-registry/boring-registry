package storage

import (
	"errors"
	"net/http"

	"github.com/TierMobility/boring-registry/pkg/core"
)

// Storage errors.
var (
	ErrObjectAlreadyExists = errors.New("object already exists")

	// module errors
	ErrModuleUploadFailed  = errors.New("failed to upload module")
	ErrModuleAlreadyExists = errors.New("module already exists")
	ErrModuleNotFound      = errors.New("failed to locate module")
	ErrModuleListFailed    = errors.New("failed to list module versions")

	// provider errors
	ErrProviderAlreadyExists = errors.New("provider already exists")
	ErrProviderNotFound      = errors.New("failed to locate provider")
	ErrProviderListFailed    = errors.New("failed to list provider versions")
)

// Transport errors.
var (
	ErrVarMissing = errors.New("variable missing")
)

func noMatchingProviderFound(provider *core.Provider) error {
	return &core.ProviderError{
		Reason:     "failed to find matching providers",
		Provider:   provider,
		StatusCode: http.StatusNotFound,
	}
}
