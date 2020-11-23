package module

import "errors"

var (
	// Registry errors.

	ErrAlreadyExists = errors.New("module already exists")
	ErrNotFound      = errors.New("failed to locate module")
	ErrUploadFailed  = errors.New("failed to upload module")
	ErrListFailed    = errors.New("failed to list module versions")
)
