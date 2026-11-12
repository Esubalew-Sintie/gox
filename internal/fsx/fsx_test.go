package fsx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/fsx"
)

func TestWriteAtomicCreatesParentsAndReplaces(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a", "b", "file.txt")

	if err := fsx.WriteAtomic(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsx.WriteAtomic(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "two" {
		t.Fatalf("got %q, %v", data, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary file left behind: %v", entries)
	}
}

func TestReadIfExistsAndRemoveEmptyParents(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "x", "y", "z.txt")
	if _, ok, err := fsx.ReadIfExists(path); ok || err != nil {
		t.Fatalf("missing file: ok=%v err=%v", ok, err)
	}
	if err := fsx.WriteAtomic(path, []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, ok, err := fsx.ReadIfExists(path); !ok || err != nil || string(data) != "v" {
		t.Fatalf("existing file: %q %v %v", data, ok, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	fsx.RemoveEmptyParents(root, path)
	if ok, _ := fsx.Exists(filepath.Join(root, "x")); ok {
		t.Fatal("empty parents must be removed")
	}
	if ok, _ := fsx.Exists(root); !ok {
		t.Fatal("root must stay")
	}
}
