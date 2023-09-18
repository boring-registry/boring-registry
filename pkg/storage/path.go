package storage

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/TierMobility/boring-registry/pkg/core"
)

const (
	internalProviderType = providerType("providers")
	mirrorProviderType   = providerType("mirror")
	internalModuleType   = moduleType("modules")
)

type providerType string
type moduleType string

// providerStoragePrefix returns a prefix in the form of either <prefix>/providers/<namespace>/<name>
// or <prefix>/mirror/<hostname>/<namespace>/<name> prefix
func providerStoragePrefix(prefix string, t providerType, hostname, namespace, name string) string {
	if t == mirrorProviderType && hostname == "" {
		panic("hostname must not be empty for mirrored provider storage")
	}

	if namespace == "" {
		panic("namespace is empty")
	} else if name == "" {
		panic("name is empty")
	}

	// Overwrite hostname in case it's non-empty as we want to omit it
	if t == internalProviderType {
		hostname = ""
	}

	return path.Clean(path.Join(prefix, string(t), hostname, namespace, name))
}

// internal function
func providerPath(prefix string, t providerType, hostname, namespace, name, version, os, arch string) (string, string, string) {
	provider := core.Provider{
		Name:    name,
		Version: version,
		OS:      os,
		Arch:    arch,
	}

	p := providerStoragePrefix(prefix, t, hostname, namespace, name)
	return path.Join(p, provider.ArchiveFileName()), path.Join(p, provider.ShasumFileName()), path.Join(p, provider.ShasumSignatureFileName())
}

// internalProviderPath returns a full path to an internal provider archive
func internalProviderPath(prefix, namespace, name, version, os, arch string) (string, string, string) {
	return providerPath(prefix, internalProviderType, "", namespace, name, version, os, arch)
}

// TODO(oliviermichaelis): consider using /mirror/providers/<hostname>
// mirrorProviderPath returns a full path to a mirrored provider archive
func mirrorProviderPath(prefix, hostname, namespace, name, version, os, arch string) (string, string, string) {
	return providerPath(prefix, mirrorProviderType, hostname, namespace, name, version, os, arch)
}

// modulePathPrefix returns a <prefix>/modules/<namespace>/<name>/<provider> prefix
func modulePathPrefix(prefix, namespace, name, provider string) string {
	return path.Join(prefix, string(internalModuleType), namespace, name, provider)
}

func modulePath(prefix, namespace, name, provider, version, archiveFormat string) string {
	f := fmt.Sprintf("%s-%s-%s-%s.%s", namespace, name, provider, version, archiveFormat)
	return path.Join(modulePathPrefix(prefix, namespace, name, provider), f)
}

func signingKeysPath(prefix string, pt providerType, hostname, namespace string) string {
	return path.Join(
		prefix,
		string(pt),
		hostname,
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

func moduleFromObject(key string, fileExtension string) (*core.Module, error) {
	dir, file := path.Split(key)

	dirParts := strings.Split(dir, "/")
	for _, part := range dirParts {
		dirParts = dirParts[1:] // Remove the first item
		if part == string(internalModuleType) {
			break
		}
	}
	if len(dirParts) < 3 {
		return nil, fmt.Errorf("module key is invalid: expected 3 directory parts, but was %d", len(dirParts))
	}

	fileExtension = fmt.Sprintf(".%s", fileExtension) // Add the dot to the file extension
	if !strings.HasSuffix(file, fileExtension) {
		return nil, fmt.Errorf("expected file extension \"%s\" but found \"%s\"", fileExtension, path.Ext(file))
	}
	file = strings.TrimSuffix(file, fileExtension) // Remove the file extension

	filePrefix := fmt.Sprintf("%s-%s-%s-", dirParts[0], dirParts[1], dirParts[2])
	if !strings.HasPrefix(file, filePrefix) {
		return nil, fmt.Errorf("expected file prefix \"%s\" but file is \"%s\"", filePrefix, file)
	}
	version := strings.TrimPrefix(file, filePrefix) // Remove everything up to the version
	if version == "" {
		return nil, fmt.Errorf("module key is invalid, could not parse version")
	}

	return &core.Module{
		Namespace: dirParts[0],
		Name:      dirParts[1],
		Provider:  dirParts[2],
		Version:   version,
	}, nil
}

// Only necessary for the migration of modules
func objectMetadata(key string) map[string]string {
	m := make(map[string]string)

	for _, part := range strings.Split(key, "/") {
		parts := strings.SplitN(part, "=", 2)
		if len(parts) != 2 {
			continue
		}

		m[parts[0]] = parts[1]
	}

	return m
}

// Only necessary for the migration of modules
func migrationTargetPath(bucketPrefix, archiveFormat, sourceKey string) string {
	prefix := path.Join(bucketPrefix, string(internalModuleType))
	directory := path.Dir(sourceKey)
	oldKey := path.Clean(strings.TrimPrefix(directory, fmt.Sprintf("%s/", prefix)))
	m := objectMetadata(oldKey)

	return modulePath(bucketPrefix, m["namespace"], m["name"], m["provider"], m["version"], archiveFormat)
}

// Only necessary for the migration of providers
func providerMigrationTargetPath(bucketPrefix, sourceKey string) (string, error) {
	prefix := path.Join(bucketPrefix, string(internalProviderType))
	oldKey := path.Clean(strings.TrimPrefix(sourceKey, fmt.Sprintf("%s/", prefix)))
	directories := path.Dir(oldKey)

	if !strings.HasPrefix(sourceKey, path.Join(prefix, "namespace=")) {
		return "", fmt.Errorf("file doesn't have prefix %s", path.Join(prefix, "namespace="))
	}

	m := objectMetadata(directories)

	if strings.HasSuffix(sourceKey, "signing-keys.json") {
		return path.Clean(path.Join(prefix, m["namespace"])), nil
	}
	return providerStoragePrefix(bucketPrefix, internalProviderType, "", m["namespace"], m["name"]), nil
}

func isUnmigratedModule(bucketPrefix, key string) bool {
	return strings.HasPrefix(key, path.Join(bucketPrefix, string(internalModuleType), "namespace="))
}
