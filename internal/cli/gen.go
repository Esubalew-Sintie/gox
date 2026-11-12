package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/esubalew-Sintie/gox/internal/apply"
	"github.com/esubalew-Sintie/gox/internal/db"
	"github.com/esubalew-Sintie/gox/internal/generate"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/model"
	"github.com/esubalew-Sintie/gox/internal/naming"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/prompt"
	"github.com/esubalew-Sintie/gox/internal/transcript"
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
		Use:     "feature [Name] [field:type ...]",
		Aliases: []string{"f", "module", "resource"},
		Short:   "Add a feature with every layer and its wiring",
		Long: `Add a feature: entity, service, repository, HTTP handler with DTOs, in-memory
store, tests, its errors in the catalogue, its interfaces, and the regenerated
composition root. Fields use name:type with types string, text, int, int64,
float64, bool, time and uuid; a trailing ? or :null makes a field nullable
(zsh needs the ? quoted, so :null is the shell-safe form).

  gox gen feature fleet name:string capacity:int opened_at:time:null

On a terminal gox asks for the name and the fields when they are not given,
one field per line until an empty line.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			var fields []string
			if len(args) > 0 {
				name, fields = args[0], args[1:]
			}
			return runGenFeature(cmd, g, name, fields)
		},
	}
}

func runGenFeature(cmd *cobra.Command, g *globalFlags, name string, fields []string) error {
	p, err := loadProject(g.dir)
	if err != nil {
		return err
	}
	out := g.writer(cmd)
	name, fields, err = askFeature(g.prompter(cmd), out, p, name, fields)
	if err != nil {
		return err
	}
	if name == "" {
		return errors.New("a feature name is needed: gox gen feature <Name> [field:type ...]")
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
	wiring := p.wiring(gen, out)
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

// askFeature asks for the name when it is missing and for the fields when
// none were given, one per question until an empty answer.
func askFeature(pr *prompt.Prompter, out *transcript.Writer, p *project, name string, fields []string) (string, []string, error) {
	if !pr.Asks() {
		return name, fields, nil
	}
	if name == "" {
		err := pr.Input(&name, "Feature name", "Singular, for example fleet or vehicle_type.", "fleet", func(s string) error {
			if _, err := naming.Parse(s); err != nil {
				return err
			}
			if _, exists := p.spec.Feature(s); exists {
				return fmt.Errorf("feature %q already exists", s)
			}
			return nil
		})
		if err != nil {
			return "", nil, err
		}
	}
	if len(fields) > 0 {
		return name, fields, nil
	}
	seen := map[string]bool{}
	for {
		title := fmt.Sprintf("Field %d (name:type, empty line to finish)", len(fields)+1)
		description := "Types: " + joinFieldTypes() + ". Add ? or :null for a nullable field."
		if len(fields) > 0 {
			description = "So far: " + strings.Join(fields, " ") + ". " + description
		}
		var field string
		err := pr.Input(&field, title, description, "", func(s string) error {
			s = strings.TrimSpace(s)
			if s == "" {
				return nil
			}
			f, err := model.ParseField(s)
			if err != nil {
				return err
			}
			if seen[f.Name.Snake] {
				return fmt.Errorf("field %q is declared twice", f.Name.Snake)
			}
			return nil
		})
		if err != nil {
			return "", nil, err
		}
		field = strings.TrimSpace(field)
		if field == "" {
			break
		}
		f, _ := model.ParseField(field)
		seen[f.Name.Snake] = true
		fields = append(fields, field)
	}
	out.Note("feature %s: %s", name, strings.Join(fields, " "))
	return name, fields, nil
}

func joinFieldTypes() string {
	names := make([]string, len(model.FieldTypes))
	for i, t := range model.FieldTypes {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}
