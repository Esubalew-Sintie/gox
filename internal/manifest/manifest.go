package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/fsx"
)

// Dir is the state directory in the project root; File is the manifest in it.
const (
	Dir  = ".gox"
	File = "manifest.json"
)

// Path is the manifest path relative to the project root.
var Path = path.Join(Dir, File)

// Class is who owns a generated file after generation.
type Class string

// The three ownership classes. See ADR 0002.
const (
	// ClassOnce files are written one time and then belong to the developer.
	ClassOnce Class = "once"
	// ClassTool files are regenerated on every run and must not be edited.
	ClassTool Class = "tool"
	// ClassCodemod files are developer-owned; gox only appends or removes its own declarations.
	ClassCodemod Class = "codemod"
)

// Valid reports whether c is one of the three classes.
func (c Class) Valid() bool {
	return c == ClassOnce || c == ClassTool || c == ClassCodemod
}

// Entry records one file gox wrote.
type Entry struct {
	Path     string   `json:"path"`
	Class    Class    `json:"class"`
	Template string   `json:"template,omitempty"`
	Owner    string   `json:"owner,omitempty"`
	Version  string   `json:"gox_version"`
	Hash     string   `json:"sha256"`
	Decls    []string `json:"decls,omitempty"`
}

// Manifest is the content of .gox/manifest.json. Files are kept sorted by
// path so the file diffs cleanly in version control.
type Manifest struct {
	Version string              `json:"gox_version"`
	Files   []Entry             `json:"files"`
	Wiring  map[string][]string `json:"wiring,omitempty"` // constructor arguments by feature/layer, see resolve
}

// ErrNotFound is returned by Load when the project has no manifest.
var ErrNotFound = errors.New("manifest not found")

// New returns an empty manifest stamped with the gox version.
func New(version string) *Manifest {
	return &Manifest{Version: version, Files: []Entry{}}
}

// Load reads dir/.gox/manifest.json.
func Load(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, Dir, File))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read %s: %w", Path, err)
	}
	return Decode(data)
}

// Decode parses manifest content and validates every entry.
func Decode(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", Path, err)
	}
	if m.Files == nil {
		m.Files = []Entry{}
	}
	seen := map[string]bool{}
	for i, e := range m.Files {
		clean, err := CleanPath(e.Path)
		if err != nil {
			return nil, fmt.Errorf("%s: files[%d]: %w", Path, i, err)
		}
		if !e.Class.Valid() {
			return nil, fmt.Errorf("%s: files[%d] %s: unknown class %q", Path, i, clean, e.Class)
		}
		if seen[clean] {
			return nil, fmt.Errorf("%s: %s is listed twice", Path, clean)
		}
		seen[clean] = true
		m.Files[i].Path = clean
	}
	m.sort()
	return &m, nil
}

// Encode renders the manifest as indented JSON with a trailing newline.
func (m *Manifest) Encode() ([]byte, error) {
	m.sort()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", Path, err)
	}
	return append(data, '\n'), nil
}

// Save writes dir/.gox/manifest.json atomically.
func (m *Manifest) Save(dir string) error {
	data, err := m.Encode()
	if err != nil {
		return err
	}
	return fsx.WriteAtomic(filepath.Join(dir, Dir, File), data, 0o644)
}

// Lookup returns the entry for a project-relative path.
func (m *Manifest) Lookup(p string) (Entry, bool) {
	clean, err := CleanPath(p)
	if err != nil {
		return Entry{}, false
	}
	i, found := slices.BinarySearchFunc(m.Files, clean, func(e Entry, target string) int {
		return strings.Compare(e.Path, target)
	})
	if !found {
		return Entry{}, false
	}
	return m.Files[i], true
}

// Set inserts or replaces the entry for e.Path.
func (m *Manifest) Set(e Entry) error {
	clean, err := CleanPath(e.Path)
	if err != nil {
		return err
	}
	if !e.Class.Valid() {
		return fmt.Errorf("%s: unknown class %q", clean, e.Class)
	}
	e.Path = clean
	i, found := slices.BinarySearchFunc(m.Files, clean, func(x Entry, target string) int {
		return strings.Compare(x.Path, target)
	})
	if found {
		m.Files[i] = e
		return nil
	}
	m.Files = slices.Insert(m.Files, i, e)
	return nil
}

// Remove deletes the entry and reports whether it existed.
func (m *Manifest) Remove(p string) bool {
	clean, err := CleanPath(p)
	if err != nil {
		return false
	}
	before := len(m.Files)
	m.Files = slices.DeleteFunc(m.Files, func(e Entry) bool { return e.Path == clean })
	return len(m.Files) != before
}

// ByOwner returns the entries recorded for an owner such as "feature:fleet".
func (m *Manifest) ByOwner(owner string) []Entry {
	var out []Entry
	for _, e := range m.Files {
		if e.Owner == owner {
			out = append(out, e)
		}
	}
	return out
}

// Clone returns a deep copy.
func (m *Manifest) Clone() *Manifest {
	c := &Manifest{Version: m.Version, Files: make([]Entry, len(m.Files))}
	for i, e := range m.Files {
		e.Decls = slices.Clone(e.Decls)
		c.Files[i] = e
	}
	if m.Wiring != nil {
		c.Wiring = make(map[string][]string, len(m.Wiring))
		for k, v := range m.Wiring {
			c.Wiring[k] = slices.Clone(v)
		}
	}
	return c
}

func (m *Manifest) sort() {
	slices.SortFunc(m.Files, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
}

// CleanPath normalises a project-relative path to slash form and rejects
// absolute paths and paths that escape the project.
func CleanPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	slash := filepath.ToSlash(p)
	if path.IsAbs(slash) || filepath.IsAbs(p) {
		return "", fmt.Errorf("path %q must be relative to the project root", p)
	}
	clean := path.Clean(slash)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path %q escapes the project root", p)
	}
	return clean, nil
}

// Hash returns the hex SHA-256 of data, the form stored in the manifest.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HashFile hashes a file on disk.
func HashFile(p string) (string, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", p, err)
	}
	return Hash(data), nil
}
