package webui

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStorage struct {
	listErr  error
	archives map[string][]byte
}

func (f *fakeStorage) ListAllModules(context.Context) ([]core.Module, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []core.Module{
		{Namespace: "acme", Name: "vpc", Provider: "aws", Version: "1.2.0"},
		{Namespace: "acme", Name: "vpc", Provider: "aws", Version: "1.3.0"},
		{Namespace: "acme", Name: "eks", Provider: "aws", Version: "0.1.0"},
	}, nil
}

func (f *fakeStorage) ListAllProviders(context.Context) ([]*core.Provider, error) {
	return []*core.Provider{
		{Namespace: "acme", Name: "dummy", Version: "0.1.0", OS: "linux", Arch: "amd64"},
		{Namespace: "acme", Name: "dummy", Version: "0.1.0", OS: "darwin", Arch: "arm64"},
		{Namespace: "acme", Name: "dummy", Version: "0.2.0", OS: "linux", Arch: "amd64"},
	}, nil
}

func (f *fakeStorage) ListAllMirroredProviders(context.Context) ([]*core.Provider, error) {
	return []*core.Provider{
		{Hostname: "registry.terraform.io", Namespace: "hashicorp", Name: "random", Version: "3.6.0", OS: "linux", Arch: "amd64"},
	}, nil
}

func (f *fakeStorage) ModuleArchive(_ context.Context, namespace, name, provider, version string) ([]byte, error) {
	m := core.Module{Namespace: namespace, Name: name, Provider: provider, Version: version}
	a, ok := f.archives[m.ID(true)]
	if !ok {
		return nil, module.ErrModuleNotFound
	}
	return a, nil
}

func (f *fakeStorage) SigningKeys(_ context.Context, namespace string) (*core.SigningKeys, error) {
	if namespace != "acme" {
		return nil, errors.New("not found")
	}
	return &core.SigningKeys{GPGPublicKeys: []core.GPGPublicKey{{KeyID: "ABCDEF0123456789", Source: "acme"}}}, nil
}

func newTestHandler(t *testing.T, s Storage) http.Handler {
	t.Helper()
	h, err := NewHandler(s, Options{ModuleArchiveFormat: "tar.gz"})
	require.NoError(t, err)
	return h
}

type response struct {
	status int
	header http.Header
	body   string
}

func get(t *testing.T, h http.Handler, target string, headers map[string]string) response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body, err := io.ReadAll(rec.Result().Body)
	require.NoError(t, err)
	return response{status: rec.Code, header: rec.Header(), body: string(body)}
}

func TestHandler(t *testing.T) {
	t.Parallel()

	s := &fakeStorage{archives: map[string][]byte{
		"acme/vpc/aws/1.2.0": tarGz(t, testFiles()),
	}}
	h := newTestHandler(t, s)

	t.Run("root redirects to modules", func(t *testing.T) {
		r := get(t, h, "/ui/", nil)
		assert.Equal(t, http.StatusFound, r.status)
		assert.Equal(t, "/ui/modules", r.header.Get("Location"))
		assert.Contains(t, r.header.Get("Content-Security-Policy"), "script-src 'self'")
	})

	t.Run("module search", func(t *testing.T) {
		r := get(t, h, "/ui/modules?q=VPC", nil)
		assert.Equal(t, http.StatusOK, r.status)
		assert.Contains(t, r.body, "<html")
		assert.Contains(t, r.body, "acme/vpc/aws")
		assert.NotContains(t, r.body, "acme/eks/aws")

		r = get(t, h, "/ui/modules?q=zzz", nil)
		assert.Contains(t, r.body, "No modules found.")
	})

	t.Run("htmx search returns fragment", func(t *testing.T) {
		r := get(t, h, "/ui/modules?q=vpc", map[string]string{"HX-Request": "true"})
		assert.Equal(t, http.StatusOK, r.status)
		assert.NotContains(t, r.body, "<html")
		assert.Contains(t, r.body, "acme/vpc/aws")
	})

	t.Run("boosted navigation returns full page", func(t *testing.T) {
		r := get(t, h, "/ui/modules", map[string]string{"HX-Request": "true", "HX-Boosted": "true"})
		assert.Contains(t, r.body, "<html")
	})

	t.Run("history restore returns full page", func(t *testing.T) {
		r := get(t, h, "/ui/modules?q=vpc", map[string]string{"HX-Request": "true", "HX-History-Restore-Request": "true"})
		assert.Contains(t, r.body, "<html")
		assert.Contains(t, r.body, "acme/vpc/aws")
	})

	t.Run("module redirects to latest version", func(t *testing.T) {
		r := get(t, h, "/ui/modules/acme/vpc/aws", nil)
		assert.Equal(t, http.StatusFound, r.status)
		assert.Equal(t, "/ui/modules/acme/vpc/aws/1.3.0", r.header.Get("Location"))
	})

	t.Run("module version with docs", func(t *testing.T) {
		r := get(t, h, "/ui/modules/acme/vpc/aws/1.2.0", nil)
		assert.Equal(t, http.StatusOK, r.status)
		assert.Contains(t, r.body, `<h1 id="vpc">VPC</h1>`)
		assert.Contains(t, r.body, "cidr")
		assert.Contains(t, r.body, "map(string)")
		assert.Contains(t, r.body, "vpc_id")
		assert.Contains(t, r.body, "example.com/acme/vpc/aws")
		assert.NotContains(t, r.body, "<script>alert")
		assert.NotContains(t, r.body, "javascript:alert")
	})

	t.Run("module version whose archive is missing", func(t *testing.T) {
		r := get(t, h, "/ui/modules/acme/vpc/aws/1.3.0", nil)
		assert.Equal(t, http.StatusNotFound, r.status)
	})

	t.Run("unknown module version", func(t *testing.T) {
		r := get(t, h, "/ui/modules/acme/vpc/aws/9.9.9", nil)
		assert.Equal(t, http.StatusNotFound, r.status)
		r = get(t, h, "/ui/modules/acme/nope/aws", nil)
		assert.Equal(t, http.StatusNotFound, r.status)
	})

	t.Run("provider page", func(t *testing.T) {
		r := get(t, h, "/ui/providers/acme/dummy", nil)
		assert.Equal(t, http.StatusOK, r.status)
		assert.Contains(t, r.body, "0.2.0")
		assert.Contains(t, r.body, "darwin_arm64")
		assert.Contains(t, r.body, "ABCDEF0123456789")
	})

	t.Run("mirror list and page", func(t *testing.T) {
		r := get(t, h, "/ui/mirror", nil)
		assert.Equal(t, http.StatusOK, r.status)
		assert.Contains(t, r.body, "registry.terraform.io/hashicorp/random")

		r = get(t, h, "/ui/mirror/registry.terraform.io/hashicorp/random", nil)
		assert.Equal(t, http.StatusOK, r.status)
		assert.Contains(t, r.body, "https://example.com/v1/mirror/")
	})

	t.Run("static assets", func(t *testing.T) {
		r := get(t, h, "/ui/static/style.css", nil)
		assert.Equal(t, http.StatusOK, r.status)
		assert.Equal(t, "public, max-age=3600", r.header.Get("Cache-Control"))
	})
}

func TestHandler_StorageError(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, &fakeStorage{listErr: errors.New("boom")})
	r := get(t, h, "/ui/modules", nil)
	assert.Equal(t, http.StatusInternalServerError, r.status)
	assert.Contains(t, r.body, "Failed to load registry contents")
}
