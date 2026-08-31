package storage

import (
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"

	assertion "github.com/stretchr/testify/assert"
)

func TestUnmarshalSigningKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		body        string
		expected    []core.GPGPublicKey
		expectError bool
	}{
		{
			name:        "invalid json",
			body:        `not json`,
			expectError: true,
		},
		{
			name:     "empty gpg_public_keys",
			body:     `{"gpg_public_keys":[]}`,
			expected: []core.GPGPublicKey{},
		},
		{
			name:     "empty object",
			body:     `{}`,
			expected: []core.GPGPublicKey{},
		},
		{
			name: "core.SigningKeys format",
			body: `{"gpg_public_keys":[{"key_id":"51852D87348FFC4C","ascii_armor":"-----BEGIN PGP PUBLIC KEY BLOCK-----"}]}`,
			expected: []core.GPGPublicKey{
				{
					KeyID:      "51852D87348FFC4C",
					ASCIIArmor: "-----BEGIN PGP PUBLIC KEY BLOCK-----",
				},
			},
		},
		{
			name: "legacy core.GPGPublicKey format",
			body: `{"key_id":"51852D87348FFC4C","ascii_armor":"-----BEGIN PGP PUBLIC KEY BLOCK-----"}`,
			expected: []core.GPGPublicKey{
				{
					KeyID:      "51852D87348FFC4C",
					ASCIIArmor: "-----BEGIN PGP PUBLIC KEY BLOCK-----",
				},
			},
		},
		{
			// The signing keys are needed to mirror a provider, so an incomplete
			// legacy document has to unmarshal into an empty set instead of failing
			name:     "legacy core.GPGPublicKey format without ascii_armor",
			body:     `{"key_id":"51852D87348FFC4C"}`,
			expected: []core.GPGPublicKey{},
		},
		{
			name:     "legacy core.GPGPublicKey format without key_id",
			body:     `{"ascii_armor":"-----BEGIN PGP PUBLIC KEY BLOCK-----"}`,
			expected: []core.GPGPublicKey{},
		},
	}

	assert := assertion.New(t)
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			signingKeys, err := unmarshalSigningKeys([]byte(tc.body))
			if tc.expectError {
				assert.Error(err)
				return
			}

			assert.NoError(err)
			if !assert.NotNil(signingKeys) {
				return
			}
			assert.Equal(tc.expected, signingKeys.GPGPublicKeys)
		})
	}
}
