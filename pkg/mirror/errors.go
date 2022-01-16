package mirror

import "errors"

// Storage errors.
var (
	ErrAlreadyExists = errors.New("provider already exists")
	ErrListFailed    = errors.New("failed to list provider versions")
)

// Transport errors.
var (
	ErrVarMissing = errors.New("variable missing")
)
