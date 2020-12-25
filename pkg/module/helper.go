package module

import (
	"fmt"
	"strings"
)

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

func moduleObjectKeyBase(namespace, name, provider, prefix string) string {
	return fmt.Sprintf("%[1]vnamespace=%[2]v/name=%[3]v/provider=%[4]v", prefix, namespace, name, provider)
}

func moduleObjectKey(namespace, name, provider, version, prefix string) string {
	base := moduleObjectKeyBase(namespace, name, provider, prefix)
	return fmt.Sprintf("%[5]v/version=%[4]v/%[1]v-%[2]v-%[3]v-%[4]v.tar.gz", namespace, name, provider, version, base)
}
