package generator

import (
	"fmt"
	"path/filepath"

	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"github.com/esubalew-Sintie/gox/internal/renderer"
)

// CapabilityGenerator adds optional infrastructure capabilities to existing projects.
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

// CapabilityOptions names the capability to add.
type CapabilityOptions struct {
	Name string
}

// capabilityTemplateData feeds capability templates.
type capabilityTemplateData struct {
	Module string
}

func (g *CapabilityGenerator) Add(
	projectPath string,
	options CapabilityOptions,
) error {
	if options.Name == "" {
		return fmt.Errorf("capability name is required")
	}

	cfg, err := g.config.Load(projectPath)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}

	data := capabilityTemplateData{Module: cfg.Module}

	switch options.Name {
	case "postgres":
		return g.addPostgres(projectPath, cfg, data)

	case "redis":
		cfg.Features.Redis = true
		if err := g.config.Save(projectPath, cfg); err != nil {
			return fmt.Errorf("save project config: %w", err)
		}
		fmt.Println("  ✓ Redis capability enabled in .gox.yaml")
		fmt.Println("    Add Redis client integration manually or wait for a future gox release.")
		return nil

	case "kafka":
		cfg.Features.Kafka = true
		if err := g.config.Save(projectPath, cfg); err != nil {
			return fmt.Errorf("save project config: %w", err)
		}
		fmt.Println("  ✓ Kafka capability enabled in .gox.yaml")
		fmt.Println("    Add Kafka client integration manually or wait for a future gox release.")
		return nil

	default:
		return fmt.Errorf("unknown capability: %s\nSupported: postgres, redis, kafka", options.Name)
	}
}

func (g *CapabilityGenerator) addPostgres(
	projectPath string,
	cfg *config.Config,
	data capabilityTemplateData,
) error {
	if cfg.Storage.Postgres {
		return fmt.Errorf("PostgreSQL is already configured for this project")
	}

	dbGo, err := g.renderer.Render("database.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("render postgres database.go: %w", err)
	}

	plan := NewGenerationPlan()

	dbPath := filepath.Join("internal", "infrastructure", "database", "database.go")
	if err := plan.AddCreate(dbPath, dbGo); err != nil {
		return err
	}

	if err := g.executor.Execute(projectPath, plan); err != nil {
		return fmt.Errorf("write postgres files: %w", err)
	}

	cfg.Storage.Postgres = true
	if err := g.config.Save(projectPath, cfg); err != nil {
		return fmt.Errorf("save project config: %w", err)
	}

	// Try to get the postgres driver (best-effort)
	if err := runCommand(projectPath, "go", "get", "github.com/lib/pq@latest"); err != nil {
		fmt.Printf("  ⚠ go get github.com/lib/pq failed: %v\n", err)
	}

	return nil
}