package module

import "errors"

// Registry errors.
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

// Middleware errors.
var (
	ErrInvalidKey = errors.New("invalid key")
)
