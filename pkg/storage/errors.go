package storage

import "fmt"

type ErrProviderNotMirrored struct {
	// Opts are the ProviderOpts used to provide more metadata to the Error method.
	Opts ProviderOpts

	// Err is the underlying error that occurred during the operation.
	Err error
}

func (e ErrProviderNotMirrored) Error() string {
	return fmt.Sprintf("mirrored provider not found: %s/%s/%s: err: %s", e.Opts.Hostname, e.Opts.Namespace, e.Opts.Name, e.Err)
}

func (e ErrProviderNotMirrored) Unwrap() error {
	return e.Err
}
