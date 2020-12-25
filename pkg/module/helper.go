package module

import "strings"

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
