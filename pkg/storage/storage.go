package storage

import (
	"encoding/json"
	"fmt"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/mirror"
	"github.com/boring-registry/boring-registry/pkg/module"
	"github.com/boring-registry/boring-registry/pkg/provider"
)

const (
	DefaultModuleArchiveFormat = "tar.gz"
)

type Storage interface {
	provider.Storage
	module.Storage
	mirror.Storage
}

// unmarshalSigningKeys tries to unmarshal the byte-array into core.SigningKeys, and if that fails into core.GPGPublicKey.
// A full core.SigningKeys is always returned for backward-compatibility reasons.
func unmarshalSigningKeys(b []byte) (*core.SigningKeys, error) {
	// Try to unmarshal into SigningKeys. Will not error even if no attribute of the source will match in the destination
	var signingKeys core.SigningKeys
	if err := json.Unmarshal(b, &signingKeys); err != nil {
		return nil, err
	}

	// The SigningKey from the storage backend is not in the core.SigningKey format.
	// Therefore, we try to unmarshal into core.GPGPublicKey format for legacy reasons.
	if signingKeys.GPGPublicKeys == nil {
		var gpgPublicKey core.GPGPublicKey
		if gpgPublicKeyErr := json.Unmarshal(b, &gpgPublicKey); gpgPublicKeyErr != nil {
			return nil, gpgPublicKeyErr
		} else if gpgPublicKey.KeyID == "" || gpgPublicKey.ASCIIArmor == "" {
			return nil, fmt.Errorf("the signing key key_ID or ascii_armor is empty")
		}

		signingKeys.GPGPublicKeys = append(signingKeys.GPGPublicKeys, gpgPublicKey)
	}

	return &signingKeys, nil
}
