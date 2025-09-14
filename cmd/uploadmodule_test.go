package cmd

import (
	//"context"
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	//"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

//type Mock struct{}
//
//func (m *Mock) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
//	return core.Module{}, nil
//}
//
//func (m *Mock) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
//	return nil, nil
//}
//
//func (m *Mock) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
//	return core.Module{}, nil
//}

func TestModuleUploadRunner_Run(t *testing.T) {
	validPath := t.TempDir()
	m := &moduleUploadRunner{
		//storage: &Mock{},
		discover: func(_ string, _ module.Storage) error { return nil },
	}

	tests := []struct {
		name                     string
		args                     []string
		versionConstraintsSemver string
		versionConstraintsRegex  string
		wantErr                  bool
	}{
		{
			name:    "no args returns error",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "more than a single args returns error",
			args:    []string{t.TempDir(), t.TempDir()},
			wantErr: true,
		},
		{
			name:    "non-existent path returns error",
			args:    []string{"/non/existent/path"},
			wantErr: true,
		},
		{
			name:                     "invalid semver constraint returns error",
			args:                     []string{validPath},
			versionConstraintsSemver: "invalid-semver",
			wantErr:                  true,
		},
		{
			name:                     "valid semver constraint",
			args:                     []string{validPath},
			versionConstraintsSemver: ">1.0.0",
			wantErr:                  false,
		},
		{
			name:                     "multiple valid semver constraint",
			args:                     []string{validPath},
			versionConstraintsSemver: ">1.0.0,<3.0.0",
			wantErr:                  false,
		},
		{
			name:                    "invalid regex constraint returns error",
			args:                    []string{validPath},
			versionConstraintsRegex: "[invalid-regex",
			wantErr:                 true,
		},
		{
			name:                    "valid regex constraint",
			args:                    []string{validPath},
			versionConstraintsRegex: "1\\.0\\.\\d+",
			wantErr:                 false,
		},
		{
			name:    "valid path",
			args:    []string{validPath},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global flags
			flagVersionConstraintsSemver = tt.versionConstraintsSemver
			flagVersionConstraintsRegex = tt.versionConstraintsRegex

			cmd := &cobra.Command{}
			err := m.run(cmd, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

//func generateModuleDirectory(t *testing.T, specContent string) string {
//	dir := t.TempDir()
//	os.MkdirAll(dir, 0755)
//	mf, err := os.OpenFile(filepath.Join(dir, "main.tf"), os.O_RDWR|os.O_CREATE, 0644)
//	if err != nil {
//		panic(err)
//	}
//	defer mf.Close()
//	f, err := os.OpenFile(filepath.Join(dir, moduleSpecFileName), os.O_RDWR|os.O_CREATE, 0644)
//	if err != nil {
//		panic(err)
//	}
//	defer f.Close()
//
//	_, err = f.WriteString(specContent)
//	if err != nil {
//		panic(err)
//	}
//
//	return dir
//}

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
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.result, archiveFileHeaderName(tc.path, tc.root))
		})
	}

}

func TestArchiveModule(t *testing.T) {
	t.Parallel()

	type file struct {
		content  string
		fileMode os.FileMode
	}
	tests := []struct {
		name               string
		files              map[string]file
		useNonExistentPath bool
		wantErr            bool
	}{
		{
			name: "archive module directory successfully",
			files: map[string]file{
				"main.tf":                 {content: "test content"},
				"variables.tf":            {content: "test content"},
				"modules/example/test.tf": {content: "nested content"},
			},
			wantErr: false,
		},
		{
			name: "file without read permissions",
			files: map[string]file{
				"main.tf":                 {content: "test content"},
				"variables.tf":            {content: "test content", fileMode: 0200}, // write-only
				"modules/example/test.tf": {content: "nested content"},
			},
			wantErr: true,
		},
		{
			name:               "non-existent directory",
			useNonExistentPath: true,
			wantErr:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var dir string
			if tt.useNonExistentPath {
				dir = "/non/existent/path"
			} else {
				dir = t.TempDir()
				// Create test files
				for path, f := range tt.files {
					fullPath := filepath.Join(dir, path)
					err := os.MkdirAll(filepath.Dir(fullPath), 0755)
					assert.NoError(t, err)

					mode := os.FileMode(0644)
					if f.fileMode != 0 {
						mode = f.fileMode
					}
					err = os.WriteFile(fullPath, []byte(f.content), mode)
					assert.NoError(t, err)
				}
			}

			reader, err := archiveModule(dir)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, reader)

			// Verify archive contents
			gzr, err := gzip.NewReader(reader)
			assert.NoError(t, err)

			defer func() {
				assert.NoError(t, gzr.Close())
			}()

			tr := tar.NewReader(gzr)
			foundFiles := make(map[string]bool)

			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				assert.NoError(t, err)
				foundFiles[header.Name] = true
			}

			// Verify all test files are in archive
			for fileName := range tt.files {
				assert.True(t, foundFiles[fileName], fmt.Sprintf("file %s not found in archive", fileName))
			}
		})
	}
}
