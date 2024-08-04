package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArchiveFileHeaderName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		root   string
		path   string
		result string
	}{
		{
			name:   "top-level file in a module",
			root:   "/tmp/boring-registry/modules/example",
			path:   "/tmp/boring-registry/modules/example/main.tf",
			result: "main.tf",
		},
		{
			name:   "nested file in a module",
			root:   "/tmp/boring-registry/modules/example",
			path:   "/tmp/boring-registry/modules/example/modules/auth/main.tf",
			result: "modules/auth/main.tf",
		},
		{
			name:   "hidden file without file extension",
			root:   "/tmp/boring-registry/modules/example",
			path:   "/tmp/boring-registry/modules/example/.hidden",
			result: ".hidden",
		},
		{
			name:   "hidden file without recursive walk",
			root:   ".",
			path:   ".hidden",
			result: ".hidden",
		},
		{
			name:   "file path with parent directory",
			root:   "../../tmp/boring-registry/modules/example",
			path:   "../../tmp/boring-registry/modules/example/main.tf",
			result: "main.tf",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.result, archiveFileHeaderName(tc.path, tc.root))
		})
	}

}
