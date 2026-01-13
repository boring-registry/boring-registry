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
		discover: func(_ string) error { return nil },
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
			reader, err := archiveModule(dir, nil)
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

func TestArchiveModuleWithExclusions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		files           map[string]file
		excludePatterns []string
		expectedFiles   []string
		excludedFiles   []string
	}{
		{
			name: "exclude .terraform directory",
			files: map[string]file{
				"main.tf":                                           {content: "test content"},
				"variables.tf":                                      {content: "test content"},
				".terraform/providers/provider.exe":                 {content: "binary content"},
				".terraform/modules/modules.json":                   {content: "json content"},
				".terraform/terraform.tfstate":                      {content: "state content"},
				"modules/submodule/.terraform/providers/cached.exe": {content: "nested binary"},
			},
			excludePatterns: []string{".terraform"},
			expectedFiles:   []string{"main.tf", "variables.tf"},
			excludedFiles:   []string{".terraform/providers/provider.exe", ".terraform/modules/modules.json", ".terraform/terraform.tfstate", "modules/submodule/.terraform/providers/cached.exe"},
		},
		{
			name: "exclude multiple patterns",
			files: map[string]file{
				"main.tf":                      {content: "test content"},
				"variables.tf":                 {content: "test content"},
				".terraform/providers/p.exe":  {content: "binary"},
				"debug.log":                   {content: "log content"},
				"nested/error.log":            {content: "nested log"},
				".git/config":                 {content: "git config"},
				"modules/example/terraform.tf": {content: "example"},
			},
			excludePatterns: []string{".terraform", "*.log", ".git"},
			expectedFiles:   []string{"main.tf", "variables.tf", "modules/example/terraform.tf"},
			excludedFiles:   []string{".terraform/providers/p.exe", "debug.log", "nested/error.log", ".git/config"},
		},
		{
			name: "exclude with glob pattern",
			files: map[string]file{
				"main.tf":       {content: "test content"},
				"test_main.go":  {content: "test file"},
				"test_utils.go": {content: "test utils"},
				"utils.go":      {content: "utils"},
			},
			excludePatterns: []string{"test_*.go"},
			expectedFiles:   []string{"main.tf", "utils.go"},
			excludedFiles:   []string{"test_main.go", "test_utils.go"},
		},
		{
			name: "no exclusions",
			files: map[string]file{
				"main.tf":      {content: "test content"},
				"variables.tf": {content: "test content"},
			},
			excludePatterns: nil,
			expectedFiles:   []string{"main.tf", "variables.tf"},
			excludedFiles:   []string{},
		},
		{
			name: "empty exclusion list",
			files: map[string]file{
				"main.tf":      {content: "test content"},
				"variables.tf": {content: "test content"},
			},
			excludePatterns: []string{},
			expectedFiles:   []string{"main.tf", "variables.tf"},
			excludedFiles:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := createModuleDirStructure(t, "", tt.files)
			reader, err := archiveModule(dir, tt.excludePatterns)
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

			// Verify expected files are in archive
			for _, expectedFile := range tt.expectedFiles {
				assert.True(t, foundFiles[expectedFile], "expected file %s not found in archive", expectedFile)
			}

			// Verify excluded files are NOT in archive
			for _, excludedFile := range tt.excludedFiles {
				assert.False(t, foundFiles[excludedFile], "excluded file %s should not be in archive", excludedFile)
			}
		})
	}
}

func TestShouldExclude(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		root     string
		isDir    bool
		patterns []string
		expected bool
	}{
		{
			name:     "exact directory name match",
			path:     "/tmp/module/.terraform",
			root:     "/tmp/module",
			isDir:    true,
			patterns: []string{".terraform"},
			expected: true,
		},
		{
			name:     "nested directory match",
			path:     "/tmp/module/submodule/.terraform",
			root:     "/tmp/module",
			isDir:    true,
			patterns: []string{".terraform"},
			expected: true,
		},
		{
			name:     "file inside excluded directory",
			path:     "/tmp/module/.terraform/providers/file.exe",
			root:     "/tmp/module",
			isDir:    false,
			patterns: []string{".terraform"},
			expected: true,
		},
		{
			name:     "glob pattern match",
			path:     "/tmp/module/debug.log",
			root:     "/tmp/module",
			isDir:    false,
			patterns: []string{"*.log"},
			expected: true,
		},
		{
			name:     "no match",
			path:     "/tmp/module/main.tf",
			root:     "/tmp/module",
			isDir:    false,
			patterns: []string{".terraform", "*.log"},
			expected: false,
		},
		{
			name:     "empty patterns",
			path:     "/tmp/module/.terraform",
			root:     "/tmp/module",
			isDir:    true,
			patterns: []string{},
			expected: false,
		},
		{
			name:     "nil patterns",
			path:     "/tmp/module/.terraform",
			root:     "/tmp/module",
			isDir:    true,
			patterns: nil,
			expected: false,
		},
		{
			name:     "path component match in nested path",
			path:     "/tmp/module/modules/auth/.terraform/cache/file.json",
			root:     "/tmp/module",
			isDir:    false,
			patterns: []string{".terraform"},
			expected: true,
		},
		{
			name:     "partial name should not match",
			path:     "/tmp/module/terraform.tf",
			root:     "/tmp/module",
			isDir:    false,
			patterns: []string{".terraform"},
			expected: false,
		},
		{
			name:     "multiple patterns first matches",
			path:     "/tmp/module/.git/config",
			root:     "/tmp/module",
			isDir:    false,
			patterns: []string{".git", ".terraform", "*.log"},
			expected: true,
		},
		{
			name:     "multiple patterns last matches",
			path:     "/tmp/module/app.log",
			root:     "/tmp/module",
			isDir:    false,
			patterns: []string{".git", ".terraform", "*.log"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := shouldExclude(tt.path, tt.root, tt.isDir, tt.patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestArchiveModuleIntegration demonstrates the real-world impact of --exclude flag
// by creating a module with large .terraform directory and comparing archive sizes
func TestArchiveModuleIntegration(t *testing.T) {
	t.Parallel()

	// Create a realistic module structure with .terraform directory
	dir := t.TempDir()
	files := map[string]struct {
		content string
		size    int // if > 0, creates a file of this size with random data
	}{
		"main.tf":                                      {content: `resource "aws_instance" "example" {}`},
		"variables.tf":                                 {content: `variable "region" { type = string }`},
		"outputs.tf":                                   {content: `output "id" { value = aws_instance.example.id }`},
		".terraform/providers/registry/aws/provider":  {size: 1024 * 100}, // 100KB fake provider binary
		".terraform/modules/modules.json":             {content: `{"Modules": []}`},
		".terraform/terraform.tfstate":                {content: `{"version": 4}`},
		"modules/auth/.terraform/plugins/cached.json": {content: `{"cached": true}`},
	}

	for path, f := range files {
		fullPath := filepath.Join(dir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		assert.NoError(t, err)

		var content []byte
		if f.size > 0 {
			content = make([]byte, f.size)
			for i := range content {
				content[i] = byte(i % 256)
			}
		} else {
			content = []byte(f.content)
		}
		err = os.WriteFile(fullPath, content, 0644)
		assert.NoError(t, err)
	}

	// Archive WITHOUT exclusions
	readerWithoutExclude, err := archiveModule(dir, nil)
	assert.NoError(t, err)

	bufWithout := new(bytes.Buffer)
	_, err = io.Copy(bufWithout, readerWithoutExclude)
	assert.NoError(t, err)
	sizeWithout := bufWithout.Len()

	// Archive WITH .terraform exclusion
	readerWithExclude, err := archiveModule(dir, []string{".terraform"})
	assert.NoError(t, err)

	bufWith := new(bytes.Buffer)
	_, err = io.Copy(bufWith, readerWithExclude)
	assert.NoError(t, err)
	sizeWith := bufWith.Len()

	// Verify the archive with exclusions is significantly smaller
	t.Logf("Archive size WITHOUT exclusions: %d bytes", sizeWithout)
	t.Logf("Archive size WITH .terraform exclusion: %d bytes", sizeWith)
	t.Logf("Size reduction: %.1f%%", float64(sizeWithout-sizeWith)/float64(sizeWithout)*100)

	assert.Greater(t, sizeWithout, sizeWith, "archive with exclusions should be smaller")

	// Verify the excluded archive only contains the expected files
	gzr, err := gzip.NewReader(bufWith)
	assert.NoError(t, err)
	defer gzr.Close()

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

	// These files should be present
	assert.True(t, foundFiles["main.tf"], "main.tf should be in archive")
	assert.True(t, foundFiles["variables.tf"], "variables.tf should be in archive")
	assert.True(t, foundFiles["outputs.tf"], "outputs.tf should be in archive")

	// These files should NOT be present (excluded)
	assert.False(t, foundFiles[".terraform/providers/registry/aws/provider"], ".terraform content should be excluded")
	assert.False(t, foundFiles[".terraform/modules/modules.json"], ".terraform content should be excluded")
	assert.False(t, foundFiles["modules/auth/.terraform/plugins/cached.json"], "nested .terraform should be excluded")
}

// These tests cannot run in parallel because they modify global state
func TestModuleUploadRunner_ProcessModule(t *testing.T) {
	validArchive := func(string, []string) (io.Reader, error) {
		return bytes.NewReader([]byte("foo-bar")), nil
	}
	tests := []struct {
		name                     string
		specContent              string
		storage                  module.Storage
		setupArchive             func(string, []string) (io.Reader, error)
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
			setupArchive: func(string, []string) (io.Reader, error) {
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
			// Clean up global state
			versionConstraintsSemver = nil
			versionConstraintsRegex = nil
			flagIgnoreExistingModule = tt.ignoreExistingModule

			dir := t.TempDir()
			specPath := filepath.Join(dir, moduleSpecFileName)
			err := os.WriteFile(specPath, []byte(tt.specContent), 0644)
			assert.NoError(t, err)

			m := &moduleUploadRunner{
				storage: tt.storage,
				archive: tt.setupArchive,
			}

			if tt.versionConstraintsSemver != "" {
				constraints, err := version.NewConstraint(tt.versionConstraintsSemver)
				assert.NoError(t, err)
				versionConstraintsSemver = constraints
			}

			if tt.versionConstraintsRegex != "" {
				constraints, err := regexp.Compile(tt.versionConstraintsRegex)
				assert.NoError(t, err)
				versionConstraintsRegex = constraints
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

// These tests cannot run in parallel because they modify global state
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
			dir := createModuleDirStructure(t, tt.root, tt.files)

			var processedPaths []string
			m := &moduleUploadRunner{
				process: func(path string) error {
					processedPaths = append(processedPaths, path)
					return tt.processErr
				},
			}

			flagRecursive = tt.recursive

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
