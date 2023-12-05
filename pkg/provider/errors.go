package provider

import "errors"

var (
	ErrProviderNotFound = errors.New("failed to locate provider")
)

// Transport errors.
var (
	ErrVarMissing = errors.New("variable missing")
)
