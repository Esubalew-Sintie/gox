package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/esubalew-Sintie/gox/internal/fsx"
	"github.com/esubalew-Sintie/gox/internal/generate"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/resolve"
	"github.com/esubalew-Sintie/gox/internal/transcript"
	"github.com/esubalew-Sintie/gox/internal/version"
)

func doctorCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the toolchain, the project files, the wiring, and that the project builds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctor(cmd, g)
		},
	}
}

func runDoctor(cmd *cobra.Command, g *globalFlags) error {
	out := g.writer(cmd)
	failures := 0
	ok := func(format string, args ...any) { out.Step("ok", fmt.Sprintf(format, args...), "") }
	warn := func(format string, args ...any) { out.Step("warn", fmt.Sprintf(format, args...), "") }
	fail := func(format string, args ...any) { out.Step("fail", fmt.Sprintf(format, args...), ""); failures++ }

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	out.Note("gox %s", version.Version)
	if v, err := output(ctx, g.dir, "go", "version"); err != nil {
		fail("go toolchain: %v", err)
	} else {
		ok("%s", v)
	}
	if _, err := output(ctx, g.dir, "git", "--version"); err != nil {
		warn("git not found; the clean-tree check and --git are unavailable")
	}

	p, err := loadProject(g.dir)
	if err != nil {
		fail("%v", err)
		out.Error("%d problem(s)", failures)
		return errors.New("doctor found problems")
	}
	ok("gox.yaml: module %s, db %s, http %s, %d feature(s)", p.model.Module, p.model.DB, p.model.HTTP, len(p.model.Features))
	ok("manifest: %d file(s) recorded by gox %s", len(p.manifest.Files), p.manifest.Version)

	edited, missing := 0, 0
	for _, e := range p.manifest.Files {
		full := filepath.Join(p.root, filepath.FromSlash(e.Path))
		data, exists, err := fsx.ReadIfExists(full)
		if err != nil {
			return err
		}
		if !exists {
			missing++
			if e.Class == manifest.ClassTool {
				fail("missing tool file %s; run gox sync", e.Path)
			} else {
				warn("%s was removed", e.Path)
			}
			continue
		}
		if manifest.Hash(data) != e.Hash && e.Class == manifest.ClassTool {
			fail("tool file %s was edited by hand; gox sync --adopt keeps it as yours", e.Path)
		} else if manifest.Hash(data) != e.Hash {
			edited++
		}
	}
	ok("%d once file(s) edited by you, %d file(s) removed", edited, missing)

	if ids, err := p.store.Overridden(); err == nil && len(ids) > 0 {
		warn("%d template override(s) in .gox/templates: %s", len(ids), strings.Join(ids, ", "))
	} else {
		ok("no template overrides")
	}

	gen := generate.New(p.root, p.model, p.manifest, p.store)
	u, err := resolve.Load(p.root)
	if err != nil {
		fail("load project: %v", err)
	} else {
		for _, c := range gen.Constructors() {
			if !u.HasPackage(c.Package) {
				fail("%s: package %s is missing", resolve.Key(c.Feature, c.Layer), c.Package)
				continue
			}
			args, err := u.Resolve(c, gen.Providers(u, c.Layer))
			if err != nil {
				fail("%s: %v", resolve.Key(c.Feature, c.Layer), err)
				continue
			}
			ok("%s: %s.%s(%s)", resolve.Key(c.Feature, c.Layer), filepath.Base(c.Package), c.Func, strings.Join(args, ", "))
		}
	}

	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}} {
		if o, err := output(ctx, p.root, "go", args...); err != nil {
			fail("go %s failed:\n%s", strings.Join(args, " "), o)
		} else {
			ok("go %s", strings.Join(args, " "))
		}
	}

	if failures > 0 {
		out.Error("%d problem(s)", failures)
		return errors.New("doctor found problems")
	}
	out.Note("no problems found")
	return nil
}

func output(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	o, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(o)), err
}

var _ = transcript.New
