package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/esubalew-Sintie/gox/internal/apply"
	"github.com/esubalew-Sintie/gox/internal/db"
	"github.com/esubalew-Sintie/gox/internal/generate"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/model"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/version"
)

func genCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gen",
		Aliases: []string{"g", "generate"},
		Short:   "Generate code into the project",
	}
	cmd.AddCommand(genFeatureCmd(g), genMigrationCmd(g))
	return cmd
}

func genMigrationCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "migration <name>",
		Short: "Add an empty golang-migrate pair with the next version number",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadProject(g.dir)
			if err != nil {
				return err
			}
			if d, ok := db.For(p.model.DB); !ok || !d.SQL {
				return fmt.Errorf("migrations are for SQL databases; this project uses %s", p.model.DB)
			}
			next, err := generate.NextMigration(p.root)
			if err != nil {
				return err
			}
			gen := generate.New(p.root, p.model, p.manifest, p.store)
			desired, err := gen.Migration(next, args[0])
			if err != nil {
				return err
			}
			pl, err := plan.New(p.root, p.manifest, plan.Options{Version: version.Version}).Build(desired, nil)
			if err != nil {
				return err
			}
			return g.run(cmd, p.root, pl, apply.Options{AllowDirty: g.allowDirty, SkipVerify: true}, "")
		},
	}
}

func genFeatureCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "feature <Name> [field:type ...]",
		Aliases: []string{"f", "module", "resource"},
		Short:   "Add a feature with every layer and its wiring",
		Long: `Add a feature: entity, service, repository, HTTP handler with DTOs, in-memory
store, tests, its errors in the catalogue, its interfaces, and the regenerated
composition root. Fields use name:type with types string, text, int, int64,
float64, bool, time and uuid; a trailing ? or :null makes a field nullable
(zsh needs the ? quoted, so :null is the shell-safe form).

  gox gen feature fleet name:string capacity:int opened_at:time:null`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenFeature(cmd, g, args[0], args[1:])
		},
	}
}

func runGenFeature(cmd *cobra.Command, g *globalFlags, name string, fields []string) error {
	p, err := loadProject(g.dir)
	if err != nil {
		return err
	}
	fs := manifest.FeatureSpec{Name: name, Fields: fields, Transports: []string{"http"}}
	if _, exists := p.spec.Feature(name); exists {
		return fmt.Errorf("feature %q already exists; destroy it first or choose another name", name)
	}
	if _, err := model.FeatureFromSpec(fs); err != nil {
		return err
	}

	newSpec := p.spec.Clone()
	if err := newSpec.AddFeature(fs); err != nil {
		return err
	}
	newModel, err := model.FromSpec(newSpec)
	if err != nil {
		return err
	}
	f, _ := newModel.Feature(name)

	gen := generate.New(p.root, newModel, p.manifest, p.store)
	desired, err := gen.Feature(f)
	if err != nil {
		return err
	}
	wiring := p.wiring(gen, g.writer(cmd))
	tools, err := gen.Tool(wiring)
	if err != nil {
		return err
	}
	desired = append(desired, tools...)

	pl, err := plan.New(p.root, p.manifest, plan.Options{Force: g.force, Adopt: g.adopt, Version: version.Version}).Build(desired, nil)
	if err != nil {
		return err
	}
	pl.Spec = newSpec
	pl.Manifest.Wiring = wiring
	return g.run(cmd, p.root, pl, apply.Options{AllowDirty: g.allowDirty}, "")
}
