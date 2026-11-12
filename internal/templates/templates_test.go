package templates_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/esubalew-Sintie/gox/internal/templates"
)

func TestEmbeddedTemplatesAreListedAndReadable(t *testing.T) {
	s, err := templates.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ids, err := s.List()
	if err != nil || len(ids) == 0 {
		t.Fatalf("list: %v %v", ids, err)
	}
	data, src, err := s.Read("project/AGENTS.md.tmpl")
	if err != nil || src != templates.SourceEmbedded || !strings.Contains(string(data), "{{ .Module }}") {
		t.Fatalf("read: %v %s", err, src)
	}
	if _, _, err := s.Read("project/missing.tmpl"); !errors.Is(err, templates.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	for _, bad := range []string{"", "../x", "/abs"} {
		if _, _, err := s.Read(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

func TestOverrideWins(t *testing.T) {
	base := fstest.MapFS{"a/one.tmpl": {Data: []byte("base one")}, "a/two.tmpl": {Data: []byte("base two")}}
	over := fstest.MapFS{"a/one.tmpl": {Data: []byte("override one")}}
	s := templates.NewFromFS(base, over)

	data, src, err := s.Read("./a/one.tmpl")
	if err != nil || src != templates.SourceOverride || string(data) != "override one" {
		t.Fatalf("override: %q %s %v", data, src, err)
	}
	data, src, err = s.Read("a/two.tmpl")
	if err != nil || src != templates.SourceEmbedded || string(data) != "base two" {
		t.Fatalf("fallback: %q %s %v", data, src, err)
	}
	ids, _ := s.Overridden()
	if len(ids) != 1 || ids[0] != "a/one.tmpl" {
		t.Fatalf("overridden: %v", ids)
	}
}

func TestEject(t *testing.T) {
	dir := t.TempDir()
	s, err := templates.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	target, err := s.Eject("project/gitignore.tmpl", dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".gox", "templates", "project", "gitignore.tmpl")
	if target != want {
		t.Fatalf("target %s", target)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eject("project/gitignore.tmpl", dir); err == nil {
		t.Fatal("second eject must refuse to overwrite")
	}
	// A new store for the same project now resolves the override.
	s2, err := templates.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, src, err := s2.Read("project/gitignore.tmpl"); err != nil || src != templates.SourceOverride {
		t.Fatalf("after eject: %s %v", src, err)
	}
}
