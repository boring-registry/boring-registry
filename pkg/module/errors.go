package module

import "errors"

var (
	// Module errors
	ErrModuleNotFound      = errors.New("failed to locate module")
	ErrModuleUploadFailed  = errors.New("failed to upload module")
	ErrModuleAlreadyExists = errors.New("module already exists")
	ErrModuleListFailed    = errors.New("failed to list module versions")
)
