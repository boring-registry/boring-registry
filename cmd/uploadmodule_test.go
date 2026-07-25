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
	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
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
	m := &ModuleUploadRunner{
		config:   NewModuleUploadConfig(),
		Discover: func(_ string) error { return nil },
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
			err := m.Run(cmd, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// These tests cannot run in parallel because they modify global state
func TestNewModuleUploadConfigFromFlags(t *testing.T) {
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
			name:          "invalid module version",
			moduleVersion: "a1.2.3",
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global flags
			flagVersionConstraintsSemver = tt.versionConstraintsSemver
			flagVersionConstraintsRegex = tt.versionConstraintsRegex
			flagModuleVersion = tt.moduleVersion
			flagRecursive = tt.recursive

			config, err := NewModuleUploadConfigFromFlags()
			if (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.versionConstraintsSemver != "" && !tt.wantErr {
				assert.NotNil(t, config.VersionConstraintsSemver)
			}

			if tt.versionConstraintsRegex != "" && !tt.wantErr {
				assert.NotNil(t, config.VersionConstraintsRegex)
			}

			if tt.moduleVersion != "" && !tt.wantErr {
				assert.NotNil(t, config.ModuleVersion)
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
				// The tar format mandates forward slashes as separator on all platforms
				assert.NotContains(t, header.Name, `\`, "archive entry must not contain windows-style separators")
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
				"main.tf":                                       {content: "test content"},
				"variables.tf":                                  {content: "test content"},
				".terraform/providers/provider":                 {content: "binary content"},
				".terraform/modules/modules.json":               {content: "json content"},
				".terraform/terraform.tfstate":                  {content: "state content"},
				"modules/submodule/.terraform/providers/cached": {content: "nested binary"},
			},
			excludePatterns: []string{".terraform"},
			expectedFiles:   []string{"main.tf", "variables.tf"},
			excludedFiles:   []string{".terraform/providers/provider", ".terraform/modules/modules.json", ".terraform/terraform.tfstate", "modules/submodule/.terraform/providers/cached"},
		},
		{
			name: "exclude multiple patterns",
			files: map[string]file{
				"main.tf":                       {content: "test content"},
				"variables.tf":                  {content: "test content"},
				".terraform/providers/provider": {content: "binary"},
				"debug.log":                     {content: "log content"},
				"nested/error.log":              {content: "nested log"},
				".git/config":                   {content: "git config"},
				"modules/example/terraform.tf":  {content: "example"},
			},
			excludePatterns: []string{".terraform", "*.log", ".git"},
			expectedFiles:   []string{"main.tf", "variables.tf", "modules/example/terraform.tf"},
			excludedFiles:   []string{".terraform/providers/provider", "debug.log", "nested/error.log", ".git/config"},
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
		{
			name: "doublestar recursive pattern",
			files: map[string]file{
				"main.tf":                       {content: "test content"},
				"terraform.tfstate":             {content: "state"},
				"nested/deep/terraform.tfstate": {content: "nested state"},
				"nested/deep/other.tf":          {content: "other"},
			},
			excludePatterns: []string{"**/*.tfstate"},
			expectedFiles:   []string{"main.tf", "nested/deep/other.tf"},
			excludedFiles:   []string{"terraform.tfstate", "nested/deep/terraform.tfstate"},
		},
		{
			name: "rooted pattern matches only at root",
			files: map[string]file{
				"main.tf":               {content: "test content"},
				"vendor/lib/dep.go":     {content: "dep"},
				"modules/vendor/lib.go": {content: "nested vendor"},
			},
			excludePatterns: []string{"/vendor"},
			expectedFiles:   []string{"main.tf", "modules/vendor/lib.go"},
			excludedFiles:   []string{"vendor/lib/dep.go"},
		},
		{
			name: "trailing slash matches only directories",
			files: map[string]file{
				"main.tf":             {content: "test content"},
				"build/out.bin":       {content: "binary"},
				"build/build/out.bin": {content: "binary"},
				"build.txt":           {content: "file with build prefix"},
			},
			excludePatterns: []string{"build/"},
			expectedFiles:   []string{"main.tf", "build.txt"},
			excludedFiles:   []string{"build/out.bin"},
		},
		{
			name: "escaped trailing space matches a file name ending in a space",
			files: map[string]file{
				"main.tf":     {content: "test content"},
				"trailing ":   {content: "file name with a trailing space"},
				"trailing.tf": {content: "unrelated file"},
			},
			excludePatterns: []string{`trailing\ `},
			expectedFiles:   []string{"main.tf", "trailing.tf"},
			excludedFiles:   []string{"trailing "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			patterns, err := parseExcludePatterns(tt.excludePatterns)
			assert.NoError(t, err)

			dir := createModuleDirStructure(t, "", tt.files)
			reader, err := archiveModule(dir, patterns)
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

func TestParseExcludePatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         []string
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid patterns",
			raw:       []string{".terraform", "*.log", "/vendor"},
			wantCount: 3,
		},
		{
			name:      "empty strings are skipped",
			raw:       []string{".terraform", "", "  ", "*.log"},
			wantCount: 2,
		},
		{
			name:        "negation pattern rejected",
			raw:         []string{"!important.tf"},
			wantErr:     true,
			errContains: "negation patterns are not supported",
		},
		{
			name:      "doublestar pattern",
			raw:       []string{"**/*.tfstate"},
			wantCount: 1,
		},
		{
			name:      "nil input",
			raw:       nil,
			wantCount: 0,
		},
		{
			name:      "empty input",
			raw:       []string{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			patterns, err := parseExcludePatterns(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, patterns, tt.wantCount)
			}
		})
	}
}

// These tests cannot run in parallel because they modify global state
func TestModuleUploadRunner_ProcessModule(t *testing.T) {
	validArchive := func(string, []gitignore.Pattern) (io.Reader, error) {
		return bytes.NewReader([]byte("foo-bar")), nil
	}
	tests := []struct {
		name                     string
		specContent              string
		storage                  module.Storage
		setupArchive             func(string, []gitignore.Pattern) (io.Reader, error)
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
			setupArchive: func(string, []gitignore.Pattern) (io.Reader, error) {
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
			specPath := filepath.Join(dir, ModuleSpecFileName)
			err := os.WriteFile(specPath, []byte(tt.specContent), 0644)
			assert.NoError(t, err)

			m := &ModuleUploadRunner{
				storage: tt.storage,
				config:  NewModuleUploadConfig(WithModuleUploadConfigIgnoreExistingModule(tt.ignoreExistingModule)),
				Archive: tt.setupArchive,
			}

			if tt.versionConstraintsSemver != "" {
				constraints, err := version.NewConstraint(tt.versionConstraintsSemver)
				assert.NoError(t, err)
				m.config.VersionConstraintsSemver = constraints
			}

			if tt.versionConstraintsRegex != "" {
				constraints, err := regexp.Compile(tt.versionConstraintsRegex)
				assert.NoError(t, err)
				m.config.VersionConstraintsRegex = constraints
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
			m := &ModuleUploadRunner{
				config: NewModuleUploadConfig(WithModuleUploadConfigRecursive(tt.recursive)),
				Process: func(path string) error {
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

func TestModuleUploadConfig_Validate(t *testing.T) {
	parseVersion := func(t *testing.T, v string) *version.Version {
		moduleVersion, err := version.NewSemver(v)
		if err != nil {
			t.Fatalf("failed to parse version: %v", err)
		}

		return moduleVersion
	}

	testCases := []struct {
		name    string
		config  *ModuleUploadConfig
		isValid bool
	}{
		{
			name:    "valid configuration",
			config:  NewModuleUploadConfig(),
			isValid: true,
		},
		{
			name: "both version and recursive are invalid",
			config: NewModuleUploadConfig(
				WithModuleUploadConfigRecursive(true),
				WithModuleUploadConfigModuleVersion(parseVersion(t, "1.2.3")),
			),
			isValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.config.Validate()
			assert.Equal(t, tc.isValid, err == nil)
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
