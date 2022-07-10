package module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"github.com/TierMobility/boring-registry/pkg/core"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testModuleData(files map[string]string) *bytes.Buffer {
	buf := new(bytes.Buffer)

	gw := gzip.NewWriter(buf)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, moduleData := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(moduleData)),
		}

		_ = tw.WriteHeader(hdr)
		_, _ = tw.Write([]byte(moduleData))
	}

	return buf
}

func TestService_GetModule(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name        string
		module      core.Module
		data        io.Reader
		expectError bool
	}{
		{
			name: "valid get",
			module: core.Module{
				Namespace: "tier",
				Name:      "s3",
				Provider:  "aws",
				Version:   "1.0.0",
			},
			data: testModuleData(map[string]string{
				"main.tf": `name = "foo"`,
			}),
		},
		{
			name: "invalid get",
			module: core.Module{
				Namespace: "tier",
				Name:      "s3",
				Provider:  "aws",
			},
			data: testModuleData(map[string]string{
				"main.tf": `name = "foo"`,
			}),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			var (
				ctx     = context.Background()
				storage = NewInmemStorage()
				svc     = NewService(storage)
			)

			_, err := storage.UploadModule(ctx, tc.module.Namespace, tc.module.Name, tc.module.Provider, tc.module.Version, tc.data)
			switch tc.expectError {
			case true:
				assert.Error(err)
			case false:
				assert.NoError(err)
			}

			module, err := svc.GetModule(ctx, tc.module.Namespace, tc.module.Name, tc.module.Provider, tc.module.Version)
			switch tc.expectError {
			case true:
				assert.Error(err)
			case false:
				assert.NoError(err)
				assert.Equal(tc.module, module)
			}
		})
	}
}

func TestService_ListModuleVersions(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name        string
		format      string
		module      core.Module
		versions    []string
		data        io.Reader
		expectError bool
	}{
		{
			name: "valid list default format",
			module: core.Module{
				Namespace: "tier",
				Name:      "s3",
				Provider:  "aws",
			},
			versions: []string{"1.0.0", "2.4.1"},
			data: testModuleData(map[string]string{
				"main.tf": `name = "foo"`,
			}),
		},
		{
			name:   "valid list custom format",
			format: "zip",
			module: core.Module{
				Namespace: "tier",
				Name:      "s3",
				Provider:  "aws",
			},
			versions: []string{"1.0.0", "2.4.1"},
			data: testModuleData(map[string]string{
				"main.tf": `name = "foo"`,
			}),
		},
		{
			name: "invalid list",
			module: core.Module{
				Namespace: "tier",
				Name:      "s3",
			},
			versions: []string{"1.0.0"},
			data: testModuleData(map[string]string{
				"main.tf": `name = "foo"`,
			}),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			var (
				ctx     = context.Background()
				storage = NewInmemStorage(WithInmemArchiveFormat(tc.format))
				svc     = NewService(storage)
			)

			// Make sure this test case is actually doing something
			assert.NotEmpty(tc.versions)

			for _, version := range tc.versions {
				_, err := storage.UploadModule(ctx, tc.module.Namespace, tc.module.Name, tc.module.Provider, version, tc.data)
				switch tc.expectError {
				case true:
					assert.Error(err)
				case false:
					assert.NoError(err)
				}
			}

			modules, err := svc.ListModuleVersions(ctx, tc.module.Namespace, tc.module.Name, tc.module.Provider)
			switch tc.expectError {
			case true:
				assert.Error(err)
			case false:
				assert.NoError(err)
				versions := make([]string, 0)
				for _, module := range modules {
					assert.True(strings.HasSuffix(module.DownloadURL, "."+tc.format))
					module.DownloadURL = ""
					versions = append(versions, module.Version)
					module.Version = ""
					assert.Equal(tc.module, module)
				}
				assert.ElementsMatch(tc.versions, versions)
			}
		})
	}
}
