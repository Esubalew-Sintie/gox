package generator

import (
	"fmt"
	"path/filepath"

	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"github.com/esubalew-Sintie/gox/internal/injector"
	"github.com/esubalew-Sintie/gox/internal/naming"
	"github.com/esubalew-Sintie/gox/internal/renderer"
)

// ModuleGenerator generates a feature and wires it into the project's
// composition root without editing user-owned feature code.
type ModuleGenerator struct {
	fs       *filesystem.FileSystem
	config   *config.ConfigManager
	renderer *renderer.Renderer
	injector *injector.Injector
	executor *Executor
}

func NewModuleGenerator(fs *filesystem.FileSystem, cfg *config.ConfigManager, r *renderer.Renderer, i *injector.Injector) *ModuleGenerator {
	return &ModuleGenerator{
		fs: fs, config: cfg, renderer: r, injector: i, executor: NewExecutor(fs),
	}
}

type ModuleOptions struct {
	Name   string
	DryRun bool
}

func (g *ModuleGenerator) BuildPlan(projectPath string, options ModuleOptions) (*GenerationPlan, error) {
	if options.Name == "" {
		return nil, fmt.Errorf("module name is required")
	}

	cfg, err := g.config.Load(projectPath)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	name, err := naming.Parse(options.Name)
	if err != nil {
		return nil, fmt.Errorf("parse module name: %w", err)
	}

	layout, err := buildModuleLayout(cfg.Layout, cfg.Module, cfg.Storage.Driver, name)
	if err != nil {
		return nil, err
	}

	data := NewModuleData(filepath.Base(projectPath), cfg, name, options)
	plan := NewGenerationPlan()

	for _, file := range layout.files {
		content, err := g.renderer.Render(file.template, data)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", file.path, err)
		}
		if err := plan.AddCreate(file.path, content); err != nil {
			return nil, err
		}
	}

	for _, wiring := range layout.wiring {
		if err := g.inject(projectPath, plan, wiring); err != nil {
			return nil, err
		}
	}

	return plan, nil
}

func (g *ModuleGenerator) inject(projectPath string, plan *GenerationPlan, wiring layerWiring) error {
	source, err := g.fs.ReadFile(filepath.Join(projectPath, wiring.path))
	if err != nil {
		return fmt.Errorf("read %s: %w", wiring.path, err)
	}

	updated := source
	for _, importPath := range wiring.imports {
		updated, err = g.injector.Inject(updated, injector.ImportsMarker, importPath)
		if err != nil {
			return fmt.Errorf("inject import into %s: %w", wiring.path, err)
		}
	}

	if wiring.field != "" {
		updated, err = g.injector.Inject(updated, injector.FieldsMarker, wiring.field)
		if err != nil {
			return fmt.Errorf("inject field into %s: %w", wiring.path, err)
		}
	}

	updated, err = g.injector.Inject(updated, injector.InitMarker, wiring.init)
	if err != nil {
		return fmt.Errorf("inject initialization into %s: %w", wiring.path, err)
	}

	return plan.AddModify(wiring.path, updated)
}

func (g *ModuleGenerator) Generate(projectPath string, options ModuleOptions) error {
	plan, err := g.BuildPlan(projectPath, options)
	if err != nil {
		return err
	}

	if options.DryRun {
		printPlan(plan)
		return nil
	}

	if err := g.executor.Execute(projectPath, plan); err != nil {
		return fmt.Errorf("execute generation plan: %w", err)
	}

	if err := runCommand(projectPath, "go", "test", "./..."); err != nil {
		return fmt.Errorf("generated project does not compile: %w", err)
	}

	return nil
}

func printPlan(plan *GenerationPlan) {
	for _, op := range plan.Operations {
		fmt.Printf("%s %s\n", op.Type, op.Path)
	}
}

func (g *ModuleGenerator) BuildRemovePlan(projectPath, resourceName string) (*GenerationPlan, error) {
	cfg, err := g.config.Load(projectPath)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	name, err := naming.Parse(resourceName)
	if err != nil {
		return nil, fmt.Errorf("parse resource name: %w", err)
	}

	layout, err := buildModuleLayout(cfg.Layout, cfg.Module, cfg.Storage.Driver, name)
	if err != nil {
		return nil, err
	}

	plan := NewGenerationPlan()
	for _, file := range layout.files {
		if err := plan.AddDelete(file.path); err != nil {
			return nil, err
		}
	}
	for _, wiring := range layout.wiring {
		if err := g.removeWiring(projectPath, plan, wiring); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

func (g *ModuleGenerator) removeWiring(projectPath string, plan *GenerationPlan, wiring layerWiring) error {
	source, err := g.fs.ReadFile(filepath.Join(projectPath, wiring.path))
	if err != nil {
		return fmt.Errorf("read %s: %w", wiring.path, err)
	}

	updated := source
	if wiring.field != "" {
		updated, err = g.injector.Remove(updated, injector.FieldsMarker, wiring.field)
		if err != nil {
			return fmt.Errorf("remove field from %s: %w", wiring.path, err)
		}
	}
	updated, err = g.injector.Remove(updated, injector.InitMarker, wiring.init)
	if err != nil {
		return fmt.Errorf("remove initialization from %s: %w", wiring.path, err)
	}
	for _, importPath := range wiring.imports {
		updated, err = g.injector.Remove(updated, injector.ImportsMarker, importPath)
		if err != nil {
			return fmt.Errorf("remove import from %s: %w", wiring.path, err)
		}
	}
	return plan.AddModify(wiring.path, updated)
}

func (g *ModuleGenerator) Remove(projectPath, resourceName string, dryRun bool) error {
	plan, err := g.BuildRemovePlan(projectPath, resourceName)
	if err != nil {
		return err
	}
	if dryRun {
		printPlan(plan)
		return nil
	}
	if err := g.executor.Execute(projectPath, plan); err != nil {
		return fmt.Errorf("execute removal plan: %w", err)
	}
	if err := runCommand(projectPath, "go", "test", "./..."); err != nil {
		return fmt.Errorf("project does not compile after removal: %w", err)
	}
	return nil
}
