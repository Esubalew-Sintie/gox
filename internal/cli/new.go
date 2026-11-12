package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/module"

	"github.com/esubalew-Sintie/gox/internal/apply"
	"github.com/esubalew-Sintie/gox/internal/generate"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/model"
	"github.com/esubalew-Sintie/gox/internal/naming"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/prompt"
	"github.com/esubalew-Sintie/gox/internal/templates"
	"github.com/esubalew-Sintie/gox/internal/version"
)

func newCmd(g *globalFlags) *cobra.Command {
	var o newOptions
	cmd := &cobra.Command{
		Use:   "new [name]",
		Short: "Create a project",
		Long: `Create a Go service in <name>/ with the hexagonal layout, the composition
root, the error and response packages, tests, Makefile, Dockerfile and lint
configuration.

On a terminal gox asks for every choice that is not given as a flag: the
name, the module path, the database, the router and git init. --defaults,
--no-interactive or an input that is not a terminal take the defaults
instead; the module path then defaults to the name. ACCESSIBLE=1 asks the
same questions as plain lines.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				o.name = args[0]
			}
			o.given = cmd.Flags().Changed
			return runNew(cmd, g, o)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.module, "module", "", "Go module path, for example github.com/acme/fleet (default: the name)")
	f.StringVar(&o.db, "db", "memory", "database: "+strings.Join(manifest.Databases, ", "))
	f.StringVar(&o.http, "http", "chi", "router: "+strings.Join(manifest.Routers, ", "))
	f.StringSliceVar(&o.with, "with", nil, "capabilities: "+strings.Join(manifest.Capabilities, ", "))
	f.BoolVar(&o.git, "git", false, "run git init in the new project")
	f.BoolVar(&o.defaults, "defaults", false, "take the default for every choice not given as a flag instead of asking")
	return cmd
}

type newOptions struct {
	name     string
	module   string
	db       string
	http     string
	with     []string
	git      bool
	defaults bool
	given    func(flag string) bool // whether a flag was set on the command line
}

var databaseDescriptions = map[string]string{
	"memory":   "in-memory stores, no infrastructure",
	"postgres": "pgx pool, migrations, repository tests with testcontainers",
	"mysql":    "database/sql, migrations, repository tests with testcontainers",
	"sqlite":   "database/sql, migrations, file or in-memory database",
	"mongo":    "official driver v2, optimistic updates",
}

var routerDescriptions = map[string]string{
	"chi":    "go-chi router with its middleware",
	"stdlib": "net/http.ServeMux with method patterns",
}

func choices(values []string, descriptions map[string]string) []prompt.Option {
	opts := make([]prompt.Option, len(values))
	for i, v := range values {
		opts[i] = prompt.Option{Value: v, Description: descriptions[v]}
	}
	return opts
}

// askNew asks, in one form, for the choices the flags left open.
func askNew(pr *prompt.Prompter, o *newOptions) error {
	if !pr.Asks() {
		return nil
	}
	form := pr.Form()
	if o.name == "" {
		form.Input(&o.name, "Project name", "The directory to create, also the default module path.", "fleet-service", func(s string) error {
			_, err := naming.Parse(s)
			return err
		})
	}
	if !o.given("module") {
		form.Input(&o.module, "Module path", "Empty keeps the project name.", "github.com/acme/fleet-service", func(s string) error {
			if s == "" {
				return nil
			}
			return module.CheckImportPath(s)
		})
	}
	if !o.given("db") {
		form.Select(&o.db, "Database", "", choices(manifest.Databases, databaseDescriptions))
	}
	if !o.given("http") {
		form.Select(&o.http, "Router", "", choices(manifest.Routers, routerDescriptions))
	}
	if !o.given("git") {
		form.Confirm(&o.git, "Run git init in the new project?", "")
	}
	return form.Run()
}

func runNew(cmd *cobra.Command, g *globalFlags, o newOptions) error {
	out := g.writer(cmd)
	if o.given == nil {
		o.given = func(string) bool { return false }
	}
	pr := g.prompter(cmd)
	if o.defaults {
		pr = prompt.WithMode(prompt.None, cmd.InOrStdin(), cmd.OutOrStdout())
	}
	if err := askNew(pr, &o); err != nil {
		return err
	}
	if o.name == "" {
		return errors.New("a project name is needed: gox new <name>")
	}
	name, err := naming.Parse(o.name)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(filepath.Join(g.dir, name.Kebab))
	if err != nil {
		return err
	}
	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s exists and is not empty", root)
	}
	if o.module == "" {
		o.module = name.Kebab
		if !pr.Asks() {
			out.Warn("no --module given; using %q as the module path", o.module)
		}
	}

	spec := &manifest.Spec{Version: manifest.SpecVersion, Module: o.module, Arch: "hexagonal", HTTP: o.http, DB: o.db, Capabilities: o.with}
	if err := spec.Validate(); err != nil {
		return err
	}
	if len(spec.Capabilities) > 0 {
		return fmt.Errorf("--with is not available in gox %s; capabilities arrive in a later release", version.Version)
	}

	proj, err := model.FromSpec(spec)
	if err != nil {
		return err
	}
	store, err := templates.New(root)
	if err != nil {
		return err
	}
	m := manifest.New(version.Version)
	gen := generate.New(root, proj, m, store)
	desired, err := gen.Project()
	if err != nil {
		return err
	}
	tools, err := gen.Tool(nil)
	if err != nil {
		return err
	}
	desired = append(desired, tools...)

	p, err := plan.New(root, m, plan.Options{Version: version.Version}).Build(desired, nil)
	if err != nil {
		return err
	}
	p.Spec = spec

	out.Note("creating %s: module %s, db %s, http %s", root, spec.Module, spec.DB, spec.HTTP)
	if err := g.run(cmd, root, p, apply.Options{AllowDirty: true, Tidy: true}, ""); err != nil {
		return err
	}
	if g.dryRun {
		return nil
	}
	if o.git {
		git := exec.CommandContext(cmd.Context(), "git", "init", "-q")
		git.Dir = root
		if outp, err := git.CombinedOutput(); err != nil {
			out.Warn("git init failed: %v\n%s", err, outp)
		}
	}
	out.Note("")
	out.Note("next steps:")
	out.Note("  cd %s", name.Kebab)
	out.Note("  make run")
	out.Note("  gox gen feature fleet name:string capacity:int")
	return nil
}
