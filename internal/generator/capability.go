package generator

import (
	"fmt"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"github.com/esubalew-Sintie/gox/internal/renderer"
)

// CapabilityGenerator manages optional non-persistence capabilities.
type CapabilityGenerator struct {
	fs       *filesystem.FileSystem
	config   *config.ConfigManager
	renderer *renderer.Renderer
	executor *Executor
}

func NewCapabilityGenerator(fs *filesystem.FileSystem, configManager *config.ConfigManager, renderer *renderer.Renderer) *CapabilityGenerator {
	return &CapabilityGenerator{fs: fs, config: configManager, renderer: renderer, executor: NewExecutor(fs)}
}

type CapabilityOptions struct {
	Name string
}

type capabilitySpec struct {
	module  string
	enabled func(*config.Config) bool
	enable  func(*config.Config)
}

var capabilitySpecs = map[string]capabilitySpec{
	"redis": {
		module:  "github.com/redis/go-redis/v9@v9.12.1",
		enabled: func(cfg *config.Config) bool { return cfg.Features.Redis },
		enable:  func(cfg *config.Config) { cfg.Features.Redis = true },
	},
	"kafka": {
		module:  "github.com/segmentio/kafka-go@v0.4.47",
		enabled: func(cfg *config.Config) bool { return cfg.Features.Kafka },
		enable:  func(cfg *config.Config) { cfg.Features.Kafka = true },
	},
}

func (g *CapabilityGenerator) Add(projectPath string, options CapabilityOptions) error {
	name := strings.ToLower(strings.TrimSpace(options.Name))
	if name == "postgres" || name == "postgresql" || name == "mongo" || name == "mongodb" {
		return fmt.Errorf("database drivers are selected when creating a project; use gox new --db memory|postgres|mongo")
	}

	spec, ok := capabilitySpecs[name]
	if !ok {
		return fmt.Errorf("unknown capability %q; supported: redis, kafka", options.Name)
	}

	cfg, err := g.config.Load(projectPath)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if spec.enabled(cfg) {
		return fmt.Errorf("%s is already configured for this project", name)
	}
	spec.enable(cfg)

	data := newProjectTemplateData("", cfg.Module, cfg.Layout, cfg.HTTP, cfg.Storage.Driver, cfg.Features.Redis, cfg.Features.Kafka)
	content, err := g.renderer.Render("project/initiator/dependencies.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("render initiator dependencies: %w", err)
	}

	plan := NewGenerationPlan()
	const dependencyPath = "initiator/dependencies.go"
	if g.fs.Exists(projectPath + "/" + dependencyPath) {
		if err := plan.AddModify(dependencyPath, content); err != nil {
			return err
		}
	} else if err := plan.AddCreate(dependencyPath, content); err != nil {
		return err
	}
	if err := g.executor.Execute(projectPath, plan); err != nil {
		return fmt.Errorf("write initiator dependencies: %w", err)
	}
	if err := g.config.Save(projectPath, cfg); err != nil {
		return fmt.Errorf("save project config: %w", err)
	}
	if err := runCommand(projectPath, "go", "get", spec.module); err != nil {
		return fmt.Errorf("add %s dependency: %w", name, err)
	}
	return runCommand(projectPath, "go", "mod", "tidy")
}
