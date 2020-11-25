package module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
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

		tw.WriteHeader(hdr)
		tw.Write([]byte(moduleData))
	}

	return buf
}

func TestService_GetModule(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name        string
		module      Module
		data        io.Reader
		expectError bool
	}{
		{
			name: "valid get",
			module: Module{
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
			module: Module{
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
				ctx      = context.Background()
				registry = NewInmemRegistry()
				svc      = NewService(registry)
			)

			_, err := registry.UploadModule(ctx, tc.module.Namespace, tc.module.Name, tc.module.Provider, tc.module.Version, tc.data)
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
