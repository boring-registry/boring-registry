package core

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	assertion "github.com/stretchr/testify/assert"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

func decodeHexString(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func TestProvider_ArchiveFileName(t *testing.T) {
	t.Parallel()
	assert := assertion.New(t)

	testCases := []struct {
		name             string
		provider         Provider
		expectedFileName string
		expectedPanic    bool
	}{
		{
			name: "valid provider",
			provider: Provider{
				Name:    "random",
				Version: "2.0.0",
				OS:      "linux",
				Arch:    "amd64",
			},
			expectedFileName: "terraform-provider-random_2.0.0_linux_amd64.zip",
			expectedPanic:    false,
		},
		{
			name: "missing name",
			provider: Provider{
				Version: "2.0.0",
				OS:      "linux",
				Arch:    "amd64",
			},
			expectedPanic: true,
		},
		{
			name: "missing version",
			provider: Provider{
				Name: "random",
				OS:   "linux",
				Arch: "amd64",
			},
			expectedPanic: true,
		},
		{
			name: "missing os",
			provider: Provider{
				Name:    "random",
				Version: "2.0.0",
				Arch:    "amd64",
			},
			expectedPanic: true,
		},
		{
			name: "missing arch",
			provider: Provider{
				Name:    "random",
				Version: "2.0.0",
				OS:      "linux",
			},
			expectedPanic: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if tc.expectedPanic {
					if r := recover(); r == nil {
						t.Errorf("The code did not panic")
					}
				}
			}()

			fileName := tc.provider.ArchiveFileName()
			assert.Equal(tc.expectedFileName, fileName)
		})
	}
}

func TestProvider_ShasumFileName(t *testing.T) {
	t.Parallel()
	assert := assertion.New(t)

	testCases := []struct {
		name             string
		provider         Provider
		expectedFileName string
		expectedPanic    bool
	}{
		{
			name: "valid provider",
			provider: Provider{
				Name:    "random",
				Version: "2.0.0",
			},
			expectedFileName: "terraform-provider-random_2.0.0_SHA256SUMS",
			expectedPanic:    false,
		},
		{
			name: "missing name",
			provider: Provider{
				Version: "2.0.0",
			},
			expectedPanic: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if tc.expectedPanic {
					if r := recover(); r == nil {
						t.Errorf("The code did not panic")
					}
				}
			}()
			fileName := tc.provider.ShasumFileName()
			assert.Equal(tc.expectedFileName, fileName)
		})
	}
}

func TestProvider_ShasumSignatureFileName(t *testing.T) {
	t.Parallel()
	assert := assertion.New(t)

	testCases := []struct {
		name             string
		provider         Provider
		expectedFileName string
		expectedPanic    bool
	}{
		{
			name: "valid provider",
			provider: Provider{
				Name:    "random",
				Version: "2.0.0",
			},
			expectedFileName: "terraform-provider-random_2.0.0_SHA256SUMS.sig",
			expectedPanic:    false,
		},
		{
			name: "missing name",
			provider: Provider{
				Version: "2.0.0",
			},
			expectedPanic: true,
		},
		{
			name: "missing version",
			provider: Provider{
				Name: "random",
			},
			expectedPanic: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if tc.expectedPanic {
					if r := recover(); r == nil {
						t.Errorf("The code did not panic")
					}
				}
			}()
			fileName := tc.provider.ShasumSignatureFileName()
			assert.Equal(tc.expectedFileName, fileName)
		})
	}
}

func TestNewProviderFromArchive(t *testing.T) {
	t.Parallel()
	assert := assertion.New(t)

	testCases := []struct {
		name             string
		fileName         string
		expectedProvider Provider
		expectError      bool
	}{
		{
			name:     "valid filename",
			fileName: "terraform-provider-random_2.0.0_linux_amd64.zip",
			expectedProvider: Provider{
				Name:     "random",
				Version:  "2.0.0",
				OS:       "linux",
				Arch:     "amd64",
				Filename: "terraform-provider-random_2.0.0_linux_amd64.zip",
			},
			expectError: false,
		},
		{
			name:        "invalid filename",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			provider, err := NewProviderFromArchive(tc.fileName)
			if tc.expectError {
				assert.Error(err)
			} else {
				assert.Equal(tc.fileName, provider.Filename)
				assert.Equal(tc.expectedProvider.Name, provider.Name)
				assert.Equal(tc.expectedProvider.Version, provider.Version)
				assert.Equal(tc.expectedProvider.OS, provider.OS)
				assert.Equal(tc.expectedProvider.Arch, provider.Arch)
			}
		})
	}
}

func TestSigningKeys_IsValidSha256Sums(t *testing.T) {
	t.Parallel()

	c := &packet.Config{
		// using a pseudorandom generator, as I'm not sure if CI systems always have enough entropy
		Rand:    rand.New(rand.NewSource(42)),
		RSABits: 4096,
	}
	e, err := openpgp.NewEntity("boring-registry", "test", "boring-registry@example.com", c)
	if err != nil {
		panic(err)
	}

	buf := new(bytes.Buffer)
	w, err := armor.Encode(buf, openpgp.PublicKeyType, nil)
	if err != nil {
		panic(err)
	}
	if err := e.Serialize(w); err != nil {
		panic(err)
	}
	w.Close()

	b := []byte("{\"boring\":\"registry\"}")
	signatureBuffer := new(bytes.Buffer)
	if err := openpgp.DetachSignText(signatureBuffer, e, bytes.NewReader(b), nil); err != nil {
		panic(err)
	}

	testCases := []struct {
		name        string
		signingKeys SigningKeys
		sums        []byte
		sig         []byte
		expectError bool
	}{
		{
			name:        "SigningKeys without public keys",
			signingKeys: SigningKeys{},
			expectError: true,
		},
		{
			name: "broken ascii armored keyring",
			signingKeys: SigningKeys{
				GPGPublicKeys: []GPGPublicKey{
					{
						ASCIIArmor: "--- test ---",
					},
				},
			},
			sums:        b,
			sig:         signatureBuffer.Bytes(),
			expectError: true,
		},
		{
			name: "document and signature don't match",
			signingKeys: SigningKeys{
				GPGPublicKeys: []GPGPublicKey{
					{
						ASCIIArmor: buf.String(),
					},
				},
			},
			sums:        []byte("something"),
			sig:         signatureBuffer.Bytes(),
			expectError: true,
		},
		{
			name: "no public key matches",
			signingKeys: SigningKeys{
				GPGPublicKeys: []GPGPublicKey{
					{
						ASCIIArmor: `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
Fyw/Va0=
=o3Ms
-----END PGP PUBLIC KEY BLOCK-----`,
					},
				},
			},
			sums:        b,
			sig:         signatureBuffer.Bytes(),
			expectError: true,
		},
		{
			name: "valid signature",
			signingKeys: SigningKeys{
				GPGPublicKeys: []GPGPublicKey{
					{
						ASCIIArmor: buf.String(),
					},
				},
			},
			sums:        b,
			sig:         signatureBuffer.Bytes(),
			expectError: false,
		},
	}

	assert := assertion.New(t)
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.signingKeys.IsValidSha256Sums(tc.sums, tc.sig)
			if tc.expectError {
				assert.Error(err)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestProvider_Clone(t *testing.T) {
	type fields struct {
		Hostname            string
		Namespace           string
		Name                string
		Version             string
		OS                  string
		Arch                string
		Filename            string
		DownloadURL         string
		Shasum              string
		SHASumsURL          string
		SHASumsSignatureURL string
		SigningKeys         SigningKeys
		Platforms           []Platform
	}
	tests := []struct {
		name     string
		provider Provider
	}{
		{
			name: "provider without SigningKeys",
			provider: Provider{
				Hostname:  "registry.example.com",
				Namespace: "hashicorp",
				Name:      "random",
				Version:   "1.2.3",
				OS:        "linux",
				Arch:      "amd64",
			},
		},
		{
			name: "provider with all fields non-empty",
			provider: Provider{
				Hostname:            "registry.example.com",
				Namespace:           "hashicorp",
				Name:                "random",
				Version:             "1.2.3",
				OS:                  "linux",
				Arch:                "amd64",
				Filename:            "terraform-provider-random_2.0.0_linux_amd64.zip",
				DownloadURL:         "registry.example.com/terraform-provider-random_2.0.0_linux_amd64.zip",
				Shasum:              "123456789",
				SHASumsURL:          "registry.example.com/terraform-provider-random_2.0.0_SHA256SUMS",
				SHASumsSignatureURL: "registry.example.com/terraform-provider-random_2.0.0_SHA256SUMS.sig",
				SigningKeys: SigningKeys{
					GPGPublicKeys: []GPGPublicKey{
						{
							KeyID: "123456789",
							ASCIIArmor: `-----BEGIN PGP PUBLIC KEY BLOCK-----
-----END PGP PUBLIC KEY BLOCK-----`,
						},
					},
				},
				Platforms: []Platform{
					{
						OS:   "linux",
						Arch: "amd64",
					},
					{
						OS:   "linux",
						Arch: "arm64",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloned := tt.provider.Clone()
			assertion.Equalf(t, tt.provider, *cloned, "Clone()")
			assertion.Equal(t, tt.provider.Platforms, cloned.Platforms)
			assertion.Equal(t, tt.provider.SigningKeys.GPGPublicKeys, cloned.SigningKeys.GPGPublicKeys)
		})
	}
}

func Test_parseSha256Line(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		line         string
		wantBytes    []byte
		wantFileName string
		wantErr      assertion.ErrorAssertionFunc
	}{
		{
			name: "empty line",
			line: "",
			wantErr: func(t assertion.TestingT, err error, _ ...interface{}) bool {
				return assertion.Error(t, err)
			},
		},
		{
			name:         "valid line",
			line:         "5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a  terraform-provider-random_2.0.0_linux_amd64.zip",
			wantBytes:    decodeHexString("5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a"),
			wantFileName: "terraform-provider-random_2.0.0_linux_amd64.zip",
			wantErr: func(t assertion.TestingT, err error, _ ...interface{}) bool {
				return assertion.NoError(t, err)
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			b, fileName, err := parseSha256Line(tc.line)
			if !tc.wantErr(t, err, fmt.Sprintf("parseSha256Line(%v)", tc.line)) {
				return
			}
			assertion.Equalf(t, tc.wantBytes, b, "parseSha256Line(%v)", tc.line)
			assertion.Equalf(t, tc.wantFileName, fileName, "parseSha256Line(%v)", tc.line)
		})
	}
}

func TestSha256Sums_Name(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "valid file name",
			filename: "terraform-provider-random_2.0.0_SHA256SUMS",
			want:     "random",
		},
		{
			name:     "valid file name with underscore in provider name",
			filename: "terraform-provider-random_provider_2.0.0_SHA256SUMS",
			want:     "random_provider",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Sha256Sums{
				Filename: tc.filename,
			}
			n, err := s.Name()
			assertion.NoError(t, err)
			assertion.Equal(t, tc.want, n)
		})
	}
}

func TestSha256Sums_Checksum(t *testing.T) {
	const sha256Sums = `be3f1e818ca58a960fd1c80216a691bbd4827c505ab7916fb68ddd186032286e  terraform-provider-random_2.0.0_linux_386.zip
5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a  terraform-provider-random_2.0.0_linux_amd64.zip
29df160b8b618227197cc9984c47412461ad66a300a8fc1db4052398bf5656ac  terraform-provider-random_2.0.0_linux_arm.zip
`
	sums, err := NewSha256Sums("terraform-provider-random_2.0.0_SHA256SUMS", strings.NewReader(sha256Sums))
	if err != nil {
		panic(err)
	}

	tests := []struct {
		name     string
		fileName string
		want     string
		wantErr  assertion.ErrorAssertionFunc
	}{
		{
			name:     "file name is not in entries",
			fileName: "terraform-provider-dummy_2.0.0_linux_amd64.zip",
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.Error(t, err)
			},
		},
		{
			name:     "file name is in entries",
			fileName: "terraform-provider-random_2.0.0_linux_amd64.zip",
			want:     "5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a",
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.NoError(t, err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sums.Checksum(tt.fileName)
			if !tt.wantErr(t, err, fmt.Sprintf("Checksum(%v)", tt.fileName)) {
				return
			}
			assertion.Equalf(t, tt.want, got, "Checksum(%v)", tt.fileName)
		})
	}
}

func TestNewSha256Sums(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		filename string
		content  string
		want     *Sha256Sums
		wantErr  assertion.ErrorAssertionFunc
	}{
		{
			name: "empty filename",
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.Error(t, err)
			},
		},
		{
			name:     "invalid filename prefix",
			filename: "invalid-provider-random_0.1.0_SHA256SUMS",
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.Error(t, err)
			},
		},
		{
			name:     "invalid filename suffix",
			filename: "terraform-provider-random_0.1.0_SHA256SUMS.sig",
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.Error(t, err)
			},
		},
		{
			name:     "valid filename and invalid content",
			filename: "terraform-provider-random_0.1.0_SHA256SUMS",
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.Error(t, err)
			},
			content: `be3f1e818ca58a960fd1c80216a691bbd4827c505ab7916fb68ddd186032286e thisbreaksit test terraform-provider-random_2.0.0_linux_386.zip`,
		},
		{
			name:     "valid filename and content",
			filename: "terraform-provider-random_0.1.0_SHA256SUMS",
			content: `be3f1e818ca58a960fd1c80216a691bbd4827c505ab7916fb68ddd186032286e  terraform-provider-random_2.0.0_linux_386.zip
5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a  terraform-provider-random_2.0.0_linux_amd64.zip
29df160b8b618227197cc9984c47412461ad66a300a8fc1db4052398bf5656ac  terraform-provider-random_2.0.0_linux_arm.zip`,
			want: &Sha256Sums{
				Entries: map[string][]byte{
					"terraform-provider-random_2.0.0_linux_386.zip":   decodeHexString("be3f1e818ca58a960fd1c80216a691bbd4827c505ab7916fb68ddd186032286e"),
					"terraform-provider-random_2.0.0_linux_amd64.zip": decodeHexString("5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a"),
					"terraform-provider-random_2.0.0_linux_arm.zip":   decodeHexString("29df160b8b618227197cc9984c47412461ad66a300a8fc1db4052398bf5656ac"),
				},
				Filename: "terraform-provider-random_0.1.0_SHA256SUMS",
			},
			wantErr: func(t assertion.TestingT, err error, i ...interface{}) bool {
				return assertion.NoError(t, err)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewSha256Sums(tc.filename, strings.NewReader(tc.content))
			if !tc.wantErr(t, err) {
				return
			}
			assertion.Equal(t, tc.want, got)
		})
	}
}

func TestSha256Checksum(t *testing.T) {
	t.Parallel()

	type file struct {
		Name, Body string
	}

	testCases := []struct {
		name        string
		files       []file
		expected    []byte
		expectError bool
	}{
		{
			name: "valid shaSumsArchive",
			files: []file{
				{
					Name: "terraform-provider-dummy_v0.1.0",
					Body: "ThisIsASampleProvider",
				},
			},
			expected: []byte{0xb2, 0xdd, 0x50, 0xa4, 0x27, 0x2f, 0xcd, 0x59, 0x6d, 0x11, 0x29, 0x76, 0x17, 0xb7, 0x27, 0x54, 0xd6, 0xb8, 0xff, 0x98, 0x74, 0x78, 0x3a, 0x99, 0x5f, 0x63, 0x3f, 0xe1, 0x9, 0x41, 0x5, 0x7d},
		},
	}

	assert := assertion.New(t)
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Construct a zip file
			buf := new(bytes.Buffer)
			w := zip.NewWriter(buf)
			for _, file := range tc.files {
				f, err := w.Create(file.Name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = f.Write([]byte(file.Body)); err != nil {
					t.Fatal(err)
				}
			}

			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			result, err := Sha256Checksum(bytes.NewReader(buf.Bytes()))
			if tc.expectError {
				assert.Error(err)
			} else {
				assert.Equal(tc.expected, result)
			}
		})
	}
}
