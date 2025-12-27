package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/module"
	"github.com/hashicorp/go-version"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

type file struct {
	content  string
	fileMode os.FileMode
}

type mockModuleStorage struct {
	getModuleErr error
	uploadErr    error
}

func (m *mockModuleStorage) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	return core.Module{}, m.getModuleErr
}

func (m *mockModuleStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	return nil, nil
}

func (m *mockModuleStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
	return core.Module{}, m.uploadErr
}

func TestModuleUploadRunner_Run(t *testing.T) {
	validPath := t.TempDir()
	m := &moduleUploadRunner{
		config:   &moduleUploadConfig{},
		discover: func(_ string) error { return nil },
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
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
			name:    "valid path",
			args:    []string{validPath},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			err := m.run(cmd, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// These tests cannot run in parallel because they modify global state
func TestModuleUploadRunner_ParseFlags(t *testing.T) {
	tests := []struct {
		name                     string
		versionConstraintsSemver string
		versionConstraintsRegex  string
		moduleVersion            string
		recursive                bool
		wantErr                  bool
	}{
		{
			name:                     "invalid semver constraint returns error",
			versionConstraintsSemver: "invalid-semver",
			wantErr:                  true,
		},
		{
			name:                     "valid semver constraint",
			versionConstraintsSemver: ">1.0.0",
			wantErr:                  false,
		},
		{
			name:                     "multiple valid semver constraint",
			versionConstraintsSemver: ">1.0.0,<3.0.0",
			wantErr:                  false,
		},
		{
			name:                    "invalid regex constraint returns error",
			versionConstraintsRegex: "[invalid-regex",
			wantErr:                 true,
		},
		{
			name:                    "valid regex constraint",
			versionConstraintsRegex: "1\\.0\\.\\d+",
			wantErr:                 false,
		},
		{
			name:          "no module version with recursive discovery disabled",
			moduleVersion: "",
			recursive:     false,
			wantErr:       false,
		},
		{
			name:          "no module version with recursive discovery enabled",
			moduleVersion: "",
			recursive:     true,
			wantErr:       false,
		},
		{
			name:          "module version with recursive discovery enabled",
			moduleVersion: "1.2.3",
			recursive:     true,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &moduleUploadRunner{
				config: &moduleUploadConfig{},
			}

			// Reset global flags
			flagVersionConstraintsSemver = tt.versionConstraintsSemver
			flagVersionConstraintsRegex = tt.versionConstraintsRegex
			flagModuleVersion = tt.moduleVersion
			flagRecursive = tt.recursive

			err := m.parseFlags()
			if (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.versionConstraintsSemver != "" && !tt.wantErr {
				assert.NotNil(t, m.config.versionConstraintsSemver)
			}

			if tt.versionConstraintsRegex != "" && !tt.wantErr {
				assert.NotNil(t, m.config.versionConstraintsRegex)
			}

			if tt.moduleVersion != "" && !tt.wantErr {
				assert.NotNil(t, m.config.moduleVersion)
			}
		})
	}

	// Set global flags to default value after tests.
	// This is not pretty and could be done better
	flagVersionConstraintsSemver = ""
	flagVersionConstraintsRegex = ""
	flagModuleVersion = ""
	flagRecursive = true
}

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
	tests := []struct {
		name    string
		files   map[string]file
		root    string
		wantErr bool
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
			name:    "non-existent directory",
			root:    "/non/existent/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := createModuleDirStructure(t, tt.root, tt.files)
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

func TestModuleUploadRunner_ProcessModule(t *testing.T) {
	validArchive := func(string) (io.Reader, error) {
		return bytes.NewReader([]byte("foo-bar")), nil
	}
	tests := []struct {
		name                     string
		specContent              string
		storage                  module.Storage
		setupArchive             func(string) (io.Reader, error)
		ignoreExistingModule     bool
		versionConstraintsSemver string
		versionConstraintsRegex  string

		wantErr bool
	}{
		{
			name:        "invalid spec file",
			specContent: "invalid content",
			wantErr:     true,
		},
		{
			name: "unexpected failure on GetModule",
			specContent: `
				metadata {
					namespace = "test"
					name = "example" 
					provider = "aws"
					version = "1.0.0"
				}`,
			storage: &mockModuleStorage{
				getModuleErr: fmt.Errorf("unexpected error"),
			},
			wantErr: true,
		},
		{
			name: "existing module with ignore flag",
			specContent: `
				metadata {
					namespace = "test"
					name = "example" 
					provider = "aws"
					version = "1.0.0"
				}`,
			storage:              &mockModuleStorage{},
			setupArchive:         validArchive,
			ignoreExistingModule: true,
			wantErr:              false,
		},
		{
			name: "existing module without ignore flag",
			specContent: `
				metadata {
					namespace = "test"
					name = "example"
					provider = "aws"
					version = "1.0.0"
				}`,
			storage:              &mockModuleStorage{},
			setupArchive:         validArchive,
			ignoreExistingModule: false,
			wantErr:              true,
		},
		{
			name: "version does not meet semver constraints",
			specContent: `
				metadata {
					namespace = "test"
					name = "example"
					provider = "aws"
					version = "1.0.0"
				}`,
			versionConstraintsSemver: ">2.0.0",
			storage:                  &mockModuleStorage{},
			wantErr:                  false,
		},
		{
			name: "version does not meet regex constraints",
			specContent: `
				metadata {
					namespace = "test"
					name = "example"
					provider = "aws"
					version = "1.0.0"
				}`,
			versionConstraintsRegex: "$2\\.0\\.\\d+",
			storage:                 &mockModuleStorage{},
			wantErr:                 false,
		},
		{
			name: "creating archive fails",
			specContent: `
				metadata {
					namespace = "test"
					name = "example"
					provider = "aws"
					version = "1.0.0"
				}`,
			storage: &mockModuleStorage{
				getModuleErr: module.ErrModuleNotFound,
			},
			setupArchive: func(string) (io.Reader, error) {
				return nil, fmt.Errorf("failed to create archive")
			},
			wantErr: true,
		},
		{
			name: "upload fails",
			specContent: `
				metadata {
					namespace = "test"
					name = "example"
					provider = "aws"
					version = "1.0.0"
				}`,
			storage: &mockModuleStorage{
				getModuleErr: module.ErrModuleNotFound,
				uploadErr:    fmt.Errorf("upload failed"),
			},
			setupArchive: validArchive,
			wantErr:      true,
		},
		{
			name: "successful upload",
			specContent: `
				metadata {
					namespace = "test"
					name = "example"
					provider = "aws"
					version = "1.0.0"
				}`,
			storage: &mockModuleStorage{
				getModuleErr: module.ErrModuleNotFound,
			},
			setupArchive: validArchive,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			specPath := filepath.Join(dir, moduleSpecFileName)
			err := os.WriteFile(specPath, []byte(tt.specContent), 0644)
			assert.NoError(t, err)

			m := &moduleUploadRunner{
				storage: tt.storage,
				config: &moduleUploadConfig{
					ignoreExistingModule: tt.ignoreExistingModule,
				},
				archive: tt.setupArchive,
			}

			if tt.versionConstraintsSemver != "" {
				constraints, err := version.NewConstraint(tt.versionConstraintsSemver)
				assert.NoError(t, err)
				m.config.versionConstraintsSemver = constraints
			}

			if tt.versionConstraintsRegex != "" {
				constraints, err := regexp.Compile(tt.versionConstraintsRegex)
				assert.NoError(t, err)
				m.config.versionConstraintsRegex = constraints
			}

			err = m.processModule(specPath)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestModuleUploadRunner_WalkModules(t *testing.T) {
	tests := []struct {
		name          string
		recursive     bool
		root          string // if empty, a temporary dir will be created
		files         map[string]file
		processErr    error
		expectedPaths int
		wantErr       bool
	}{
		{
			name:      "recursive with non-existent root path",
			recursive: true,
			root:      "/non/existent/path",
			wantErr:   true,
		},
		{
			name:      "single module non-recursive",
			recursive: false,
			files: map[string]file{
				"boring-registry.hcl": {content: "content"},
				"main.tf":             {content: "content"},
			},
			expectedPaths: 1,
			wantErr:       false,
		},
		{
			name:      "recursive with multiple modules",
			recursive: true,
			files: map[string]file{
				"modules/foo/boring-registry.hcl":         {content: "content"},
				"modules/foo/main.tf":                     {content: "content"},
				"modules/bar/boring-registry.hcl":         {content: "content"},
				"modules/bar/main.tf":                     {content: "content"},
				"modules/ignored/not-boring-registry.hcl": {content: "ignored"},
			},
			expectedPaths: 2,
			wantErr:       false,
		},
		{
			name:      "processing error",
			recursive: false,
			files: map[string]file{
				"boring-registry.hcl": {content: "content"},
			},
			processErr:    fmt.Errorf("process failed"),
			expectedPaths: 1,
			wantErr:       true,
		},
		{
			name:          "no module file non-recursive",
			recursive:     false,
			files:         map[string]file{},
			expectedPaths: 1,
			wantErr:       false,
		},
		{
			name:          "no module file recursive",
			recursive:     true,
			files:         map[string]file{},
			expectedPaths: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := createModuleDirStructure(t, tt.root, tt.files)

			var processedPaths []string
			m := &moduleUploadRunner{
				config: &moduleUploadConfig{
					recursive: tt.recursive,
				},
				process: func(path string) error {
					processedPaths = append(processedPaths, path)
					return tt.processErr
				},
			}

			err := m.walkModules(dir)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPaths, len(processedPaths))
			}
		})
	}
}

func createModuleDirStructure(t *testing.T, root string, files map[string]file) string {
	dir := root
	if root == "" {
		dir = t.TempDir()
	}

	for path, f := range files {
		fullPath := filepath.Join(dir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		assert.NoError(t, err)

		mode := os.FileMode(0644) //default
		if f.fileMode != 0 {
			mode = f.fileMode
		}
		err = os.WriteFile(fullPath, []byte(f.content), mode)
		assert.NoError(t, err)
	}

	return dir
}
