package provider

import (
	"fmt"
	"strings"
)

// deprecated: use core.Provider instead
// Provider represents Terraform provider metadata.
type Provider struct {
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

// ID returns the module metadata in a compact format.
func (p *Provider) ID(version bool) string {
	id := fmt.Sprintf("namespace=%s/name=%s", p.Namespace, p.Name)
	if version {
		id = fmt.Sprintf("%s/version=%s", id, p.Version)
	}

	return id
}

func (p *Provider) Valid() bool {
	return p.Name != "" &&
		p.Arch != "" &&
		p.Namespace != "" &&
		p.OS != "" &&
		p.Version != ""
}

type ProviderVersion struct {
	Namespace string     `json:"namespace,omitempty"`
	Name      string     `json:"name,omitempty"`
	Version   string     `json:"version,omitempty"`
	Platforms []Platform `json:"platforms,omitempty"`
}

type Platform struct {
	OS   string `json:"os,omitempty"`
	Arch string `json:"arch,omitempty"`
}

type GPGPublicKey struct {
	KeyID      string `json:"key_id,omitempty"`
	ASCIIArmor string `json:"ascii_armor,omitempty"`
	Source     string `json:"source,omitempty"`
	SourceURL  string `json:"source_url,omitempty"`
}

type SigningKeys struct {
	GPGPublicKeys []GPGPublicKey `json:"gpg_public_keys,omitempty"`
}

func Parse(v string) (Provider, error) {
	m := make(map[string]string)

	for _, part := range strings.Split(v, "/") {
		parts := strings.SplitN(part, "=", 2)
		if len(parts) != 2 {
			continue
		}

		m[parts[0]] = parts[1]
	}

	provider := Provider{
		Namespace: m["namespace"],
		Name:      m["name"],
		Version:   m["version"],
		OS:        m["os"],
		Arch:      m["arch"],
	}

	if !provider.Valid() {
		return Provider{}, fmt.Errorf("%q is not a valid path", v)
	}

	return provider, nil
}
