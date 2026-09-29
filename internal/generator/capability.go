package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"github.com/esubalew-Sintie/gox/internal/renderer"
)

// CapabilityGenerator adds optional infrastructure clients to existing projects.
type CapabilityGenerator struct {
	fs       *filesystem.FileSystem
	config   *config.ConfigManager
	renderer *renderer.Renderer
	executor *Executor
}

func NewCapabilityGenerator(
	fs *filesystem.FileSystem,
	configManager *config.ConfigManager,
	renderer *renderer.Renderer,
) *CapabilityGenerator {
	return &CapabilityGenerator{
		fs:       fs,
		config:   configManager,
		renderer: renderer,
		executor: NewExecutor(fs),
	}
}

type CapabilityOptions struct {
	Name string
}

type capabilitySpec struct {
	template string
	path     string
	module   string
	enabled  func(*config.Config) bool
	enable   func(*config.Config)
}

var capabilitySpecs = map[string]capabilitySpec{
	"postgres": {
		template: "capability/postgres/client.go.tmpl",
		path:     filepath.Join("internal", "infrastructure", "postgres", "client.go"),
		module:   "github.com/jackc/pgx/v5@v5.7.6",
		enabled:  func(cfg *config.Config) bool { return cfg.Storage.Postgres },
		enable:   func(cfg *config.Config) { cfg.Storage.Postgres = true },
	},
	"mongo": {
		template: "capability/mongo/client.go.tmpl",
		path:     filepath.Join("internal", "infrastructure", "mongo", "client.go"),
		module:   "go.mongodb.org/mongo-driver@v1.17.6",
		enabled:  func(cfg *config.Config) bool { return cfg.Storage.Mongo },
		enable:   func(cfg *config.Config) { cfg.Storage.Mongo = true },
	},
	"redis": {
		template: "capability/redis/client.go.tmpl",
		path:     filepath.Join("internal", "infrastructure", "redis", "client.go"),
		module:   "github.com/redis/go-redis/v9@v9.12.1",
		enabled:  func(cfg *config.Config) bool { return cfg.Features.Redis },
		enable:   func(cfg *config.Config) { cfg.Features.Redis = true },
	},
	"kafka": {
		template: "capability/kafka/producer.go.tmpl",
		path:     filepath.Join("internal", "infrastructure", "kafka", "producer.go"),
		module:   "github.com/segmentio/kafka-go@v0.4.47",
		enabled:  func(cfg *config.Config) bool { return cfg.Features.Kafka },
		enable:   func(cfg *config.Config) { cfg.Features.Kafka = true },
	},
}

// Add writes a generated adapter, records the capability, and adds its module.
func (g *CapabilityGenerator) Add(projectPath string, options CapabilityOptions) error {
	name := strings.ToLower(strings.TrimSpace(options.Name))
	if name == "postgresql" {
		name = "postgres"
	}
	if name == "mongodb" {
		name = "mongo"
	}

	spec, ok := capabilitySpecs[name]
	if !ok {
		return fmt.Errorf("unknown capability %q; supported: postgres, mongo, redis, kafka", options.Name)
	}

	cfg, err := g.config.Load(projectPath)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if spec.enabled(cfg) {
		return fmt.Errorf("%s is already configured for this project", name)
	}

	content, err := g.renderer.Render(spec.template, struct{}{})
	if err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}
	plan := NewGenerationPlan()
	if err := plan.AddCreate(spec.path, content); err != nil {
		return err
	}
	if err := g.executor.Execute(projectPath, plan); err != nil {
		return fmt.Errorf("write %s adapter: %w", name, err)
	}

	spec.enable(cfg)
	if err := g.config.Save(projectPath, cfg); err != nil {
		return fmt.Errorf("save project config: %w", err)
	}
	if err := runCommand(projectPath, "go", "get", spec.module); err != nil {
		return fmt.Errorf("add %s dependency: %w", name, err)
	}
	if err := runCommand(projectPath, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("tidy module after adding %s: %w", name, err)
	}
	return nil
}
