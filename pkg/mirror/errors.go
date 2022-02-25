package mirror

import (
	"errors"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
)

// Storage errors.
var (
	ErrAlreadyExists = errors.New("provider already exists")
	ErrListFailed    = errors.New("failed to list provider versions")
)

// Transport errors.
var (
	ErrVarMissing = errors.New("variable missing")
)

// ErrMirrorFailed is used to indicate that the component failed to look up a provider both upstream and in the mirror
type ErrMirrorFailed struct {
	provider core.Provider
	errors   []error
}

func (e *ErrMirrorFailed) Error() string {
	return fmt.Sprintf("lookup of provider failed: first error: %v; second error: %v", e.errors[0], e.errors[1])
}
