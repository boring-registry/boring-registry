package storage

import (
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
)

type ErrProviderNotMirrored struct {
	// Provider is used to provide more metadata to the Error method.
	Provider core.Provider

	// Err is the underlying error that occurred during the operation.
	Err error
}

func (e ErrProviderNotMirrored) Error() string {
	return fmt.Sprintf("mirrored provider not found: %s/%s/%s: err: %s", e.Provider.Hostname, e.Provider.Namespace, e.Provider.Name, e.Err)
}

func (e ErrProviderNotMirrored) Unwrap() error {
	return e.Err
}
