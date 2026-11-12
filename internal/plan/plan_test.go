package plan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/plan"
)

type fixture struct {
	t    *testing.T
	root string
	m    *manifest.Manifest
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return &fixture{t: t, root: t.TempDir(), m: manifest.New("test")}
}

func (f *fixture) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// generated writes a file and records it in the manifest as gox wrote it.
func (f *fixture) generated(path string, class manifest.Class, content string) {
	f.t.Helper()
	f.write(path, content)
	if err := f.m.Set(manifest.Entry{Path: path, Class: class, Hash: manifest.Hash([]byte(content)), Owner: "project"}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) build(opts plan.Options, desired []plan.Desired, remove []string) *plan.Plan {
	f.t.Helper()
	p, err := plan.New(f.root, f.m, opts).Build(desired, remove)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func want(t *testing.T, p *plan.Plan, path string, op plan.Op) plan.Step {
	t.Helper()
	for _, s := range p.Steps {
		if s.Path == path {
			if s.Op != op {
				t.Fatalf("%s: got %s (%s), want %s", path, s.Op, s.Reason, op)
			}
			return s
		}
	}
	t.Fatalf("%s: no step", path)
	return plan.Step{}
}

func tool(path, content string) plan.Desired {
	return plan.Desired{Path: path, Class: manifest.ClassTool, Owner: "project", Content: []byte(content)}
}

func once(path, content string) plan.Desired {
	return plan.Desired{Path: path, Class: manifest.ClassOnce, Owner: "project", Content: []byte(content)}
}

func TestToolFiles(t *testing.T) {
	f := newFixture(t)
	f.generated("app/same.go", manifest.ClassTool, "v1\n")
	f.generated("app/changed.go", manifest.ClassTool, "v1\n")
	f.generated("app/edited.go", manifest.ClassTool, "v1\n")
	f.write("app/edited.go", "developer edit\n")
	f.write("app/unknown.go", "not ours\n")
	f.generated("app/gone.go", manifest.ClassTool, "v1\n")
	_ = os.Remove(filepath.Join(f.root, "app", "gone.go"))

	desired := []plan.Desired{
		tool("app/same.go", "v1\n"), tool("app/changed.go", "v2\n"), tool("app/edited.go", "v2\n"),
		tool("app/unknown.go", "v1\n"), tool("app/gone.go", "v1\n"), tool("app/new.go", "v1\n"),
	}
	p := f.build(plan.Options{Version: "test"}, desired, nil)

	want(t, p, "app/same.go", plan.OpIdentical)
	want(t, p, "app/changed.go", plan.OpRegen)
	want(t, p, "app/edited.go", plan.OpConflict)
	want(t, p, "app/unknown.go", plan.OpConflict)
	s := want(t, p, "app/gone.go", plan.OpCreate)
	if s.Reason != "restored" {
		t.Fatalf("gone reason: %q", s.Reason)
	}
	want(t, p, "app/new.go", plan.OpCreate)
	if len(p.Conflicts()) != 2 || p.Changes() != 3 {
		t.Fatalf("conflicts %d changes %d", len(p.Conflicts()), p.Changes())
	}
	// Conflicts leave the manifest untouched; others are recorded with the new hash.
	if e, _ := p.Manifest.Lookup("app/edited.go"); e.Hash != manifest.Hash([]byte("v1\n")) {
		t.Fatal("conflict must not change the manifest")
	}
	if e, _ := p.Manifest.Lookup("app/changed.go"); e.Hash != manifest.Hash([]byte("v2\n")) || e.Version != "test" {
		t.Fatalf("regen entry: %+v", e)
	}
	if _, ok := p.Manifest.Lookup("app/unknown.go"); ok {
		t.Fatal("unknown conflict must not be recorded")
	}
	// The planner's own manifest is never modified.
	if _, ok := f.m.Lookup("app/new.go"); ok {
		t.Fatal("planner manifest mutated")
	}

	forced := f.build(plan.Options{Force: true}, desired, nil)
	want(t, forced, "app/edited.go", plan.OpRegen)
	want(t, forced, "app/unknown.go", plan.OpRegen)

	adopted := f.build(plan.Options{Adopt: true}, desired, nil)
	s = want(t, adopted, "app/edited.go", plan.OpAdopt)
	if e, _ := adopted.Manifest.Lookup("app/edited.go"); e.Class != manifest.ClassOnce || e.Hash != manifest.Hash([]byte("developer edit\n")) || s.Class != manifest.ClassOnce {
		t.Fatalf("adopt entry: %+v", e)
	}
}

func TestOnceFiles(t *testing.T) {
	f := newFixture(t)
	f.generated("internal/a.go", manifest.ClassOnce, "v1\n")
	f.write("internal/a.go", "edited by developer\n")
	f.write("internal/b.go", "theirs\n")
	f.generated("internal/removed.go", manifest.ClassOnce, "v1\n")
	_ = os.Remove(filepath.Join(f.root, "internal", "removed.go"))

	p := f.build(plan.Options{}, []plan.Desired{
		once("internal/a.go", "v2\n"), once("internal/b.go", "v1\n"), once("internal/removed.go", "v1\n"), once("internal/new.go", "v1\n"),
		{Path: "internal/errors.go", Class: manifest.ClassCodemod, Content: []byte("package x\n")},
	}, nil)
	for _, path := range []string{"internal/a.go", "internal/b.go", "internal/removed.go"} {
		want(t, p, path, plan.OpKeep)
	}
	want(t, p, "internal/new.go", plan.OpCreate)
	want(t, p, "internal/errors.go", plan.OpCreate)
	if s := want(t, p, "internal/removed.go", plan.OpKeep); !strings.Contains(s.Reason, "--force") {
		t.Fatalf("removed reason: %q", s.Reason)
	}
	if _, ok := p.Manifest.Lookup("internal/b.go"); ok {
		t.Fatal("a file gox did not write must not enter the manifest")
	}
	if e, ok := p.Manifest.Lookup("internal/errors.go"); !ok || e.Class != manifest.ClassCodemod {
		t.Fatalf("codemod create: %+v %v", e, ok)
	}
}

func TestCodemodFiles(t *testing.T) {
	f := newFixture(t)
	f.generated("internal/service/service.go", manifest.ClassCodemod, "package service\n")
	f.write("internal/service/service.go", "package service\n\ntype Mine interface{}\n")
	edited := []plan.Desired{
		{Path: "internal/service/service.go", Class: manifest.ClassCodemod, Content: []byte("package service\n\ntype Mine interface{}\n\ntype FleetService interface{}\n"), Decls: []string{"FleetService"}},
		{Path: "internal/handler/handler.go", Class: manifest.ClassCodemod, Content: []byte("package handler\n"), Decls: nil},
	}
	p := f.build(plan.Options{}, edited, nil)
	s := want(t, p, "internal/service/service.go", plan.OpModify)
	if !s.Writes() || !strings.Contains(p.Diff(s), "+type FleetService interface{}") {
		t.Fatalf("modify step: writes=%v diff=%s", s.Writes(), p.Diff(s))
	}
	if e, _ := p.Manifest.Lookup("internal/service/service.go"); len(e.Decls) != 1 || e.Decls[0] != "FleetService" || e.Hash != manifest.Hash(edited[0].Content) {
		t.Fatalf("modify entry: %+v", e)
	}
	want(t, p, "internal/handler/handler.go", plan.OpCreate)

	same := []plan.Desired{{Path: "internal/service/service.go", Class: manifest.ClassCodemod, Content: []byte("package service\n\ntype Mine interface{}\n")}}
	want(t, f.build(plan.Options{}, same, nil), "internal/service/service.go", plan.OpIdentical)
}

func TestRemovals(t *testing.T) {
	f := newFixture(t)
	f.generated("x/clean.go", manifest.ClassOnce, "v1\n")
	f.generated("x/edited.go", manifest.ClassOnce, "v1\n")
	f.write("x/edited.go", "v2\n")
	f.generated("x/gone.go", manifest.ClassOnce, "v1\n")
	_ = os.Remove(filepath.Join(f.root, "x", "gone.go"))
	f.generated("x/errors.go", manifest.ClassCodemod, "v1\n")

	p := f.build(plan.Options{}, nil, []string{"x/clean.go", "x/edited.go", "x/gone.go"})
	want(t, p, "x/clean.go", plan.OpDelete)
	want(t, p, "x/edited.go", plan.OpSkip)
	want(t, p, "x/gone.go", plan.OpMissing)
	for _, path := range []string{"x/clean.go", "x/edited.go", "x/gone.go"} {
		if _, ok := p.Manifest.Lookup(path); ok {
			t.Fatalf("%s must leave the manifest", path)
		}
	}
	forced := f.build(plan.Options{Force: true}, nil, []string{"x/edited.go"})
	want(t, forced, "x/edited.go", plan.OpDelete)

	if _, err := plan.New(f.root, f.m, plan.Options{}).Build(nil, []string{"x/notours.go"}); err == nil {
		t.Fatal("removing an unknown path must fail")
	}
	if _, err := plan.New(f.root, f.m, plan.Options{}).Build(nil, []string{"x/errors.go"}); err == nil {
		t.Fatal("removing a codemod file must fail")
	}
	if _, err := plan.New(f.root, f.m, plan.Options{}).Build([]plan.Desired{once("x/clean.go", "")}, []string{"x/clean.go"}); err == nil {
		t.Fatal("desired and removed must fail")
	}
	if _, err := plan.New(f.root, f.m, plan.Options{}).Build([]plan.Desired{once("../x.go", "")}, nil); err == nil {
		t.Fatal("escaping path must fail")
	}
}

func TestDiff(t *testing.T) {
	f := newFixture(t)
	f.generated("a.txt", manifest.ClassTool, "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\n")
	p := f.build(plan.Options{}, []plan.Desired{tool("a.txt", "one\ntwo\nthree\nfour\nFIVE\nsix\nseven\neight\nnine\n"), tool("b.txt", "new\n")}, nil)
	d := p.Diff(want(t, p, "a.txt", plan.OpRegen))
	for _, line := range []string{"--- a/a.txt", "+++ b/a.txt", "@@ -2,7 +2,7 @@", "-five", "+FIVE", " four", " eight"} {
		if !strings.Contains(d, line) {
			t.Errorf("missing %q in\n%s", line, d)
		}
	}
	if strings.Contains(d, " one\n") {
		t.Errorf("line outside context included:\n%s", d)
	}
	created := p.Diff(want(t, p, "b.txt", plan.OpCreate))
	if !strings.HasPrefix(created, "--- /dev/null\n+++ b/b.txt\n") || !strings.Contains(created, "+new") {
		t.Errorf("create diff:\n%s", created)
	}
	if plan.Unified("x", []byte("same\n"), []byte("same\n")) != "" {
		t.Error("identical content must produce no diff")
	}
	two := plan.Unified("x", []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\n"), []byte("A\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nL\n"))
	if strings.Count(two, "@@") != 4 {
		t.Errorf("expected two hunks:\n%s", two)
	}
}
