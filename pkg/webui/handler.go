package webui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/module"

	"github.com/maypok86/otter/v2"
)

// Options configures the web UI handler.
type Options struct {
	// ModuleArchiveFormat is the archive format of modules in the storage, e.g. tar.gz
	ModuleArchiveFormat string
	// CatalogTTL is the duration the storage listing is cached, 0 disables caching
	CatalogTTL time.Duration
	// Version is the boring-registry version shown in the footer
	Version string
}

const (
	requestTimeout = 20 * time.Second

	contentSecurityPolicy = "default-src 'self'; img-src 'self' https: data:; style-src 'self'; script-src 'self'; frame-ancestors 'none'"
)

type handler struct {
	storage       Storage
	archiveFormat string
	catalog       *catalogLoader
	docs          *otter.Cache[string, *ModuleDocs]
	pages         map[string]*template.Template
	appVersion    string
	assetVersion  string
}

// pageData is the data passed to every template.
type pageData struct {
	Title  string
	Active string // modules, providers or mirror
	Query  string

	// List pages
	Modules   []ModuleEntry
	Providers []ProviderEntry

	// Module page
	Module    *ModuleEntry
	Version   string
	Docs      *ModuleDocs
	DocsError string

	// Provider and mirror pages
	Provider    *ProviderEntry
	SigningKeys []core.GPGPublicKey

	Usage string

	// Error page
	Message string

	// Set by render
	AppVersion   string
	AssetVersion string // cache-busting suffix for static assets
}

// NewHandler returns the read-only web UI handler serving everything under /ui/.
func NewHandler(s Storage, opts Options) (http.Handler, error) {
	pages := map[string]*template.Template{}
	for _, page := range []string{"modules", "module", "providers", "provider", "mirror", "mirror_provider", "error"} {
		t, err := template.ParseFS(templateFS, "templates/layout.html", "templates/"+page+".html")
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", page, err)
		}
		pages[page] = t
	}

	docs, err := otter.New(&otter.Options[string, *ModuleDocs]{MaximumSize: 256})
	if err != nil {
		return nil, fmt.Errorf("failed to create documentation cache: %w", err)
	}

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	assetVersion, err := hashFS(staticSub)
	if err != nil {
		return nil, fmt.Errorf("failed to hash static assets: %w", err)
	}

	h := &handler{
		storage:       s,
		archiveFormat: opts.ModuleArchiveFormat,
		catalog:       &catalogLoader{storage: s, ttl: opts.CatalogTTL},
		docs:          docs,
		pages:         pages,
		appVersion:    opts.Version,
		assetVersion:  assetVersion,
	}

	static := http.StripPrefix("/ui/static/", http.FileServerFS(staticSub))
	mux := http.NewServeMux()
	mux.Handle("GET /ui/{$}", http.RedirectHandler("/ui/modules", http.StatusFound))
	mux.HandleFunc("GET /ui/static/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		static.ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /ui/modules", h.listModules)
	mux.HandleFunc("GET /ui/modules/{namespace}/{name}/{provider}", h.latestModule)
	mux.HandleFunc("GET /ui/modules/{namespace}/{name}/{provider}/{version}", h.moduleVersion)
	mux.HandleFunc("GET /ui/providers", h.listProviders)
	mux.HandleFunc("GET /ui/providers/{namespace}/{name}", h.provider)
	mux.HandleFunc("GET /ui/mirror", h.listMirror)
	mux.HandleFunc("GET /ui/mirror/{hostname}/{namespace}/{name}", h.mirrorProvider)

	return securityHeaders(mux), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// loadCatalog returns the catalog or renders the error page and returns nil.
func (h *handler) loadCatalog(ctx context.Context, w http.ResponseWriter, r *http.Request) *catalog {
	c, err := h.catalog.get(ctx)
	if err != nil {
		slog.Error("failed to load registry contents", "error", err)
		h.renderError(w, r, http.StatusInternalServerError, "Failed to load registry contents")
		return nil
	}
	return c
}

func (h *handler) listModules(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	c := h.loadCatalog(ctx, w, r)
	if c == nil {
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	modules := []ModuleEntry{}
	for _, m := range c.Modules {
		if matches(q, m.Namespace, m.Name, m.Provider) {
			modules = append(modules, m)
		}
	}
	h.render(w, r, http.StatusOK, "modules", &pageData{Title: "Modules", Active: "modules", Query: q, Modules: modules})
}

func (h *handler) findModule(c *catalog, r *http.Request) *ModuleEntry {
	ns, name, provider := r.PathValue("namespace"), r.PathValue("name"), r.PathValue("provider")
	for i := range c.Modules {
		m := &c.Modules[i]
		if m.Namespace == ns && m.Name == name && m.Provider == provider {
			return m
		}
	}
	return nil
}

func (h *handler) latestModule(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	c := h.loadCatalog(ctx, w, r)
	if c == nil {
		return
	}

	m := h.findModule(c, r)
	if m == nil {
		h.renderError(w, r, http.StatusNotFound, "Module not found")
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/ui/modules/%s/%s/%s/%s", m.Namespace, m.Name, m.Provider, m.Latest), http.StatusFound)
}

func (h *handler) moduleVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	c := h.loadCatalog(ctx, w, r)
	if c == nil {
		return
	}

	m := h.findModule(c, r)
	v := r.PathValue("version")
	if m == nil || !slices.Contains(m.Versions, v) {
		h.renderError(w, r, http.StatusNotFound, "Module version not found")
		return
	}

	id := (&core.Module{Namespace: m.Namespace, Name: m.Name, Provider: m.Provider, Version: v}).ID(true)
	data := &pageData{
		Title:   m.Namespace + "/" + m.Name + "/" + m.Provider,
		Active:  "modules",
		Module:  m,
		Version: v,
		Usage: fmt.Sprintf("module %q {\n  source  = \"%s/%s/%s/%s\"\n  version = %q\n}\n",
			m.Name, r.Host, m.Namespace, m.Name, m.Provider, v),
	}

	docs, ok := h.docs.GetIfPresent(id)
	if !ok {
		archive, err := h.storage.ModuleArchive(ctx, m.Namespace, m.Name, m.Provider, v)
		if errors.Is(err, module.ErrModuleNotFound) {
			h.renderError(w, r, http.StatusNotFound, "Module version not found")
			return
		}
		if err == nil {
			docs, err = extractModuleDocs(archive, h.archiveFormat)
		}
		if err != nil {
			slog.Error("failed to load module docs", "module", id, "error", err)
			data.DocsError = "Documentation could not be loaded"
		} else {
			h.docs.Set(id, docs)
		}
	}
	data.Docs = docs
	h.render(w, r, http.StatusOK, "module", data)
}

func (h *handler) listProviders(w http.ResponseWriter, r *http.Request) {
	h.listProviderEntries(w, r, "providers", "Providers", func(c *catalog) []ProviderEntry { return c.Providers })
}

func (h *handler) listMirror(w http.ResponseWriter, r *http.Request) {
	h.listProviderEntries(w, r, "mirror", "Mirror", func(c *catalog) []ProviderEntry { return c.Mirrored })
}

func (h *handler) listProviderEntries(w http.ResponseWriter, r *http.Request, page, title string, entries func(*catalog) []ProviderEntry) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	c := h.loadCatalog(ctx, w, r)
	if c == nil {
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	providers := []ProviderEntry{}
	for _, p := range entries(c) {
		parts := []string{p.Namespace, p.Name}
		if p.Hostname != "" {
			parts = append([]string{p.Hostname}, parts...)
		}
		if matches(q, parts...) {
			providers = append(providers, p)
		}
	}
	h.render(w, r, http.StatusOK, page, &pageData{Title: title, Active: page, Query: q, Providers: providers})
}

func findProvider(entries []ProviderEntry, hostname, namespace, name string) *ProviderEntry {
	for i := range entries {
		p := &entries[i]
		if p.Hostname == hostname && p.Namespace == namespace && p.Name == name {
			return p
		}
	}
	return nil
}

func (h *handler) provider(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	c := h.loadCatalog(ctx, w, r)
	if c == nil {
		return
	}

	p := findProvider(c.Providers, "", r.PathValue("namespace"), r.PathValue("name"))
	if p == nil {
		h.renderError(w, r, http.StatusNotFound, "Provider not found")
		return
	}

	data := &pageData{
		Title:    p.Namespace + "/" + p.Name,
		Active:   "providers",
		Provider: p,
		Usage: fmt.Sprintf("terraform {\n  required_providers {\n    %s = {\n      source  = \"%s/%s/%s\"\n      version = %q\n    }\n  }\n}\n",
			p.Name, r.Host, p.Namespace, p.Name, p.Latest),
	}
	keys, err := h.storage.SigningKeys(ctx, p.Namespace)
	if err != nil {
		slog.Debug("failed to load signing keys", "namespace", p.Namespace, "error", err)
	} else if keys != nil {
		data.SigningKeys = keys.GPGPublicKeys
	}
	h.render(w, r, http.StatusOK, "provider", data)
}

func (h *handler) mirrorProvider(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	c := h.loadCatalog(ctx, w, r)
	if c == nil {
		return
	}

	p := findProvider(c.Mirrored, r.PathValue("hostname"), r.PathValue("namespace"), r.PathValue("name"))
	if p == nil {
		h.renderError(w, r, http.StatusNotFound, "Mirrored provider not found")
		return
	}

	h.render(w, r, http.StatusOK, "mirror_provider", &pageData{
		Title:    p.Hostname + "/" + p.Namespace + "/" + p.Name,
		Active:   "mirror",
		Provider: p,
		Usage:    fmt.Sprintf("provider_installation {\n  network_mirror {\n    url = \"https://%s/v1/mirror/\"\n  }\n}\n", r.Host),
	})
}

func (h *handler) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.render(w, r, status, "error", &pageData{Title: http.StatusText(status), Message: message})
}

// render executes the page into a buffer first, so half-written pages are never sent.
// htmx requests that are neither boosted nor history restores only receive the "results" fragment of list pages.
func (h *handler) render(w http.ResponseWriter, r *http.Request, status int, page string, data *pageData) {
	t := h.pages[page]
	name := "layout"
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") == "" &&
		r.Header.Get("HX-History-Restore-Request") != "true" && t.Lookup("results") != nil {
		name = "results"
	}

	data.AppVersion = h.appVersion
	data.AssetVersion = h.assetVersion

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		slog.Error("failed to render template", "page", page, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Vary", "HX-Request, HX-History-Restore-Request")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// hashFS returns a short content hash of every file in fsys, used to bust browser caches of static assets.
func hashFS(fsys fs.FS) (string, error) {
	sum := sha256.New()
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		sum.Write([]byte(p))
		sum.Write(b)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil))[:12], nil
}

// matches reports whether q is a case-insensitive substring of the parts joined by "/".
func matches(q string, parts ...string) bool {
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.Join(parts, "/")), strings.ToLower(q))
}
