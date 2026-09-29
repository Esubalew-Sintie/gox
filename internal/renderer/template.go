package renderer

import (
	"fmt"
	"io/fs"
	"text/template"
)

// LoadTemplates parses all .tmpl files from the given filesystem.
// The caller is responsible for providing the embedded or real FS.
func LoadTemplates(fsys fs.FS) (*template.Template, error) {
	tmpl := template.New("gox")

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Only process .tmpl files
		if len(path) < 5 || path[len(path)-5:] != ".tmpl" {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read template %q: %w", path, err)
		}

		name := templateName(path)

		_, err = tmpl.New(name).Parse(string(data))
		if err != nil {
			return fmt.Errorf("parse template %q: %w", path, err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}

	return tmpl, nil
}

// templateName extracts a short template name from the full path.
// e.g. "templates/module/service.go.tmpl" → "service.go.tmpl"
func templateName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}