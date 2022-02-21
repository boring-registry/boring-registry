package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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

func (p *Provider) ArchiveFileName() (string, error) {
	// Validate the Provider struct
	if p.Name == "" {
		return "", errors.New("provider Name is empty")
	} else if p.Version == "" {
		return "", errors.New("provider Version is empty")
	} else if p.OS == "" {
		return "", errors.New("provider OS is empty")
	} else if p.Arch == "" {
		return "", errors.New("provider Arch is empty")
	}

	return fmt.Sprintf("%s%s_%s_%s_%s%s", ProviderPrefix, p.Name, p.Version, p.OS, p.Arch, ProviderExtension), nil
}

func (p *Provider) ShasumFileName() (string, error) {
	if p.Name == "" {
		return "", errors.New("provider Name is empty")
	} else if p.Version == "" {
		return "", errors.New("provider Version is empty")
	}

	return fmt.Sprintf("%s%s_%s_SHA256SUMS", ProviderPrefix, p.Name, p.Version), nil
}

func (p *Provider) ShasumSignatureFileName() (string, error) {
	if p.Name == "" {
		return "", errors.New("provider Name is empty")
	} else if p.Version == "" {
		return "", errors.New("provider Version is empty")
	}

	return fmt.Sprintf("%s%s_%s_SHA256SUMS.sig", ProviderPrefix, p.Name, p.Version), nil
}

func NewProviderFromArchive(filename string) Provider {
	// Criterias for terraform archives:
	// https://www.terraform.io/docs/registry/providers/publishing.html#manually-preparing-a-release
	f := filepath.Base(filename) // This is just a precaution
	trimmed := strings.TrimPrefix(f, ProviderPrefix)
	trimmed = strings.TrimSuffix(trimmed, ProviderExtension)
	tokens := strings.Split(trimmed, "_")

	return Provider{
		Name:     tokens[0],
		Version:  tokens[1],
		OS:       tokens[2],
		Arch:     tokens[3],
		Filename: f,
	}
}

type SigningKeys struct {
	GPGPublicKeys []GPGPublicKey `json:"gpg_public_keys,omitempty"`
}

type GPGPublicKey struct {
	KeyID      string `json:"key_id,omitempty"`
	ASCIIArmor string `json:"ascii_armor,omitempty"`
	Source     string `json:"source,omitempty"`
	SourceURL  string `json:"source_url,omitempty"`
}

// Doesn't really belong here, but is used by multiple packages

// The ProviderVersion is a copy from provider.ProviderVersion
type ProviderVersion struct {
	Namespace string     `json:"namespace,omitempty"`
	Name      string     `json:"name,omitempty"`
	Version   string     `json:"version,omitempty"`
	Platforms []Platform `json:"platforms,omitempty"`
}

// Platform is a copy from provider.Platform
type Platform struct {
	OS   string `json:"os,omitempty"`
	Arch string `json:"arch,omitempty"`
}
