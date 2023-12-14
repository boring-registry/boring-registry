package core

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	openpgpErrors "github.com/ProtonMail/go-crypto/openpgp/errors"
)

const (
	ProviderPrefix    = "terraform-provider-"
	ProviderExtension = ".zip"
)

// Provider copied from provider.Provider
// Provider represents Terraform provider metadata.
type Provider struct {
	Hostname            string      `json:"hostname,omitempty"`
	Namespace           string      `json:"namespace,omitempty"`
	Name                string      `json:"name,omitempty"`
	Version             string      `json:"version,omitempty"`
	OS                  string      `json:"os,omitempty"`
	Arch                string      `json:"arch,omitempty"`
	Filename            string      `json:"filename,omitempty"`
	DownloadURL         string      `json:"download_url,omitempty"`
	Shasum              string      `json:"shasum,omitempty"`
	SHASumsURL          string      `json:"shasums_url,omitempty"`
	SHASumsSignatureURL string      `json:"shasums_signature_url,omitempty"`
	SigningKeys         SigningKeys `json:"signing_keys,omitempty"`
	Platforms           []Platform  `json:"platforms,omitempty"`
}

func (p *Provider) ArchiveFileName() string {
	// Validate the Provider struct
	if p.Name == "" {
		panic("provider Name is empty")
	} else if p.Version == "" {
		panic("provider Version is empty")
	} else if p.OS == "" {
		panic("provider OS is empty")
	} else if p.Arch == "" {
		panic("provider Arch is empty")
	}

	return fmt.Sprintf("%s%s_%s_%s_%s%s", ProviderPrefix, p.Name, p.Version, p.OS, p.Arch, ProviderExtension)
}

func (p *Provider) ShasumFileName() string {
	if p.Name == "" {
		panic(errors.New("provider Name is empty"))
	} else if p.Version == "" {
		panic(errors.New("provider Version is empty"))
	}

	return fmt.Sprintf("%s%s_%s_SHA256SUMS", ProviderPrefix, p.Name, p.Version)
}

func (p *Provider) ShasumSignatureFileName() string {
	if p.Name == "" {
		panic("provider Name is empty")
	} else if p.Version == "" {
		panic("provider Version is empty")
	}

	return fmt.Sprintf("%s%s_%s_SHA256SUMS.sig", ProviderPrefix, p.Name, p.Version)
}

// Clone returns a deep copy of the struct
func (p *Provider) Clone() *Provider {
	r := &Provider{
		Hostname:            p.Hostname,
		Namespace:           p.Namespace,
		Name:                p.Name,
		Version:             p.Version,
		OS:                  p.OS,
		Arch:                p.Arch,
		Filename:            p.Filename,
		DownloadURL:         p.DownloadURL,
		Shasum:              p.Shasum,
		SHASumsURL:          p.SHASumsURL,
		SHASumsSignatureURL: p.SHASumsSignatureURL,
	}
	if p.Platforms != nil {
		r.Platforms = make([]Platform, len(p.Platforms))
		copy(r.Platforms, p.Platforms)
	}
	if p.SigningKeys.GPGPublicKeys != nil {
		r.SigningKeys = SigningKeys{GPGPublicKeys: make([]GPGPublicKey, len(p.SigningKeys.GPGPublicKeys))}
		copy(p.SigningKeys.GPGPublicKeys, r.SigningKeys.GPGPublicKeys)
	}
	return r
}

func NewProviderFromArchive(filename string) (Provider, error) {
	// Criterias for terraform archives:
	// https://www.terraform.io/docs/registry/providers/publishing.html#manually-preparing-a-release
	f := filepath.Base(filename) // This is just a precaution
	trimmed := strings.TrimPrefix(f, ProviderPrefix)
	trimmed = strings.TrimSuffix(trimmed, ProviderExtension)
	tokens := strings.Split(trimmed, "_")
	if len(tokens) != 4 {
		return Provider{}, fmt.Errorf("couldn't parse provider file name: %s", filename)
	}

	return Provider{
		Name:     tokens[0],
		Version:  tokens[1],
		OS:       tokens[2],
		Arch:     tokens[3],
		Filename: f,
	}, nil
}

// SigningKeys represents the signing-keys.json that we expect in the storage backend
// https://github.com/boring-registry/boring-registry#gpg-public-key-format
type SigningKeys struct {
	GPGPublicKeys []GPGPublicKey `json:"gpg_public_keys,omitempty"`
}

// IsValidSha256Sums verifies whether the GPG signature of to the SHA256SUMS file was created with a private key
// corresponding to one of the public keys in SigningKeys
func (s *SigningKeys) IsValidSha256Sums(sha256Sums, sha256SumsSig []byte) error {
	for _, key := range s.GPGPublicKeys {
		keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(key.ASCIIArmor))
		if err != nil {
			return fmt.Errorf("error reading signing key: %w", err)
		}

		_, err = openpgp.CheckDetachedSignature(keyring, bytes.NewReader(sha256Sums), bytes.NewReader(sha256SumsSig), nil)

		// If the signature issuer does not match the key, keep trying the rest of the provided keys.
		if errors.Is(err, openpgpErrors.ErrUnknownIssuer) {
			continue
		} else if err != nil {
			return err
		}

		if err == nil {
			return nil
		}
	}

	return errors.New("no valid key found for signature")
}

type GPGPublicKey struct {
	KeyID      string `json:"key_id,omitempty"`
	ASCIIArmor string `json:"ascii_armor,omitempty"`
	Source     string `json:"source,omitempty"`
	SourceURL  string `json:"source_url,omitempty"`
}

type ProviderVersions struct {
	Versions []ProviderVersion `json:"versions,omitempty"`
}

// The ProviderVersion is a copy from provider.ProviderVersion
type ProviderVersion struct {
	Namespace string     `json:"namespace,omitempty"`
	Name      string     `json:"name,omitempty"`
	Version   string     `json:"version,omitempty"`
	Protocols []string   `json:"protocols,omitempty"`
	Platforms []Platform `json:"platforms,omitempty"`
}

// Platform is a copy from provider.Platform
type Platform struct {
	OS   string `json:"os,omitempty"`
	Arch string `json:"arch,omitempty"`
}

type providerOption struct {
	Hostname  string `json:"hostname,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
}

type ProviderOption func(option *providerOption)

// parseSha256Line parses a line as found in the *_SHA256SUMS file
func parseSha256Line(line string) ([]byte, string, error) {
	r := regexp.MustCompile(`\s+`)
	s := r.Split(line, -1)
	if len(s) != 2 {
		return nil, "", fmt.Errorf("line contains %d parts instead of 2", len(s))
	}

	sum, err := hex.DecodeString(s[0])
	if err != nil {
		return nil, "", err
	}

	return sum, s[1], nil
}

type Sha256Sums struct {
	Entries  map[string][]byte
	Filename string
}

// Name returns the name of the provider of the SHA256SUMS file
func (s *Sha256Sums) Name() (string, error) {
	// RegEx could fail in rare cases as the first capture group doesn't try to match as much as possible
	r := regexp.MustCompile("^terraform-provider-(?P<name>.+)_(?P<version>.+)_SHA256SUMS$")
	matches := r.FindStringSubmatch(s.Filename)
	if len(matches) != 3 {
		return "", fmt.Errorf("regex for %s matched %d times instead of 3 times", s.Filename, len(matches))
	}
	return matches[1], nil
}

// Checksum returns the corresponding stringified checksum for the archive file name parameter
func (s *Sha256Sums) Checksum(fileName string) (string, error) {
	checksum, exists := s.Entries[fileName]
	if !exists {
		return "", fmt.Errorf("no checksum for file %s", fileName)
	}
	return fmt.Sprintf("%x", checksum), nil
}

func NewSha256Sums(filename string, r io.Reader) (*Sha256Sums, error) {
	if !isValidSha256SumsFilename(filename) {
		return nil, fmt.Errorf("SHA256SUMS file %s doesn't have valid file name", filename)
	}

	s := &Sha256Sums{
		Entries:  map[string][]byte{},
		Filename: filename,
	}
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		checksum, fileName, err := parseSha256Line(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("failed to parse entry: %w", err)
		}
		s.Entries[fileName] = checksum
	}

	return s, nil
}

// isValidSha256SumsFilename only does basic validation
func isValidSha256SumsFilename(filename string) bool {
	return regexp.MustCompile("^terraform-provider-.+_.+_SHA256SUMS$").MatchString(filename)
}

// Sha256Checksum returns the SHA256 checksum of the stream passed to the io.Reader
func Sha256Checksum(r io.Reader) ([]byte, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}
