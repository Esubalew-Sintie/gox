package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/esubalew-Sintie/gox/internal/apply"
	"github.com/esubalew-Sintie/gox/internal/generate"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/model"
	"github.com/esubalew-Sintie/gox/internal/naming"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/templates"
	"github.com/esubalew-Sintie/gox/internal/version"
)

func newCmd(g *globalFlags) *cobra.Command {
	var (
		module     string
		db         string
		httpRouter string
		with       []string
		gitInit    bool
	)
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a project",
		Long:  "Create a Go service in <name>/ with the hexagonal layout, the composition root, the error and response packages, tests, Makefile, Dockerfile and lint configuration. The module path defaults to the name; pass --module for a real one.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, g, newOptions{name: args[0], module: module, db: db, http: httpRouter, with: with, git: gitInit})
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "Go module path, for example github.com/acme/fleet (default: the name)")
	cmd.Flags().StringVar(&db, "db", "memory", "database: "+strings.Join(manifest.Databases, ", "))
	cmd.Flags().StringVar(&httpRouter, "http", "chi", "router: "+strings.Join(manifest.Routers, ", "))
	cmd.Flags().StringSliceVar(&with, "with", nil, "capabilities: "+strings.Join(manifest.Capabilities, ", "))
	cmd.Flags().BoolVar(&gitInit, "git", false, "run git init in the new project")
	return cmd
}

type newOptions struct {
	name   string
	module string
	db     string
	http   string
	with   []string
	git    bool
}

func runNew(cmd *cobra.Command, g *globalFlags, o newOptions) error {
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
	out := g.writer(cmd)
	if o.module == "" {
		o.module = name.Kebab
		out.Warn("no --module given; using %q as the module path", o.module)
	}

	spec := &manifest.Spec{Version: manifest.SpecVersion, Module: o.module, Arch: "hexagonal", HTTP: o.http, DB: o.db, Capabilities: o.with}
	if err := spec.Validate(); err != nil {
		return err
	}
	if len(spec.Capabilities) > 0 {
		return fmt.Errorf("--with is not available in gox %s; capabilities arrive in 0.4", version.Version)
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

	out.Note("creating %s", root)
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
