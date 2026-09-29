package generator

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/esubalew-Sintie/gox/internal/architecture"
	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"github.com/esubalew-Sintie/gox/internal/naming"
	"github.com/esubalew-Sintie/gox/internal/renderer"
)

type ProjectGenerator struct {
	fs       *filesystem.FileSystem
	config   *config.ConfigManager
	renderer *renderer.Renderer
	executor *Executor
}

func NewProjectGenerator(fs *filesystem.FileSystem, configManager *config.ConfigManager, renderer *renderer.Renderer) *ProjectGenerator {
	return &ProjectGenerator{fs: fs, config: configManager, renderer: renderer, executor: NewExecutor(fs)}
}

type ProjectOptions struct {
	Name     string
	Module   string
	Layout   string
	HTTP     string
	Postgres bool
	Mongo    bool
	Redis    bool
	Kafka    bool
	Git      bool
}

type projectTemplateData struct {
	ProjectName string
	Module      string
	Layout      string
	HTTP        string
	Storage     struct {
		Postgres bool
		Mongo    bool
	}
	Features struct {
		Redis bool
		Kafka bool
	}
}

type projectFile struct {
	template string
	path     string
}

func (g *ProjectGenerator) Generate(options ProjectOptions) error {
	if options.Name == "" {
		return fmt.Errorf("project name is required")
	}
	if options.Module == "" {
		return fmt.Errorf("module path is required")
	}
	requestedLayout := options.Layout
	if requestedLayout == "" {
		requestedLayout = architecture.LayeredREST
	}
	options.Layout = architecture.Normalize(requestedLayout)
	if options.Layout == "" {
		return fmt.Errorf("unsupported layout: %s", requestedLayout)
	}
	if options.HTTP == "" {
		options.HTTP = "net/http"
	}
	if options.HTTP != "net/http" && options.HTTP != "chi" && options.HTTP != "gin" {
		return fmt.Errorf("unsupported http router: %s", options.HTTP)
	}

	name, err := naming.Parse(options.Name)
	if err != nil {
		return fmt.Errorf("parse project name: %w", err)
	}
	projectPath := filepath.Clean(name.Kebab)
	if g.fs.Exists(projectPath) {
		return fmt.Errorf("project %q already exists", options.Name)
	}

	data := projectTemplateData{ProjectName: name.Kebab, Module: options.Module, Layout: options.Layout, HTTP: options.HTTP}
	data.Storage.Postgres = options.Postgres
	data.Storage.Mongo = options.Mongo
	data.Features.Redis = options.Redis
	data.Features.Kafka = options.Kafka

	plan := NewGenerationPlan()
	for _, file := range projectFiles(options) {
		content, err := g.renderer.Render(file.template, data)
		if err != nil {
			return fmt.Errorf("render %s: %w", file.path, err)
		}
		if err := plan.AddCreate(file.path, content); err != nil {
			return err
		}
	}
	if err := g.fs.CreateDir(projectPath); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}
	if err := g.executor.Execute(projectPath, plan); err != nil {
		_ = g.fs.Remove(projectPath)
		return fmt.Errorf("write project files: %w", err)
	}
	if options.Git {
		if err := runCommand(projectPath, "git", "init"); err != nil {
			fmt.Printf("git init failed: %v\n", err)
		}
	}
	if err := runCommand(projectPath, "go", "mod", "tidy"); err != nil {
		fmt.Printf("go mod tidy failed: %v\n", err)
	}
	return nil
}

func projectFiles(options ProjectOptions) []projectFile {
	files := []projectFile{
		{"project/go.mod.tmpl", "go.mod"},
		{"project/main.go.tmpl", filepath.Join("cmd", "main.go")},
		{"project/initiator/initiator.go.tmpl", filepath.Join("initiator", "initiator.go")},
		{"project/initiator/module.go.tmpl", filepath.Join("initiator", "module.go")},
		{"project/initiator/persistence.go.tmpl", filepath.Join("initiator", "persistence.go")},
		{"project/initiator/config.go.tmpl", filepath.Join("initiator", "config.go")},
		{"project/initiator/handler.go.tmpl", filepath.Join("initiator", "handler.go")},
		{"project/initiator/routes.go.tmpl", filepath.Join("initiator", "routes.go")},
		{"project/gox.yaml.tmpl", ".gox.yaml"},
	}
	if options.Postgres {
		files = append(files, projectFile{"capability/postgres/client.go.tmpl", filepath.Join("internal", "infrastructure", "postgres", "client.go")})
	}
	if options.Mongo {
		files = append(files, projectFile{"capability/mongo/client.go.tmpl", filepath.Join("internal", "infrastructure", "mongo", "client.go")})
	}
	if options.Redis {
		files = append(files, projectFile{"capability/redis/client.go.tmpl", filepath.Join("internal", "infrastructure", "redis", "client.go")})
	}
	if options.Kafka {
		files = append(files, projectFile{"capability/kafka/producer.go.tmpl", filepath.Join("internal", "infrastructure", "kafka", "producer.go")})
	}
	return files
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
