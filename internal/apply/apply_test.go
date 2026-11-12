package apply_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/apply"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/transcript"
)

const goMod = "module example.com/t\n\ngo 1.26\n"
const mainV1 = "package main\n\nfunc main() {}\n"

func project(t *testing.T) (string, *manifest.Manifest) {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", goMod)
	write(t, root, "main.go", mainV1)
	m := manifest.New("test")
	_ = m.Set(manifest.Entry{Path: "main.go", Class: manifest.ClassTool, Hash: manifest.Hash([]byte(mainV1)), Owner: "project"})
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	return root, m
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, rel string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data), true
}

func build(t *testing.T, root string, m *manifest.Manifest, desired []plan.Desired, remove []string) *plan.Plan {
	t.Helper()
	p, err := plan.New(root, m, plan.Options{Version: "test"}).Build(desired, remove)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// cleanGit pretends the project is a clean repository.
type cleanGit struct{ real apply.Runner }

func (r cleanGit) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	if name == "git" {
		return nil, nil
	}
	return r.real.Run(ctx, dir, name, args...)
}

func TestApplyWritesVerifiesAndRecords(t *testing.T) {
	root, m := project(t)
	spec := &manifest.Spec{Version: 2, Module: "example.com/t", Arch: "hexagonal", HTTP: "chi", DB: "memory"}
	p := build(t, root, m, []plan.Desired{
		{Path: "internal/x/x.go", Class: manifest.ClassOnce, Owner: "feature:x", Content: []byte("package x\n")},
		{Path: "main.go", Class: manifest.ClassTool, Owner: "project", Content: []byte("package main\n\nimport _ \"example.com/t/internal/x\"\n\nfunc main() {}\n")},
	}, nil)
	p.Spec = spec

	var out bytes.Buffer
	a := &apply.Applier{Root: root, Runner: cleanGit{apply.ExecRunner{}}, Out: transcript.New(&out, false)}
	if err := a.Apply(context.Background(), p, apply.Options{}); err != nil {
		t.Fatal(err)
	}
	if got, ok := read(t, root, "internal/x/x.go"); !ok || got != "package x\n" {
		t.Fatalf("created file: %q %v", got, ok)
	}
	saved, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := saved.Lookup("internal/x/x.go"); !ok || e.Owner != "feature:x" || e.Version != "test" {
		t.Fatalf("manifest entry: %+v %v", e, ok)
	}
	if _, err := manifest.LoadSpec(root); err != nil {
		t.Fatalf("spec written: %v", err)
	}
	if !strings.Contains(out.String(), "create  internal/x/x.go") || !strings.Contains(out.String(), "regen  main.go") {
		t.Fatalf("transcript:\n%s", out.String())
	}
}

func TestApplyRollsBackWhenBuildFails(t *testing.T) {
	root, m := project(t)
	before, _ := manifest.Load(root)
	p := build(t, root, m, []plan.Desired{
		{Path: "main.go", Class: manifest.ClassTool, Owner: "project", Content: []byte("package main\n\nfunc main() { helper() }\n")},
		{Path: "broken/b.go", Class: manifest.ClassOnce, Owner: "project", Content: []byte("package broken\n\nfunc {\n")},
	}, nil)

	a := &apply.Applier{Root: root, Runner: cleanGit{apply.ExecRunner{}}}
	err := a.Apply(context.Background(), p, apply.Options{})
	var verr *apply.VerifyError
	if !errors.As(err, &verr) || verr.Command != "go build ./..." || !strings.Contains(verr.Output, "broken") {
		t.Fatalf("expected build failure, got %v", err)
	}
	if got, _ := read(t, root, "main.go"); got != mainV1 {
		t.Fatalf("main.go not restored: %q", got)
	}
	if _, ok := read(t, root, "broken/b.go"); ok {
		t.Fatal("created file not removed")
	}
	if _, err := os.Stat(filepath.Join(root, "broken")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created directory not removed")
	}
	after, _ := manifest.Load(root)
	if len(after.Files) != len(before.Files) || after.Files[0].Hash != before.Files[0].Hash {
		t.Fatal("manifest changed despite rollback")
	}
}

func TestApplyDeleteAndMissing(t *testing.T) {
	root, m := project(t)
	write(t, root, "internal/old/old.go", "package old\n")
	_ = m.Set(manifest.Entry{Path: "internal/old/old.go", Class: manifest.ClassOnce, Hash: manifest.Hash([]byte("package old\n"))})
	_ = m.Set(manifest.Entry{Path: "internal/gone/gone.go", Class: manifest.ClassOnce, Hash: "x"})
	p := build(t, root, m, nil, []string{"internal/old/old.go", "internal/gone/gone.go"})

	a := &apply.Applier{Root: root, Runner: cleanGit{apply.ExecRunner{}}}
	if err := a.Apply(context.Background(), p, apply.Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("empty parents must be removed after delete")
	}
	saved, _ := manifest.Load(root)
	if len(saved.Files) != 1 {
		t.Fatalf("manifest after delete: %+v", saved.Files)
	}
}

type fakeGit struct{ porcelain string }

func (r fakeGit) Run(_ context.Context, _, name string, args ...string) ([]byte, error) {
	if name == "git" && args[0] == "status" {
		return []byte(r.porcelain), nil
	}
	return nil, nil
}

func TestApplyRefusesDirtyTreeAndConflicts(t *testing.T) {
	root, m := project(t)
	p := build(t, root, m, []plan.Desired{{Path: "n.go", Class: manifest.ClassOnce, Content: []byte("package main\n")}}, nil)

	a := &apply.Applier{Root: root, Runner: fakeGit{" M main.go\n"}}
	if err := a.Apply(context.Background(), p, apply.Options{}); !errors.Is(err, apply.ErrDirty) {
		t.Fatalf("dirty: %v", err)
	}
	if _, ok := read(t, root, "n.go"); ok {
		t.Fatal("nothing may be written on a dirty tree")
	}
	if err := a.Apply(context.Background(), p, apply.Options{AllowDirty: true, SkipVerify: true}); err != nil {
		t.Fatal(err)
	}

	write(t, root, "main.go", "edited\n")
	conflicted := build(t, root, m, []plan.Desired{{Path: "main.go", Class: manifest.ClassTool, Content: []byte("x")}}, nil)
	if err := a.Apply(context.Background(), conflicted, apply.Options{AllowDirty: true}); !errors.Is(err, apply.ErrConflicts) {
		t.Fatalf("conflict: %v", err)
	}
}
