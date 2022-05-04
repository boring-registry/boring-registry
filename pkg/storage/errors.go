package storage

import "errors"

// Storage errors.
var (
	ErrAlreadyExists = errors.New("provider already exists")
	ErrNotFound      = errors.New("failed to locate provider")
	ErrListFailed    = errors.New("failed to list provider versions")
)

// Transport errors.
var (
	ErrVarMissing = errors.New("variable missing")
)
