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
	path    string
	imports []string
	field   string
	init    string
}

type moduleLayout struct {
	files  []generatedFile
	wiring []layerWiring
}

func buildModuleLayout(layout, modulePath, driver string, name *naming.Name) (moduleLayout, error) {
	switch architecture.Normalize(layout) {
	case architecture.LayeredREST:
		return layeredRESTLayout(modulePath, driver, name), nil
	case architecture.FeatureFirst:
		return featureFirstLayout(modulePath, name), nil
	default:
		return moduleLayout{}, fmt.Errorf("unsupported layout %q", layout)
	}
}

func layeredRESTLayout(modulePath, driver string, name *naming.Name) moduleLayout {
	persistencePath := modulePath + "/internal/storage/persistence"
	moduleContractPath := modulePath + "/internal/module"
	handlerContractPath := modulePath + "/internal/handler/rest"
	entityPath := modulePath + "/internal/constant/entities/" + name.Snake
	dtoPath := modulePath + "/internal/constant/dto"

	persistenceContract := fmt.Sprintf("type %s interface {\n\tCreate(dto.Create%sRequest) %s.%s\n\tList() []%s.%s\n\tGet(string) (%s.%s, bool)\n\tUpdate(string, dto.Update%sRequest) (%s.%s, bool)\n\tDelete(string) bool\n}", name.Pascal, name.Pascal, name.Snake, name.Pascal, name.Snake, name.Pascal, name.Snake, name.Pascal, name.Pascal, name.Snake, name.Pascal)
	handlerContract := fmt.Sprintf("type %s interface {\n\tCreate(http.ResponseWriter, *http.Request)\n\tList(http.ResponseWriter, *http.Request)\n\tGet(http.ResponseWriter, *http.Request)\n\tUpdate(http.ResponseWriter, *http.Request)\n\tDelete(http.ResponseWriter, *http.Request)\n\tHealth(http.ResponseWriter, *http.Request)\n}", name.Pascal)

	return moduleLayout{
		files: []generatedFile{
			{"module/persistence.go.tmpl", filepath.Join("internal", "storage", "persistence", name.Snake, name.Snake+".go")},
			{"module/service.go.tmpl", filepath.Join("internal", "module", name.Snake, name.Snake+".go")},
			{"module/handler.go.tmpl", filepath.Join("internal", "handler", "rest", "http", name.Snake, name.Snake+".go")},
			{"module/handler_test.go.tmpl", filepath.Join("internal", "handler", "rest", "http", name.Snake, name.Snake+"_test.go")},
			{"module/route.go.tmpl", filepath.Join("internal", "glue", "routing", name.Snake, name.Snake+".go")},
			{"module/model.go.tmpl", filepath.Join("internal", "constant", "entities", name.Snake, name.Snake+".go")},
			{"module/dto.go.tmpl", filepath.Join("internal", "constant", "dto", name.Snake+".go")},
		},
		wiring: []layerWiring{
			{filepath.Join("internal", "storage", "persistence", "persistence.go"), []string{fmt.Sprintf("%q", dtoPath), fmt.Sprintf("%q", entityPath)}, "", persistenceContract},
			{filepath.Join("internal", "module", "module.go"), []string{fmt.Sprintf("%q", dtoPath), fmt.Sprintf("%q", entityPath)}, "", persistenceContract},
			{filepath.Join("internal", "handler", "rest", "handler.go"), []string{"\"net/http\""}, "", handlerContract},
			{"initiator/persistence.go", []string{"\"fmt\"", fmt.Sprintf("%q", persistencePath), fmt.Sprintf("%q", persistencePath+"/"+name.Snake)}, fmt.Sprintf("%s persistence.%s", name.Pascal, name.Pascal), persistenceInitialization(driver, name)},
			{"initiator/module.go", []string{fmt.Sprintf("%q", moduleContractPath), fmt.Sprintf("%q", moduleContractPath+"/"+name.Snake)}, fmt.Sprintf("%s module.%s", name.Pascal, name.Pascal), fmt.Sprintf("%s: %s.Init(persistence.%s),", name.Pascal, name.Snake, name.Pascal)},
			{"initiator/handler.go", []string{fmt.Sprintf("%q", handlerContractPath), fmt.Sprintf("%q", handlerContractPath+"/http/"+name.Snake)}, fmt.Sprintf("%s rest.%s", name.Pascal, name.Pascal), fmt.Sprintf("%s: %s.Init(module.%s),", name.Pascal, name.Snake, name.Pascal)},
			{"initiator/routes.go", []string{fmt.Sprintf("%q", modulePath+"/internal/glue/routing/"+name.Snake)}, "", fmt.Sprintf("%s.InitRoutes(router, handlers.%s)", name.Snake, name.Pascal)},
		},
	}
}

func persistenceInitialization(driver string, name *naming.Name) string {
	argument := ""
	switch driver {
	case "postgres":
		argument = "dependencies.Postgres"
	case "mongo":
		argument = "dependencies.MongoDatabase"
	}

	call := name.Snake + ".Init(" + argument + ")"
	repository := name.Camel + "Repository"
	return fmt.Sprintf("%s, err := %s\nif err != nil {\n\treturn Persistence{}, fmt.Errorf(\"initialize %s persistence: %%w\", err)\n}\npersistence.%s = %s", repository, call, name.Snake, name.Pascal, repository)
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
			{"initiator/persistence.go", []string{importPath}, fmt.Sprintf("%s *%s.Repository", name.Pascal, name.Snake), fmt.Sprintf("persistence.%s = %s.NewRepository()", name.Pascal, name.Snake)},
			{"initiator/module.go", []string{importPath}, fmt.Sprintf("%s *%s.Service", name.Pascal, name.Snake), fmt.Sprintf("%s: %s.NewService(persistence.%s),", name.Pascal, name.Snake, name.Pascal)},
			{"initiator/handler.go", []string{importPath}, fmt.Sprintf("%s *%s.Handler", name.Pascal, name.Snake), fmt.Sprintf("%s: %s.NewHandler(module.%s),", name.Pascal, name.Snake, name.Pascal)},
			{"initiator/routes.go", []string{importPath}, "", fmt.Sprintf("%s.RegisterRoutes(router, handlers.%s)", name.Snake, name.Pascal)},
		},
	}
}
