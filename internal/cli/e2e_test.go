package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/esubalew-Sintie/gox/internal/cli"
)

// gox runs a command without prompts; goxIn answers them from input.
func gox(t *testing.T, args ...string) string {
	t.Helper()
	root := cli.NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append(args, "--no-interactive"))
	if err := root.Execute(); err != nil {
		t.Fatalf("gox %s: %v\n%s", strings.Join(args, " "), err, out.String())
	}
	return out.String()
}

func goxIn(t *testing.T, input string, args ...string) string {
	t.Helper()
	root := cli.NewRoot()
	var out bytes.Buffer
	root.SetIn(strings.NewReader(input))
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("gox %s: %v\n%s", strings.Join(args, " "), err, out.String())
	}
	return out.String()
}

func goxFails(t *testing.T, args ...string) string {
	t.Helper()
	root := cli.NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append(args, "--no-interactive"))
	err := root.Execute()
	if err == nil {
		t.Fatalf("gox %s: expected an error\n%s", strings.Join(args, " "), out.String())
	}
	return err.Error()
}

func hashTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func treeDiff(before, after map[string]string) string {
	var lines []string
	for p, h := range before {
		switch got, ok := after[p]; {
		case !ok:
			lines = append(lines, "missing: "+p)
		case got != h:
			lines = append(lines, "changed: "+p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			lines = append(lines, "extra: "+p)
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func goRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// dockerAvailable reports whether testcontainers can run.
func dockerAvailable() bool {
	return exec.Command("docker", "info").Run() == nil
}

func needsDocker(database string) bool {
	return database == "postgres" || database == "mysql" || database == "mongo"
}

// TestEndToEnd is the exit criterion of phases 2 and 3: for every database,
// new, gen feature, the generated project's own tests pass (repository tests
// against a real database when Docker is available), and destroy returns
// the tree to the state after new, interface files and catalogue included.
func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("needs network for go mod tidy")
	}
	docker := dockerAvailable()
	if !docker {
		t.Log("Docker is not available; repository tests of the container databases run with -short")
	}
	for _, database := range []string{"memory", "sqlite", "postgres", "mysql", "mongo"} {
		t.Run(database, func(t *testing.T) { runEndToEnd(t, database, "chi", docker) })
	}
	t.Run("stdlib", func(t *testing.T) { runEndToEnd(t, "memory", "stdlib", docker) })
}

func runEndToEnd(t *testing.T, database, router string, docker bool) {
	parent := t.TempDir()
	out := gox(t, "new", "fleet-service", "--module", "example.com/fleet-service", "--db", database, "--http", router, "-C", parent)
	root := filepath.Join(parent, "fleet-service")
	if !strings.Contains(out, "create  app/app.go") || !strings.Contains(out, "verified with go build and go vet") {
		t.Fatalf("new output:\n%s", out)
	}
	before := hashTree(t, root)
	if _, ok := before[".gox/manifest.json"]; !ok {
		t.Fatal("manifest not written")
	}

	feature := []string{"gen", "feature", "fleet", "name:string", "capacity:int", "opened_at:time:null", "-C", root}
	gox(t, append(feature, "--dry-run")...)
	if d := treeDiff(before, hashTree(t, root)); d != "" {
		t.Fatalf("dry run changed the tree:\n%s", d)
	}
	out = gox(t, feature...)
	for _, want := range []string{"create  internal/domain/entity/fleet.go", "create  internal/domain/dto/fleet.go", "modify  internal/domain/errors.go", "modify  internal/storage/repository.go", "regen  app/routes.go"} {
		if !strings.Contains(out, want) {
			t.Fatalf("gen output missing %q:\n%s", want, out)
		}
	}
	if needsDocker(database) || database == "sqlite" {
		if !strings.Contains(out, "create  internal/storage/"+database+"/fleet/repository.go") {
			t.Fatalf("gen output missing the %s repository:\n%s", database, out)
		}
	}
	if database != "memory" && database != "mongo" && !strings.Contains(out, "create  migrations/000002_create_fleets.up.sql") {
		t.Fatalf("gen output missing the migration:\n%s", out)
	}

	args := []string{"test", "./..."}
	if needsDocker(database) && !docker {
		args = append(args, "-short")
	}
	goRun(t, root, args...)

	if database == "memory" && router == "chi" {
		checkSyncRewires(t, root)
	}

	if msg := goxFails(t, "gen", "feature", "fleet", "-C", root); !strings.Contains(msg, "already exists") {
		t.Fatalf("second gen: %s", msg)
	}
	if msg := goxFails(t, "gen", "feature", "vehicle", "id:int", "-C", root); !strings.Contains(msg, "generated on every entity") {
		t.Fatalf("reserved field: %s", msg)
	}

	manifestBefore, _ := os.ReadFile(filepath.Join(root, ".gox", "manifest.json"))
	gox(t, "destroy", "feature", "fleet", "-C", root, "--yes")
	if d := treeDiff(before, hashTree(t, root)); d != "" {
		manifestAfter, _ := os.ReadFile(filepath.Join(root, ".gox", "manifest.json"))
		t.Fatalf("destroy did not restore the tree:\n%s\n--- manifest before ---\n%s\n--- manifest after ---\n%s", d, manifestBefore, manifestAfter)
	}
	goRun(t, root, "build", "./...")

	if database == "memory" || database == "mongo" {
		if msg := goxFails(t, "gen", "migration", "add_index", "-C", root); !strings.Contains(msg, "SQL databases") {
			t.Fatalf("gen migration on %s: %s", database, msg)
		}
		return
	}
	gox(t, "gen", "migration", "add_fleet_index", "-C", root)
	for _, f := range []string{"migrations/000002_add_fleet_index.up.sql", "migrations/000002_add_fleet_index.down.sql"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Fatalf("gen migration: %s missing", f)
		}
	}
}

// checkSyncRewires is the exit criterion of phase 4: a constructor edited
// by the developer is re-wired by gox sync, and gox check catches the drift
// in between. It also exercises doctor and the templates commands.
func checkSyncRewires(t *testing.T, root string) {
	t.Helper()
	gox(t, "check", "-C", root)
	gox(t, "doctor", "-C", root)

	// The developer gives the fleet service a configuration dependency and,
	// as they would have to, updates the two tests that call the constructor.
	edits := map[string][2]string{
		"internal/service/fleet/service.go": {
			"func New(repo storage.FleetRepository, log *slog.Logger) *Service {",
			"func New(repo storage.FleetRepository, server config.ServerConfig, log *slog.Logger) *Service {",
		},
		"internal/service/fleet/service_test.go": {
			"fleetsvc.New(memstore.NewFleetStore(), slog.Default())",
			"fleetsvc.New(memstore.NewFleetStore(), config.Default().Server, slog.Default())",
		},
		"internal/handler/http/fleet/handler_test.go": {
			"fleetsvc.New(memstore.NewFleetStore(), slog.Default())",
			"fleetsvc.New(memstore.NewFleetStore(), config.Default().Server, slog.Default())",
		},
	}
	originals := map[string][]byte{}
	for rel, pair := range edits {
		full := filepath.Join(root, filepath.FromSlash(rel))
		src, err := os.ReadFile(full)
		if err != nil {
			t.Fatal(err)
		}
		originals[rel] = src
		edited := strings.Replace(string(src), pair[0], pair[1], 1)
		edited = strings.Replace(edited, "import (\n", "import (\n\t\"example.com/fleet-service/internal/config\"\n", 1)
		if edited == string(src) {
			t.Fatalf("could not edit %s:\n%s", rel, src)
		}
		if err := os.WriteFile(full, []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if msg := goxFails(t, "check", "-C", root); !strings.Contains(msg, "drifted") {
		t.Fatalf("check must report drift after a constructor change: %s", msg)
	}
	out := gox(t, "sync", "-C", root)
	if !strings.Contains(out, "regen  app/service.go") {
		t.Fatalf("sync must regenerate app/service.go:\n%s", out)
	}
	appService, _ := os.ReadFile(filepath.Join(root, "app", "service.go"))
	if !strings.Contains(string(appService), "fleetsvc.New(s.Fleet, cfg.Server, log)") {
		t.Fatalf("sync did not wire the new parameter:\n%s", appService)
	}
	goRun(t, root, "test", "./...")
	gox(t, "check", "-C", root)
	gox(t, "doctor", "-C", root)

	// A second feature regenerates the root through the same resolution.
	// The name is given, the fields are answered one per line.
	driver := goxIn(t, "name:string\nlicense:string:null\n\n", "gen", "feature", "driver", "-C", root)
	if !strings.Contains(driver, "feature driver: name:string license:string:null") || !strings.Contains(driver, "create  internal/domain/entity/driver.go") {
		t.Fatalf("interactive gen feature:\n%s", driver)
	}
	appService, _ = os.ReadFile(filepath.Join(root, "app", "service.go"))
	if !strings.Contains(string(appService), "fleetsvc.New(s.Fleet, cfg.Server, log)") || !strings.Contains(string(appService), "driversvc.New(s.Driver, log)") {
		t.Fatalf("gen feature lost the resolved wiring:\n%s", appService)
	}
	goRun(t, root, "build", "./...")
	if aborted := goxIn(t, "n\n", "destroy", "feature", "driver", "-C", root); !strings.Contains(aborted, "aborted") {
		t.Fatalf("destroy answered no:\n%s", aborted)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "domain", "entity", "driver.go")); err != nil {
		t.Fatalf("destroy answered no still removed files: %v", err)
	}
	goxIn(t, "y\n", "destroy", "feature", "driver", "-C", root)

	// Put the files back so the destroy round trip below compares equal.
	for rel, src := range originals {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gox(t, "sync", "-C", root)

	// Templates: list, eject, diff.
	list := gox(t, "templates", "list", "-C", root)
	if !strings.Contains(list, "project/README.md.tmpl") {
		t.Fatalf("templates list:\n%s", list)
	}
	gox(t, "templates", "eject", "project/README.md.tmpl", "-C", root)
	override := filepath.Join(root, ".gox", "templates", "project", "README.md.tmpl")
	if err := os.WriteFile(override, []byte("# custom readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := gox(t, "templates", "diff", "-C", root); !strings.Contains(d, "+# custom readme") {
		t.Fatalf("templates diff:\n%s", d)
	}
	if err := os.RemoveAll(filepath.Join(root, ".gox", "templates")); err != nil {
		t.Fatal(err)
	}
}

func newRootWithInput(input string) *cobra.Command {
	root := cli.NewRoot()
	root.SetIn(strings.NewReader(input))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root
}
