package provider

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
)

// Storage represents the Storage of Terraform providers.
type Storage interface {
	GetProvider(ctx context.Context, namespace, name, version, os, arch string) (Provider, error)
	ListProviderVersions(ctx context.Context, namespace, name string) ([]ProviderVersion, error)
}

func storagePrefix(prefix, namespace, name string) string {
	return path.Join(
		prefix,
		fmt.Sprintf("namespace=%s", namespace),
		fmt.Sprintf("name=%s", name),
	)
}

func storagePath(prefix, namespace, name, version, os, arch string) string {
	return path.Join(
		prefix,
		fmt.Sprintf("namespace=%s", namespace),
		fmt.Sprintf("name=%s", name),
		fmt.Sprintf("version=%s", version),
		fmt.Sprintf("os=%s", os),
		fmt.Sprintf("arch=%s", arch),
		fmt.Sprintf("terraform-provider-%s_%s_%s_%s.zip", name, version, os, arch),
	)
}

func shasumsPath(prefix, namespace, name, version string) string {
	return path.Join(
		prefix,
		fmt.Sprintf("namespace=%s", namespace),
		fmt.Sprintf("name=%s", name),
		fmt.Sprintf("version=%s", version),
		fmt.Sprintf("terraform-provider-%s_%s_SHA256SUMS", name, version),
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

		if strings.HasSuffix(parts[2], name) {
			sha = parts[0]
		}
	}

	if sha == "" {
		return "", fmt.Errorf("did not find package: %s in shasums file", name)
	}

	return sha, nil
}

func signingKeysPath(prefix, namespace string) string {
	return path.Join(
		prefix,
		fmt.Sprintf("namespace=%s", namespace),
		"signing-keys.json",
	)
}
