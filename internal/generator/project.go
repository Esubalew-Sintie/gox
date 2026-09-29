package generator

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"github.com/esubalew-Sintie/gox/internal/naming"
	"github.com/esubalew-Sintie/gox/internal/renderer"
)

// ProjectGenerator creates new gox projects from templates.
type ProjectGenerator struct {
	fs       *filesystem.FileSystem
	config   *config.ConfigManager
	renderer *renderer.Renderer
	executor *Executor
}

func NewProjectGenerator(
	fs *filesystem.FileSystem,
	configManager *config.ConfigManager,
	renderer *renderer.Renderer,
) *ProjectGenerator {
	return &ProjectGenerator{
		fs:       fs,
		config:   configManager,
		renderer: renderer,
		executor: NewExecutor(fs),
	}
}

// ProjectOptions carries the user's architectural choices.
type ProjectOptions struct {
	Name   string
	Module string
	Layout string

	Postgres bool
	Redis    bool
	Kafka    bool

	Git bool
}

// projectTemplateData is the data fed to project templates.
type projectTemplateData struct {
	ProjectName string
	Module      string
	Storage     struct {
		Postgres bool
	}
	Features struct {
		Redis bool
		Kafka bool
	}
}

func (g *ProjectGenerator) Generate(options ProjectOptions) error {
	if options.Name == "" {
		return fmt.Errorf("project name is required")
	}

	if options.Module == "" {
		return fmt.Errorf("module path is required")
	}

	if options.Layout == "" {
		options.Layout = "clean-rest"
	}

	name, err := naming.Parse(options.Name)
	if err != nil {
		return fmt.Errorf("parse project name: %w", err)
	}

	projectPath := filepath.Clean(name.Kebab)

	if g.fs.Exists(projectPath) {
		return fmt.Errorf("project %q already exists", options.Name)
	}

	data := projectTemplateData{
		ProjectName: name.Kebab,
		Module:      options.Module,
	}
	data.Storage.Postgres = options.Postgres
	data.Features.Redis = options.Redis
	data.Features.Kafka = options.Kafka

	plan := NewGenerationPlan()

	// go.mod
	goMod, err := g.renderer.Render("go.mod.tmpl", data)
	if err != nil {
		return fmt.Errorf("render go.mod: %w", err)
	}
	if err := plan.AddCreate("go.mod", goMod); err != nil {
		return err
	}

	// cmd/server/main.go
	mainGo, err := g.renderer.Render("main.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("render main.go: %w", err)
	}
	if err := plan.AddCreate(filepath.Join("cmd", "server", "main.go"), mainGo); err != nil {
		return err
	}

	// initiator/initiator.go
	initiatorGo, err := g.renderer.Render("initiator.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("render initiator.go: %w", err)
	}
	if err := plan.AddCreate(filepath.Join("initiator", "initiator.go"), initiatorGo); err != nil {
		return err
	}

	// initiator/modules.go
	modulesGo, err := g.renderer.Render("modules.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("render modules.go: %w", err)
	}
	if err := plan.AddCreate(filepath.Join("initiator", "modules.go"), modulesGo); err != nil {
		return err
	}

	// initiator/routes.go
	routesGo, err := g.renderer.Render("routes.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("render routes.go: %w", err)
	}
	if err := plan.AddCreate(filepath.Join("initiator", "routes.go"), routesGo); err != nil {
		return err
	}

	// initiator/database.go (only when postgres)
	if options.Postgres {
		dbGo, err := g.renderer.Render("database.go.tmpl", data)
		if err != nil {
			return fmt.Errorf("render database.go: %w", err)
		}
		if err := plan.AddCreate(filepath.Join("initiator", "database.go"), dbGo); err != nil {
			return err
		}
	}

	// .gox.yaml
	goxYaml, err := g.renderer.Render("gox.yaml.tmpl", data)
	if err != nil {
		return fmt.Errorf("render .gox.yaml: %w", err)
	}
	if err := plan.AddCreate(".gox.yaml", goxYaml); err != nil {
		return err
	}

	// Create root directory and apply plan
	if err := g.fs.CreateDir(projectPath); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}

	if err := g.executor.Execute(projectPath, plan); err != nil {
		_ = g.fs.Remove(projectPath)
		return fmt.Errorf("write project files: %w", err)
	}

	// Optional: git init
	if options.Git {
		if err := runCommand(projectPath, "git", "init"); err != nil {
			// non-fatal: git may not be installed
			fmt.Printf("  ⚠ git init failed: %v\n", err)
		}
	}

	// go mod tidy (needs network; best-effort)
	if err := runCommand(projectPath, "go", "mod", "tidy"); err != nil {
		fmt.Printf("  ⚠ go mod tidy failed: %v\n", err)
	}

	return nil
}

func runCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w\noutput: %s", name, args, err, out)
	}
	return nil
}