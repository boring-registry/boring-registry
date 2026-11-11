package webui

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testReadme = "# VPC\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1))\n"

	testVariables = `
variable "tags" {
  type        = map(string)
  description = "Tags to apply"
  default     = {}
}

variable "cidr" {
  type        = string
  description = "VPC CIDR block"
}

variable "token" {
  type      = string
  sensitive = true
  default   = null
}
`

	testOutputs = `
output "vpc_id" {
  description = "ID of the VPC"
  value       = "vpc-123"
}
`

	testSubmodule = `
variable "ignored" {
  type = string
}
`
)

type archiveFile struct {
	name, body string
}

func testFiles() []archiveFile {
	return []archiveFile{
		{"README.md", testReadme},
		{"./variables.tf", testVariables},
		{"outputs.tf", testOutputs},
		{"bad.tf", `variable "broken" {`},
		{"modules/sub/variables.tf", testSubmodule},
	}
}

func tarGz(t *testing.T, files []archiveFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o644, Size: int64(len(f.body)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(f.body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func zipArchive(t *testing.T, files []archiveFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.name)
		require.NoError(t, err)
		_, err = w.Write([]byte(f.body))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestExtractModuleDocs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format  string
		archive func(*testing.T, []archiveFile) []byte
	}{
		{"tar.gz", tarGz},
		{"tgz", tarGz},
		{"zip", zipArchive},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()

			docs, err := extractModuleDocs(tt.archive(t, testFiles()), tt.format)
			require.NoError(t, err)

			html := string(docs.ReadmeHTML)
			assert.Contains(t, html, `<h1 id="vpc">VPC</h1>`)
			assert.Contains(t, html, "<table>")
			assert.NotContains(t, html, "<script>")
			assert.NotContains(t, html, "javascript:")

			assert.Equal(t, []Variable{
				{Name: "cidr", Type: "string", Description: "VPC CIDR block", Required: true},
				{Name: "tags", Type: "map(string)", Description: "Tags to apply", Default: "{}"},
				{Name: "token", Type: "string", Default: "null", Sensitive: true},
			}, docs.Variables)
			assert.Equal(t, []Output{{Name: "vpc_id", Description: "ID of the VPC"}}, docs.Outputs)

			require.Len(t, docs.Warnings, 1)
			assert.True(t, strings.HasPrefix(docs.Warnings[0], "failed to parse bad.tf: "), docs.Warnings[0])
		})
	}
}

func TestExtractModuleDocs_NoReadme(t *testing.T) {
	t.Parallel()

	docs, err := extractModuleDocs(tarGz(t, []archiveFile{{"main.tf", `locals {}`}}), "tar.gz")
	require.NoError(t, err)
	assert.Empty(t, docs.ReadmeHTML)
	assert.Empty(t, docs.Variables)
	assert.Empty(t, docs.Outputs)
	assert.Empty(t, docs.Warnings)
}

func TestExtractModuleDocs_LargeFile(t *testing.T) {
	t.Parallel()

	large := strings.Repeat("#", maxDocFileBytes+1)
	docs, err := extractModuleDocs(tarGz(t, []archiveFile{{"README.md", large}}), "tar.gz")
	require.NoError(t, err)
	assert.Empty(t, docs.ReadmeHTML)
	assert.Equal(t, []string{"skipped README.md: larger than 1 MiB"}, docs.Warnings)
}

func TestExtractModuleDocs_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	_, err := extractModuleDocs([]byte("irrelevant"), "7z")
	assert.EqualError(t, err, `unsupported module archive format "7z"`)
}
