package generator

import (
	"fmt"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"github.com/esubalew-Sintie/gox/internal/renderer"
)

// CapabilityGenerator adds infrastructure clients to the initiator composition root.
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
	module  string
	enabled func(*config.Config) bool
	enable  func(*config.Config)
}

var capabilitySpecs = map[string]capabilitySpec{
	"postgres": {
		module:  "github.com/jackc/pgx/v5@v5.7.6",
		enabled: func(cfg *config.Config) bool { return cfg.Storage.Postgres },
		enable:  func(cfg *config.Config) { cfg.Storage.Postgres = true },
	},
	"mongo": {
		module:  "go.mongodb.org/mongo-driver@v1.17.6",
		enabled: func(cfg *config.Config) bool { return cfg.Storage.Mongo },
		enable:  func(cfg *config.Config) { cfg.Storage.Mongo = true },
	},
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

// Add updates the generator-owned initiator dependency lifecycle and config.
func (g *CapabilityGenerator) Add(projectPath string, options CapabilityOptions) error {
	name := normalizeCapability(options.Name)
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

	spec.enable(cfg)
	data := projectTemplateData{Module: cfg.Module}
	data.Storage.Postgres = cfg.Storage.Postgres
	data.Storage.Mongo = cfg.Storage.Mongo
	data.Features.Redis = cfg.Features.Redis
	data.Features.Kafka = cfg.Features.Kafka

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
	if err := runCommand(projectPath, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("tidy module after adding %s: %w", name, err)
	}
	return nil
}

func normalizeCapability(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "postgresql":
		return "postgres"
	case "mongodb":
		return "mongo"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}
