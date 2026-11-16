package webui

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"html/template"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/zclconf/go-cty/cty"
)

// Variable is a module input variable.
type Variable struct {
	Name, Type, Description, Default string
	Required, Sensitive              bool
}

// Output is a module output value.
type Output struct {
	Name, Description string
	Sensitive         bool
}

// ModuleDocs is the documentation extracted from a module archive.
type ModuleDocs struct {
	ReadmeHTML template.HTML // empty when no README
	Variables  []Variable    // sorted by Name
	Outputs    []Output      // sorted by Name
	Warnings   []string
}

const (
	maxModuleArchiveBytes = 64 << 20
	maxDocFileBytes       = 1 << 20
)

// markdown never renders raw HTML (no html.WithUnsafe), so its output is safe to embed.
var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

type rootFile struct {
	name string
	data []byte
}

// extractModuleDocs renders the README.md and parses inputs and outputs from the root .tf files of a module archive.
func extractModuleDocs(archive []byte, format string) (*ModuleDocs, error) {
	if len(archive) > maxModuleArchiveBytes {
		return nil, errors.New("module archive exceeds 64 MiB, documentation skipped")
	}

	docs := &ModuleDocs{}
	files, err := readRootFiles(archive, format, docs)
	if err != nil {
		return nil, err
	}

	readmeFound := false
	for _, f := range files {
		switch {
		case !readmeFound && strings.EqualFold(f.name, "README.md"):
			readmeFound = true
			var buf bytes.Buffer
			if err := markdown.Convert(f.data, &buf); err != nil {
				docs.Warnings = append(docs.Warnings, fmt.Sprintf("failed to render %s: %v", f.name, err))
				continue
			}
			// Safe: goldmark escapes raw HTML and dangerous URLs without html.WithUnsafe
			docs.ReadmeHTML = template.HTML(buf.String())
		case strings.HasSuffix(f.name, ".tf"):
			parseTerraformFile(f, docs)
		}
	}

	sort.Slice(docs.Variables, func(i, j int) bool { return docs.Variables[i].Name < docs.Variables[j].Name })
	sort.Slice(docs.Outputs, func(i, j int) bool { return docs.Outputs[i].Name < docs.Outputs[j].Name })
	return docs, nil
}

// readRootFiles returns the regular files at the archive root that are relevant for documentation.
func readRootFiles(archive []byte, format string, docs *ModuleDocs) ([]rootFile, error) {
	var files []rootFile
	add := func(name string, r io.Reader) error {
		name = strings.TrimPrefix(path.Clean(name), "./")
		if strings.Contains(name, "/") || (!strings.EqualFold(name, "README.md") && !strings.HasSuffix(name, ".tf")) {
			return nil
		}
		data, err := io.ReadAll(io.LimitReader(r, maxDocFileBytes+1))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", name, err)
		}
		if len(data) > maxDocFileBytes {
			docs.Warnings = append(docs.Warnings, fmt.Sprintf("skipped %s: larger than 1 MiB", name))
			return nil
		}
		files = append(files, rootFile{name: name, data: data})
		return nil
	}

	switch format {
	case "tar.gz", "tgz":
		gz, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return nil, fmt.Errorf("failed to open gzip archive: %w", err)
		}
		defer func() { _ = gz.Close() }()
		return files, readTar(gz, add)
	case "tar":
		return files, readTar(bytes.NewReader(archive), add)
	case "zip":
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, fmt.Errorf("failed to open zip archive: %w", err)
		}
		for _, f := range zr.File {
			if !f.Mode().IsRegular() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open %s: %w", f.Name, err)
			}
			err = add(f.Name, rc)
			_ = rc.Close()
			if err != nil {
				return nil, err
			}
		}
		return files, nil
	default:
		return nil, fmt.Errorf("unsupported module archive format %q", format)
	}
}

func readTar(r io.Reader, add func(string, io.Reader) error) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read tar archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if err := add(hdr.Name, tr); err != nil {
			return err
		}
	}
}

func parseTerraformFile(f rootFile, docs *ModuleDocs) {
	file, diags := hclsyntax.ParseConfig(f.data, f.name, hcl.InitialPos)
	if diags.HasErrors() {
		docs.Warnings = append(docs.Warnings, fmt.Sprintf("failed to parse %s: %s", f.name, diags.Error()))
		return
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return
	}

	for _, block := range body.Blocks {
		if len(block.Labels) != 1 {
			continue
		}
		attrs := block.Body.Attributes
		switch block.Type {
		case "variable":
			_, hasDefault := attrs["default"]
			docs.Variables = append(docs.Variables, Variable{
				Name:        block.Labels[0],
				Type:        rawExpr(f.data, attrs["type"]),
				Description: stringAttr(f.data, attrs["description"]),
				Default:     rawExpr(f.data, attrs["default"]),
				Required:    !hasDefault,
				Sensitive:   boolAttr(attrs["sensitive"]),
			})
		case "output":
			docs.Outputs = append(docs.Outputs, Output{
				Name:        block.Labels[0],
				Description: stringAttr(f.data, attrs["description"]),
				Sensitive:   boolAttr(attrs["sensitive"]),
			})
		}
	}
}

// rawExpr returns the source text of an attribute expression.
func rawExpr(src []byte, attr *hclsyntax.Attribute) string {
	if attr == nil {
		return ""
	}
	rng := attr.Expr.Range()
	return string(src[rng.Start.Byte:rng.End.Byte])
}

// stringAttr returns the static string value of an attribute, falling back to its source text.
func stringAttr(src []byte, attr *hclsyntax.Attribute) string {
	if attr == nil {
		return ""
	}
	v, diags := attr.Expr.Value(nil)
	if !diags.HasErrors() && v.IsKnown() && !v.IsNull() && v.Type() == cty.String {
		return v.AsString()
	}
	return rawExpr(src, attr)
}

func boolAttr(attr *hclsyntax.Attribute) bool {
	if attr == nil {
		return false
	}
	v, diags := attr.Expr.Value(nil)
	return !diags.HasErrors() && v.IsKnown() && !v.IsNull() && v.Type() == cty.Bool && v.True()
}
