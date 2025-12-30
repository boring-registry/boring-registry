//go:build integration

package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/boring-registry/boring-registry/cmd"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

func renderModuleSpecWithoutVersion(name string) string {
	spec := `
		metadata {
		  name      = "%s"
		  namespace = "example"
		  provider  = "acme"
		}
	`
	return fmt.Sprintf(spec, name)
}
func renderModuleSpecWithVersion(name, version string) string {
	spec := `
		metadata {
		  name      = "%s"
		  namespace = "example"
		  provider  = "acme"
		  version   = "%s"
		}
	`
	return fmt.Sprintf(spec, name, version)
}

func createMainFile() file {
	return file{
		name: "main.tf",
		content: `
			resource "aws_subnet" "az" {
			  count = length(var.availability_zones)
			  availability_zone = var.availability_zones[count.index]
			  vpc_id = aws_vpc.main.id
			  cidr_block = cidrsubnet(aws_vpc.main.cidr_block, 4, count.index+1)
			}
		`,
	}
}

func parseVersion(t *testing.T, v string) *version.Version {
	moduleVersion, err := version.NewSemver(v)
	if err != nil {
		t.Fatalf("failed to parse version: %v", err)
	}

	return moduleVersion
}

func parseSemver(t *testing.T, s string) version.Constraints {
	constraints, err := version.NewConstraint(s)
	if err != nil {
		t.Fatalf("failed to parse version-constraints-semver: %v", err)
	}
	return constraints
}

func parseRegex(t *testing.T, s string) *regexp.Regexp {
	constraints, err := regexp.Compile(s)
	if err != nil {
		t.Fatalf("failed to parse version-constraints-regex: %v", err)
	}
	return constraints
}

func TestUploadModule(t *testing.T) {
	backgroundCtx := context.Background()
	// This defines the maximum duration for completing the test.
	// Includes the time it might take to download the container image and start/stop everything.
	rootCtx, rootCancel := context.WithTimeout(backgroundCtx, 30*time.Minute)
	defer rootCancel()

	runner := &storageHarness{}
	terminateContainer := runner.createAzuriteContainer(rootCtx, t)
	defer terminateContainer()
	runner.setupClient(rootCtx, t)

	testCases := []struct {
		name          string
		fs            *fsStructure
		path          string // The subpath (without the temp dir prefix) which is passed to the upload command
		version       string
		config        *cmd.ModuleUploadConfig
		expected      []string
		expectedError bool
	}{
		{
			name: "single simple module in root dir",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "private-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithoutVersion("private-tls-key"),
						},
						createMainFile(),
					},
				},
			}),
			path: "private-tls-key",
			config: cmd.NewModuleUploadConfig(
				cmd.WithModuleUploadConfigModuleVersion(parseVersion(t, "1.2.3")),
				cmd.WithModuleUploadConfigRecursive(false),
			),
			expected:      []string{"modules/example/private-tls-key/acme/example-private-tls-key-acme-1.2.3.tar.gz"},
			expectedError: false,
		},
		{
			name: "two simple modules in root dir without recursive",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "private-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithoutVersion("private-tls-key"),
						},
						createMainFile(),
					},
				},
				dir{
					name: "public-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithoutVersion("public-tls-key"),
						},
						createMainFile(),
					},
				},
			}),
			path: "private-tls-key",
			config: cmd.NewModuleUploadConfig(
				cmd.WithModuleUploadConfigModuleVersion(parseVersion(t, "1.2.3")),
				cmd.WithModuleUploadConfigRecursive(false),
			),
			expected:      []string{"modules/example/private-tls-key/acme/example-private-tls-key-acme-1.2.3.tar.gz"},
			expectedError: false,
		},
		{
			name: "version with recursive is not allowed",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "private-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithoutVersion("private-tls-key"),
						},
						createMainFile(),
					},
				},
			}),
			path: "private-tls-key",
			config: cmd.NewModuleUploadConfig(
				cmd.WithModuleUploadConfigModuleVersion(parseVersion(t, "1.2.3")),
				cmd.WithModuleUploadConfigRecursive(true),
			),
			expected:      []string{},
			expectedError: true,
		},
		{
			name: "discover module in subdirectory",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "modules",
					subdir: []dir{
						dir{
							name: "private-tls-key",
							files: []file{
								{
									name:    cmd.ModuleSpecFileName,
									content: renderModuleSpecWithVersion("private-tls-key", "1.0.0"),
								},
								createMainFile(),
							},
						},
					},
				},
			}),
			path:   "modules",
			config: cmd.NewModuleUploadConfig(),
			expected: []string{
				"modules/example/private-tls-key/acme/example-private-tls-key-acme-1.0.0.tar.gz",
			},
			expectedError: false,
		},
		{
			name: "discover multiple modules",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "modules",
					subdir: []dir{
						dir{
							name: "private-tls-key",
							files: []file{
								{
									name:    cmd.ModuleSpecFileName,
									content: renderModuleSpecWithVersion("private-tls-key", "1.0.0"),
								},
								createMainFile(),
							},
						},
						dir{
							name: "public-tls-key",
							files: []file{
								{
									name:    cmd.ModuleSpecFileName,
									content: renderModuleSpecWithVersion("public-tls-key", "1.0.0"),
								},
								createMainFile(),
							},
						},
					},
				},
			}),
			path:   "modules",
			config: cmd.NewModuleUploadConfig(),
			expected: []string{
				"modules/example/private-tls-key/acme/example-private-tls-key-acme-1.0.0.tar.gz",
				"modules/example/public-tls-key/acme/example-public-tls-key-acme-1.0.0.tar.gz",
			},
			expectedError: false,
		},
		{
			name: "module without module spec file is not uploaded",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "modules",
					subdir: []dir{
						dir{
							name: "private-tls-key",
							files: []file{
								{
									name:    cmd.ModuleSpecFileName,
									content: renderModuleSpecWithVersion("private-tls-key", "1.0.0"),
								},
								createMainFile(),
							},
						},
						dir{
							name: "public-tls-key",
							files: []file{
								createMainFile(),
							},
						},
					},
				},
			}),
			path:   "modules",
			config: cmd.NewModuleUploadConfig(),
			expected: []string{
				"modules/example/private-tls-key/acme/example-private-tls-key-acme-1.0.0.tar.gz",
			},
			expectedError: false,
		},
		{
			name: "module version flag does not match semver constraint",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "private-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithoutVersion("private-tls-key"),
						},
						createMainFile(),
					},
				},
			}),
			path: "private-tls-key",
			config: cmd.NewModuleUploadConfig(
				cmd.WithModuleUploadConfigModuleVersion(parseVersion(t, "1.2.3")),
				cmd.WithModuleUploadConfigRecursive(false),
				cmd.WithModuleUploadConfigVersionConstraintsSemver(parseSemver(t, ">2.0")),
			),
			expected:      []string{},
			expectedError: false,
		},
		{
			name: "module version flag matches semver constraint",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "private-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithoutVersion("private-tls-key"),
						},
						createMainFile(),
					},
				},
			}),
			path: "private-tls-key",
			config: cmd.NewModuleUploadConfig(
				cmd.WithModuleUploadConfigModuleVersion(parseVersion(t, "1.2.3")),
				cmd.WithModuleUploadConfigRecursive(false),
				cmd.WithModuleUploadConfigVersionConstraintsSemver(parseSemver(t, ">=1.0")),
			),
			expected: []string{
				"modules/example/private-tls-key/acme/example-private-tls-key-acme-1.2.3.tar.gz",
			},
			expectedError: false,
		},
		{
			name: "module version flag does not match regex constraint",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "private-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithoutVersion("private-tls-key"),
						},
						createMainFile(),
					},
				},
			}),
			path: "private-tls-key",
			config: cmd.NewModuleUploadConfig(
				cmd.WithModuleUploadConfigModuleVersion(parseVersion(t, "1.2.3")),
				cmd.WithModuleUploadConfigRecursive(false),
				cmd.WithModuleUploadConfigVersionConstraintsRegex(parseRegex(t, "abcd")),
			),
			expected:      []string{},
			expectedError: false,
		},
		{
			name: "module version flag matches regex constraint",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "private-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithoutVersion("private-tls-key"),
						},
						createMainFile(),
					},
				},
			}),
			path: "private-tls-key",
			config: cmd.NewModuleUploadConfig(
				cmd.WithModuleUploadConfigModuleVersion(parseVersion(t, "1.2.3")),
				cmd.WithModuleUploadConfigRecursive(false),
				cmd.WithModuleUploadConfigVersionConstraintsRegex(parseRegex(t, "1\\.2\\.3")),
			),
			expected: []string{
				"modules/example/private-tls-key/acme/example-private-tls-key-acme-1.2.3.tar.gz",
			},
			expectedError: false,
		},
		{
			name: "uploading the same module twice should fail with ignore-existing-module disabled",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "private-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithVersion("private-tls-key", "1.0.0"),
						},
						createMainFile(),
					},
				},
				dir{
					name: "private-tls-key-copy",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithVersion("private-tls-key", "1.0.0"),
						},
						createMainFile(),
					},
				},
			}),
			path: ".",
			config: cmd.NewModuleUploadConfig(
				cmd.WithModuleUploadConfigIgnoreExistingModule(false),
			),
			expected: []string{
				"modules/example/private-tls-key/acme/example-private-tls-key-acme-1.0.0.tar.gz",
			},
			expectedError: true,
		},
		{
			name: "uploading the same module twice should not fail with ignore-existing-module enabled",
			fs: newFsStructure(t).setDirs([]dir{
				dir{
					name: "private-tls-key",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithVersion("private-tls-key", "1.0.0"),
						},
						createMainFile(),
					},
				},
				dir{
					name: "private-tls-key-copy",
					files: []file{
						{
							name:    cmd.ModuleSpecFileName,
							content: renderModuleSpecWithVersion("private-tls-key", "1.0.0"),
						},
						createMainFile(),
					},
				},
			}),
			path: ".",
			config: cmd.NewModuleUploadConfig(
				cmd.WithModuleUploadConfigIgnoreExistingModule(true),
			),
			expected: []string{
				"modules/example/private-tls-key/acme/example-private-tls-key-acme-1.0.0.tar.gz",
			},
			expectedError: false,
		},
	}

	// The outer group is necessary to prevent the parent from tearing down the test resources when
	// running parallel tests. See the following blog post:
	// https://go.dev/blog/subtests
	t.Run("group", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				instance := runner.newStorageInstance(rootCtx, t)

				if err := tc.fs.create(); err != nil {
					t.Fatalf("failed to create test filesystem: %v", err)
				}

				moduleUploadRunner := cmd.NewModuleUploadRunnerWithDefaultConfig()
				moduleUploadRunner.Config = tc.config
				moduleUploadRunner.Storage = instance.setupStorage()
				moduleUploadRunner.InitializeMethods()

				p := filepath.Join(tc.fs.rootPath, tc.path)
				err := moduleUploadRunner.Run(nil, []string{p})
				if err != nil {
					assert.Equal(t, tc.expectedError, err != nil, "error", err.Error())
				} else {
					assert.Equal(t, tc.expectedError, err != nil)
				}

				objects := instance.uploadedObjects(rootCtx, t)
				assert.Equal(t, tc.expected, objects)
			})
		}
	})
}
