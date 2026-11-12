package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/esubalew-Sintie/gox/internal/apply"
	"github.com/esubalew-Sintie/gox/internal/codemod"
	"github.com/esubalew-Sintie/gox/internal/fsx"
	"github.com/esubalew-Sintie/gox/internal/generate"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/version"
)

func syncCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Re-wire the composition root from the constructors as they are now",
		Long: `Read every constructor the composition root calls, pick an argument for each
parameter by type from what the Init function has in scope, and regenerate
app/infra.go, external_api.go, cache.go, storage.go, service.go, handler.go and
routes.go. Run it after changing a New signature; gen and destroy run it too.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := loadProject(g.dir)
			if err != nil {
				return err
			}
			gen := generate.New(p.root, p.model, p.manifest, p.store)
			wiring := p.wiring(gen, g.writer(cmd))
			desired, err := gen.Tool(wiring)
			if err != nil {
				return err
			}
			pl, err := plan.New(p.root, p.manifest, plan.Options{Force: g.force, Adopt: g.adopt, Version: version.Version}).Build(desired, nil)
			if err != nil {
				return err
			}
			pl.Manifest.Wiring = wiring
			return g.run(cmd, p.root, pl, apply.Options{AllowDirty: g.allowDirty}, "")
		},
	}
}

// ErrDrift is returned by check when the project does not match what gox
// would generate.
var ErrDrift = errors.New("the project has drifted from gox.yaml; run gox sync")

func checkCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Report tool files and declarations that differ from what gox would generate",
		Long: `Regenerate the composition root in memory and compare it with the disk; check
that every declaration gox owns in the codemod files is still there. Nothing is
written. Exit status 1 on drift, for CI.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := loadProject(g.dir)
			if err != nil {
				return err
			}
			out := g.writer(cmd)
			gen := generate.New(p.root, p.model, p.manifest, p.store)
			wiring := p.wiring(gen, out)
			desired, err := gen.Tool(wiring)
			if err != nil {
				return err
			}
			pl, err := plan.New(p.root, p.manifest, plan.Options{Version: version.Version}).Build(desired, nil)
			if err != nil {
				return err
			}
			problems := 0
			for _, s := range pl.Steps {
				switch s.Op {
				case plan.OpIdentical:
					out.Step("ok", s.Path, "")
				case plan.OpRegen:
					out.Step("stale", s.Path, "differs from what gox generates; run gox sync")
					problems++
				case plan.OpConflict:
					out.Step("edited", s.Path, "tool file changed by hand; gox sync --adopt keeps it, --force regenerates it")
					problems++
				case plan.OpCreate:
					out.Step("missing", s.Path, "run gox sync")
					problems++
				case plan.OpKeep, plan.OpModify, plan.OpAdopt, plan.OpDelete, plan.OpMissing, plan.OpSkip:
					out.Step(string(s.Op), s.Path, s.Reason)
				}
				if g.verbose && s.Op == plan.OpRegen {
					out.Raw(pl.Diff(s))
				}
			}
			for _, e := range p.manifest.Files {
				if e.Class != manifest.ClassCodemod {
					continue
				}
				src, exists, err := fsx.ReadIfExists(filepath.Join(p.root, filepath.FromSlash(e.Path)))
				if err != nil {
					return err
				}
				if !exists {
					out.Step("missing", e.Path, "codemod file removed")
					problems++
					continue
				}
				f, err := codemod.Parse(e.Path, src)
				if err != nil {
					out.Step("broken", e.Path, err.Error())
					problems++
					continue
				}
				for _, name := range e.Decls {
					if !f.HasDecl(name) {
						out.Step("missing", e.Path, fmt.Sprintf("declaration %s that gox wrote is gone", name))
						problems++
					}
				}
			}
			if problems > 0 {
				out.Error("%d problem(s)", problems)
				return ErrDrift
			}
			out.Note("up to date: %d tool file(s) match, every generated declaration is present", len(pl.Steps))
			return nil
		},
	}
}
