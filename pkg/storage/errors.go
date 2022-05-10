package storage

import "errors"

// Storage errors.
var (
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
