package storage

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
	"io"
	"path"
	"strings"
)

const (
	internalProviderType = providerType("providers")
	mirrorProviderType   = providerType("mirror")
)

type providerType string

// providerStoragePrefix returns a <prefix>/<internal|mirror>/<hostname>/<namespace>/<name> prefix
func providerStoragePrefix(prefix string, t providerType, hostname, namespace, name string) (string, error) {
	if t == mirrorProviderType && hostname == "" {
		return "", errors.New("hostname must not be empty for mirrored provider storage")
	}

	if namespace == "" {
		return "", errors.New("namespace is empty")
	} else if name == "" {
		return "", errors.New("name is empty")
	}

	// Overwrite hostname in case it's non-empty as we want to omit it
	if t == internalProviderType {
		hostname = ""
	}

	return path.Clean(path.Join(prefix, string(t), hostname, namespace, name)), nil
}

// internal function
func providerPath(prefix string, t providerType, hostname, namespace, name, version, os, arch string) (string, string, string, error) {
	if prefix == "" {
		return "", "", "", errors.New("prefix is empty")
	}

	p, err := providerStoragePrefix(prefix, t, hostname, namespace, name)
	if err != nil {
		return "", "", "", err
	}

	provider := core.Provider{
		Name:    name,
		Version: version,
		OS:      os,
		Arch:    arch,
	}

	archive, err := provider.ArchiveFileName()
	if err != nil {
		return "", "", "", err
	}

	shasum, err := provider.ShasumFileName()
	if err != nil {
		return "", "", "", err
	}

	shasumSig, err := provider.ShasumSignatureFileName()
	if err != nil {
		return "", "", "", err
	}

	return path.Join(p, archive), path.Join(p, shasum), path.Join(p, shasumSig), nil
}

// internalProviderPath returns a full path to an internal provider archive
func internalProviderPath(prefix, namespace, name, version, os, arch string) (string, string, string, error) {
	return providerPath(prefix, internalProviderType, "", namespace, name, version, os, arch)
}

// mirrorProviderPath returns a full path to a mirrored provider archive
func mirrorProviderPath(prefix, hostname, namespace, name, version, os, arch string) (string, string, string, error) {
	return providerPath(prefix, mirrorProviderType, hostname, namespace, name, version, os, arch)
}

func signingKeysPath(prefix string, namespace string) string {
	return path.Join(
		prefix,
		string(internalProviderType),
		namespace,
		"signing-keys.json",
	)
}

func readSHASums(r io.Reader, name string) (string, error) {
	scanner := bufio.NewScanner(r)

	sha := ""
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), " ")
		if len(parts) != 3 {
			continue
		}

		if parts[2] == name {
			sha = parts[0]
			break
		}
	}

	if sha == "" {
		return "", fmt.Errorf("did not find package: %s in shasums file", name)
	}

	return sha, nil
}
