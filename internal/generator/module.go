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

// ModuleGenerator creates complete business modules including wiring.
type ModuleGenerator struct {
	fs       *filesystem.FileSystem
	config   *config.ConfigManager
	renderer *renderer.Renderer
	injector *injector.Injector
	executor *Executor
}

func NewModuleGenerator(
	fs *filesystem.FileSystem,
	configManager *config.ConfigManager,
	renderer *renderer.Renderer,
	injector *injector.Injector,
) *ModuleGenerator {
	return &ModuleGenerator{
		fs:       fs,
		config:   configManager,
		renderer: renderer,
		injector: injector,
		executor: NewExecutor(fs),
	}
}

// ModuleOptions carries the module generation request.
type ModuleOptions struct {
	Name   string
	DryRun bool
}

func (g *ModuleGenerator) BuildPlan(
	projectPath string,
	options ModuleOptions,
) (*GenerationPlan, error) {
	if options.Name == "" {
		return nil, fmt.Errorf("module name is required")
	}

	cfg, err := g.config.Load(projectPath)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	moduleName, err := naming.Parse(options.Name)
	if err != nil {
		return nil, fmt.Errorf("parse module name: %w", err)
	}

	data := NewModuleData(
		filepath.Base(projectPath),
		cfg,
		moduleName,
		options,
	)

	plan := NewGenerationPlan()

	// ---------- render all module files ----------

	repository, err := g.renderer.Render("repository.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render repository: %w", err)
	}

	service, err := g.renderer.Render("service.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render service: %w", err)
	}

	handler, err := g.renderer.Render("handler.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render handler: %w", err)
	}

	moduleWiring, err := g.renderer.Render("module.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render module wiring: %w", err)
	}

	route, err := g.renderer.Render("route.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render route: %w", err)
	}

	model, err := g.renderer.Render("model.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render model: %w", err)
	}

	dto, err := g.renderer.Render("dto.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render dto: %w", err)
	}

	// ---------- render wrapper files (lightweight delegates) ----------
	wrapperRepo, err := g.renderer.Render("wrapper_repository.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render wrapper repository: %w", err)
	}

	wrapperService, err := g.renderer.Render("wrapper_service.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render wrapper service: %w", err)
	}

	wrapperHandler, err := g.renderer.Render("wrapper_handler.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render wrapper handler: %w", err)
	}

	wrapperGlue, err := g.renderer.Render("wrapper_glue_route.go.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("render wrapper glue route: %w", err)
	}

	// ---------- module base paths ----------

	// Keep module wiring in internal/module/<module>
	modBase := filepath.Join("internal", "module", moduleName.Snake)

	// Also place generated artifacts into the more structured layout
	handlerPath := filepath.Join("internal", "handler", "rest", "http", moduleName.Snake, moduleName.Snake+".go")
	servicePath := filepath.Join("internal", "service", moduleName.Snake, moduleName.Snake+".go")
	repositoryPath := filepath.Join("internal", "storage", "persistence", moduleName.Snake, moduleName.Snake+".go")
	glueRoutePath := filepath.Join("internal", "glue", "routing", moduleName.Snake, moduleName.Snake+".go")
	constantModelPath := filepath.Join("internal", "constant", "entities", moduleName.Snake+".go")
	constantDtoPath := filepath.Join("internal", "constant", "entities", "dto", moduleName.Snake+".go")

	// ---------- add CREATE operations ----------

	files := []struct {
		path    string
		content []byte
	}{
		// Structured wrapper locations
		{repositoryPath, wrapperRepo},
		{servicePath, wrapperService},
		{handlerPath, wrapperHandler},
		{glueRoutePath, wrapperGlue},

		// Also create the canonical implementations inside internal/module/<module>
		{filepath.Join(modBase, "repository.go"), repository},
		{filepath.Join(modBase, "service.go"), service},
		{filepath.Join(modBase, "handler.go"), handler},
		{filepath.Join(modBase, "route.go"), route},

		// Module wiring
		{filepath.Join(modBase, "module.go"), moduleWiring},

		// Constants (entities/dto)
		{constantModelPath, model},
		{constantDtoPath, dto},
	}

	for _, f := range files {
		if err := plan.AddCreate(f.path, f.content); err != nil {
			return nil, err
		}
	}

	// ---------- inject into modules.go ----------

	modulesGoPath := filepath.Join(projectPath, "initiator", "modules.go")

	if g.fs.Exists(modulesGoPath) {
		modulesSource, err := g.fs.ReadFile(modulesGoPath)
		if err != nil {
			return nil, fmt.Errorf("read modules.go: %w", err)
		}

		importLine := fmt.Sprintf(
			`"%s/internal/module/%s"`,
			cfg.Module,
			moduleName.Snake,
		)
		fieldLine := fmt.Sprintf(
			`%s %s.Module`,
			moduleName.Pascal,
			moduleName.Snake,
		)

		var dbArg string
		if cfg.Storage.Postgres {
			dbArg = "db"
		}
		initLine := fmt.Sprintf(
			`%s: %s.New(%s),`,
			moduleName.Pascal,
			moduleName.Snake,
			dbArg,
		)

		updated, err := g.injector.Inject(modulesSource, injector.ImportsMarker, importLine)
		if err != nil {
			return nil, fmt.Errorf("inject import into modules.go: %w", err)
		}

		updated, err = g.injector.Inject(updated, injector.FieldsMarker, fieldLine)
		if err != nil {
			return nil, fmt.Errorf("inject field into modules.go: %w", err)
		}

		updated, err = g.injector.Inject(updated, injector.InitMarker, initLine)
		if err != nil {
			return nil, fmt.Errorf("inject init into modules.go: %w", err)
		}

		if err := plan.AddModify("initiator/modules.go", updated); err != nil {
			return nil, err
		}
	}

	// ---------- inject into routes.go ----------

	routesGoPath := filepath.Join(projectPath, "initiator", "routes.go")

	if g.fs.Exists(routesGoPath) {
		routesSource, err := g.fs.ReadFile(routesGoPath)
		if err != nil {
			return nil, fmt.Errorf("read routes.go: %w", err)
		}

		// Import glue routing package so routes live under internal/glue/routing/<module>
		routeImport := fmt.Sprintf(
			`"%s/internal/glue/routing/%s"`,
			cfg.Module,
			moduleName.Snake,
		)
		routeCall := fmt.Sprintf(
			`%s.Register(mux, modules.%s.Handler)`,
			moduleName.Snake,
			moduleName.Pascal,
		)

		updated, err := g.injector.Inject(routesSource, injector.ImportsMarker, routeImport)
		if err != nil {
			return nil, fmt.Errorf("inject import into routes.go: %w", err)
		}

		updated, err = g.injector.Inject(updated, injector.RoutesMarker, routeCall)
		if err != nil {
			return nil, fmt.Errorf("inject route into routes.go: %w", err)
		}

		if err := plan.AddModify("initiator/routes.go", updated); err != nil {
			return nil, err
		}
	}

	return plan, nil
}

func (g *ModuleGenerator) Generate(
	projectPath string,
	options ModuleOptions,
) error {
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

	// Post-generation: gofmt + go build
	if err := runCommand(projectPath, "go", "build", "./..."); err != nil {
		fmt.Printf("  ⚠ go build failed: %v\n", err)
	}

	return nil
}

// printPlan outputs a dry-run summary without touching the filesystem.
func printPlan(plan *GenerationPlan) {
	fmt.Println("Plan")
	fmt.Println()

	var creates, modifies []string

	for _, op := range plan.Operations {
		switch op.Type {
		case OperationCreate:
			creates = append(creates, op.Path)
		case OperationModify:
			modifies = append(modifies, op.Path)
		}
	}

	if len(creates) > 0 {
		fmt.Println("CREATE")
		for _, p := range creates {
			fmt.Println(" ", p)
		}
		fmt.Println()
	}

	if len(modifies) > 0 {
		fmt.Println("MODIFY")
		for _, p := range modifies {
			fmt.Println(" ", p)
		}
		fmt.Println()
	}

	fmt.Println("No files were changed.")
}
