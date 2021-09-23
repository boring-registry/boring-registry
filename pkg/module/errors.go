package module

import "errors"

// Storage errors.
var (
	ErrAlreadyExists = errors.New("module already exists")
	ErrNotFound      = errors.New("failed to locate module")
	ErrUploadFailed  = errors.New("failed to upload module")
	ErrListFailed    = errors.New("failed to list module versions")
)

// Transport errors.
var (
	ErrVarMissing = errors.New("variable missing")
)
