package module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

var moduleData = `
resource "aws_s3_bucket" "mod" {
	name = "foo"
}
`

func TestService(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	var (
		namespace = "tier"
		name      = "s3"
		provider  = "aws"
		version   = "1.0.0"
	)

	var (
		ctx      = context.Background()
		registry = NewInmemRegistry()
		svc      = NewService(registry)
		fs       = afero.NewMemMapFs()
		buf      = new(bytes.Buffer)
		gw       = gzip.NewWriter(buf)
		tw       = tar.NewWriter(gw)
	)

	assert.NoError(afero.WriteFile(fs, "main.tf", []byte(moduleData), 0644))

	file, err := fs.Open("main.tf")
	assert.NoError(err)

	stat, err := file.Stat()
	assert.NoError(err)

	header, err := tar.FileInfoHeader(stat, file.Name())
	assert.NoError(err)

	assert.NoError(tw.WriteHeader(header))

	if _, err := io.Copy(tw, strings.NewReader(moduleData)); err != nil {
		panic(err)
	}

	assert.NoError(gw.Close())
	assert.NoError(tw.Close())
	assert.NoError(file.Close())

	file, err = fs.Open("main.tf")
	assert.NoError(err)

	module, err := registry.UploadModule(ctx, namespace, name, provider, version, file)
	assert.NoError(err)
	assert.NoError(file.Close())

	module2, err := svc.GetModule(ctx, namespace, name, provider, version)
	assert.NoError(err)

	expected := Module{
		Namespace: namespace,
		Name:      name,
		Provider:  provider,
		Version:   version,
	}

	assert.Equal(expected, module)
	assert.Equal(module, module2)
}
