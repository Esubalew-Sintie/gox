// Package render executes templates from the template store and formats Go
// output. Rendering is deterministic: no template function depends on time,
// randomness or the environment, so gox check can regenerate to memory and
// compare byte for byte.
package render

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"

	"github.com/esubalew-Sintie/gox/internal/naming"
	"github.com/esubalew-Sintie/gox/internal/templates"
)

// Renderer executes templates by id.
type Renderer struct {
	store *templates.Store
	funcs template.FuncMap
}

// New returns a renderer over the store. localPrefix is the generated
// project's module path, used to group its imports after third-party ones.
func New(store *templates.Store, localPrefix string) *Renderer {
	imports.LocalPrefix = localPrefix
	return &Renderer{store: store, funcs: Funcs()}
}

// Render executes the template with data. Missing keys are errors.
func (r *Renderer) Render(id string, data any) ([]byte, error) {
	content, src, err := r.store.Read(id)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New(id).Funcs(r.funcs).Option("missingkey=error").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %s (%s): %w", id, src, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template %s (%s): %w", id, src, err)
	}
	return buf.Bytes(), nil
}

// RenderGo renders a template that produces a Go file and formats it.
// filename is the output path, used in error messages and import grouping.
func (r *Renderer) RenderGo(id, filename string, data any) ([]byte, error) {
	out, err := r.Render(id, data)
	if err != nil {
		return nil, err
	}
	return Format(filename, out)
}

// Format runs goimports on Go source: gofmt formatting, unused imports
// removed, missing standard-library imports added.
func Format(filename string, src []byte) ([]byte, error) {
	out, err := imports.Process(filename, src, &imports.Options{Comments: true, TabIndent: true, TabWidth: 8})
	if err != nil {
		return nil, fmt.Errorf("format %s: %w", filename, err)
	}
	return out, nil
}

// Funcs is the template function map. Every function is pure.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"pascal":   nameFunc(func(n *naming.Name) string { return n.Pascal }),
		"camel":    nameFunc(func(n *naming.Name) string { return n.Camel }),
		"snake":    nameFunc(func(n *naming.Name) string { return n.Snake }),
		"kebab":    nameFunc(func(n *naming.Name) string { return n.Kebab }),
		"plural":   nameFunc(func(n *naming.Name) string { return n.Plural }),
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
		"title":    titleFirst,
		"quote":    strconv.Quote,
		"join":     strings.Join,
		"backtick": func() string { return "`" },
		"repeat":   strings.Repeat,
	}
}

func nameFunc(pick func(*naming.Name) string) func(string) (string, error) {
	return func(s string) (string, error) {
		n, err := naming.Parse(s)
		if err != nil {
			return "", err
		}
		return pick(n), nil
	}
}

func titleFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
