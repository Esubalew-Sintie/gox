package renderer

import (
	"bytes"
	"fmt"
	"text/template"
)

// Renderer executes named Go templates against data.
type Renderer struct {
	templates *template.Template
}

// New creates a Renderer from a pre-parsed template set.
func New(templates *template.Template) *Renderer {
	return &Renderer{
		templates: templates,
	}
}

// Render executes a named template and returns the rendered bytes.
func (r *Renderer) Render(name string, data any) ([]byte, error) {
	if r.templates == nil {
		return nil, fmt.Errorf("renderer templates are nil")
	}

	var buffer bytes.Buffer

	if err := r.templates.ExecuteTemplate(&buffer, name, data); err != nil {
		return nil, fmt.Errorf("render template %q: %w", name, err)
	}

	return buffer.Bytes(), nil
}