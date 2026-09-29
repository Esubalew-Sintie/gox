package generator

import (
	"fmt"
	"path/filepath"

	"github.com/esubalew-Sintie/gox/internal/architecture"
	"github.com/esubalew-Sintie/gox/internal/naming"
)

type generatedFile struct {
	template string
	path     string
}

type layerWiring struct {
	path       string
	importPath string
	field      string
	init       string
}

type moduleLayout struct {
	files  []generatedFile
	wiring []layerWiring
}

func buildModuleLayout(layout, modulePath string, name *naming.Name) (moduleLayout, error) {
	switch architecture.Normalize(layout) {
	case architecture.LayeredREST:
		return layeredRESTLayout(modulePath, name), nil
	case architecture.FeatureFirst:
		return featureFirstLayout(modulePath, name), nil
	default:
		return moduleLayout{}, fmt.Errorf("unsupported layout %q", layout)
	}
}

func layeredRESTLayout(modulePath string, name *naming.Name) moduleLayout {
	return moduleLayout{
		files: []generatedFile{
			{"module/persistence.go.tmpl", filepath.Join("internal", "storage", "persistence", name.Snake, name.Snake+".go")},
			{"module/service.go.tmpl", filepath.Join("internal", "module", name.Snake, name.Snake+".go")},
			{"module/handler.go.tmpl", filepath.Join("internal", "handler", "rest", "http", name.Snake, name.Snake+".go")},
			{"module/handler_test.go.tmpl", filepath.Join("internal", "handler", "rest", "http", name.Snake, name.Snake+"_test.go")},
			{"module/route.go.tmpl", filepath.Join("internal", "glue", "routing", name.Snake, name.Snake+".go")},
			{"module/model.go.tmpl", filepath.Join("internal", "constant", "model", name.Snake+".go")},
			{"module/dto.go.tmpl", filepath.Join("internal", "constant", "model", "dto", name.Snake+".go")},
		},
		wiring: []layerWiring{
			{"initiator/persistence.go", fmt.Sprintf("%q", modulePath+"/internal/storage/persistence/"+name.Snake), fmt.Sprintf("%s %s.Repository", name.Pascal, name.Snake), fmt.Sprintf("%s: %s.Init(),", name.Pascal, name.Snake)},
			{"initiator/module.go", fmt.Sprintf("%q", modulePath+"/internal/module/"+name.Snake), fmt.Sprintf("%s %s.Service", name.Pascal, name.Snake), fmt.Sprintf("%s: %s.Init(persistence.%s),", name.Pascal, name.Snake, name.Pascal)},
			{"initiator/handler.go", fmt.Sprintf("%q", modulePath+"/internal/handler/rest/http/"+name.Snake), fmt.Sprintf("%s *%s.Handler", name.Pascal, name.Snake), fmt.Sprintf("%s: %s.Init(module.%s),", name.Pascal, name.Snake, name.Pascal)},
			{"initiator/routes.go", fmt.Sprintf("%q", modulePath+"/internal/glue/routing/"+name.Snake), "", fmt.Sprintf("%s.InitRoutes(router, handlers.%s)", name.Snake, name.Pascal)},
		},
	}
}

func featureFirstLayout(modulePath string, name *naming.Name) moduleLayout {
	base := filepath.Join("internal", "feature", name.Snake)
	importPath := fmt.Sprintf("%q", modulePath+"/internal/feature/"+name.Snake)
	return moduleLayout{
		files: []generatedFile{
			{"feature-first/repository.go.tmpl", filepath.Join(base, "repository.go")},
			{"feature-first/service.go.tmpl", filepath.Join(base, "service.go")},
			{"feature-first/handler.go.tmpl", filepath.Join(base, "handler.go")},
			{"feature-first/handler_test.go.tmpl", filepath.Join(base, "handler_test.go")},
			{"feature-first/route.go.tmpl", filepath.Join(base, "route.go")},
		},
		wiring: []layerWiring{
			{"initiator/persistence.go", importPath, fmt.Sprintf("%s *%s.Repository", name.Pascal, name.Snake), fmt.Sprintf("%s: %s.NewRepository(),", name.Pascal, name.Snake)},
			{"initiator/module.go", importPath, fmt.Sprintf("%s *%s.Service", name.Pascal, name.Snake), fmt.Sprintf("%s: %s.NewService(persistence.%s),", name.Pascal, name.Snake, name.Pascal)},
			{"initiator/handler.go", importPath, fmt.Sprintf("%s *%s.Handler", name.Pascal, name.Snake), fmt.Sprintf("%s: %s.NewHandler(module.%s),", name.Pascal, name.Snake, name.Pascal)},
			{"initiator/routes.go", importPath, "", fmt.Sprintf("%s.RegisterRoutes(router, handlers.%s)", name.Snake, name.Pascal)},
		},
	}
}
