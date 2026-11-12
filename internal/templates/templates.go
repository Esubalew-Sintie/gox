// Package templates holds the embedded template files of generated projects
// and resolves per-project overrides from .gox/templates/. A template id is
// its path below files/, for example "project/AGENTS.md.tmpl".
package templates

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/fsx"
)

//go:embed all:files
var embedded embed.FS

// OverrideDir is where a project keeps ejected templates, relative to its root.
const OverrideDir = ".gox/templates"

// Source says where a template was read from.
type Source string

// The two sources.
const (
	SourceEmbedded Source = "embedded"
	SourceOverride Source = "override"
)

// ErrNotFound is returned for an unknown template id.
var ErrNotFound = errors.New("template not found")

// Store resolves template ids against an override file system first and the
// embedded files second.
type Store struct {
	base      fs.FS
	overrides fs.FS // nil when the project has no override directory
}

// New returns the store for a project directory. A missing override
// directory is not an error.
func New(projectDir string) (*Store, error) {
	base, err := fs.Sub(embedded, "files")
	if err != nil {
		return nil, fmt.Errorf("embedded templates: %w", err)
	}
	dir := filepath.Join(projectDir, filepath.FromSlash(OverrideDir))
	exists, err := fsx.Exists(dir)
	if err != nil {
		return nil, err
	}
	var overrides fs.FS
	if exists {
		overrides = os.DirFS(dir)
	}
	return &Store{base: base, overrides: overrides}, nil
}

// NewFromFS builds a store from explicit file systems, for tests and tools.
func NewFromFS(base, overrides fs.FS) *Store {
	return &Store{base: base, overrides: overrides}
}

// Read returns a template's content and where it came from.
func (s *Store) Read(id string) ([]byte, Source, error) {
	clean, err := cleanID(id)
	if err != nil {
		return nil, "", err
	}
	if s.overrides != nil {
		data, err := fs.ReadFile(s.overrides, clean)
		if err == nil {
			return data, SourceOverride, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("read override %s: %w", clean, err)
		}
	}
	data, err := fs.ReadFile(s.base, clean)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("%w: %s", ErrNotFound, clean)
		}
		return nil, "", fmt.Errorf("read template %s: %w", clean, err)
	}
	return data, SourceEmbedded, nil
}

// List returns every embedded template id, sorted.
func (s *Store) List() ([]string, error) {
	return walk(s.base)
}

// Overridden returns the ids present in the override directory, sorted.
func (s *Store) Overridden() ([]string, error) {
	if s.overrides == nil {
		return nil, nil
	}
	return walk(s.overrides)
}

// Eject copies the embedded template into projectDir/.gox/templates so it
// can be edited. It refuses to overwrite an existing override and returns the
// written path.
func (s *Store) Eject(id, projectDir string) (string, error) {
	clean, err := cleanID(id)
	if err != nil {
		return "", err
	}
	data, err := fs.ReadFile(s.base, clean)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: %s", ErrNotFound, clean)
		}
		return "", fmt.Errorf("read template %s: %w", clean, err)
	}
	target := filepath.Join(projectDir, filepath.FromSlash(OverrideDir), filepath.FromSlash(clean))
	exists, err := fsx.Exists(target)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("override %s already exists", target)
	}
	if err := fsx.WriteAtomic(target, data, 0o644); err != nil {
		return "", err
	}
	return target, nil
}

func walk(fsys fs.FS) ([]string, error) {
	var ids []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			ids = append(ids, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	sort.Strings(ids)
	return ids, nil
}

func cleanID(id string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(filepath.ToSlash(id), "./"))
	if clean == "." || clean == "" || path.IsAbs(clean) || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid template id %q", id)
	}
	return clean, nil
}

// Both returns the embedded and the override content of an id, for diffs.
func (s *Store) Both(id string) (embedded, override []byte, err error) {
	clean, err := cleanID(id)
	if err != nil {
		return nil, nil, err
	}
	embedded, err = fs.ReadFile(s.base, clean)
	if err != nil {
		return nil, nil, fmt.Errorf("read template %s: %w", clean, err)
	}
	if s.overrides == nil {
		return embedded, nil, fmt.Errorf("%s has no override", clean)
	}
	override, err = fs.ReadFile(s.overrides, clean)
	if err != nil {
		return nil, nil, fmt.Errorf("read override %s: %w", clean, err)
	}
	return embedded, override, nil
}
