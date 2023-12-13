package provider

import "errors"

var (
	// Provider errors
	ErrProviderNotFound = errors.New("failed to locate provider")
)
