package storage

import (
	"bufio"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
	"io"
	"strings"
)

func ReadSHASums(r io.Reader, provider core.Provider) (string, error) {
	scanner := bufio.NewScanner(r)

	sha := ""
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), " ")
		if len(parts) != 3 {
			continue
		}

		if parts[2] == provider.ArchiveFileName() {
			sha = parts[0]
		}
	}

	if sha == "" {
		return "", fmt.Errorf("did not find package: %s in shasums file", provider.ArchiveFileName())
	}

	return sha, nil
}
