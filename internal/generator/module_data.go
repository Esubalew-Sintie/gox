package generator

import (
	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/naming"
)

// ModuleData is the template data fed to all module templates.
type ModuleData struct {
	ProjectName string
	ModulePath  string
	HTTP        string

	Name   string
	Pascal string
	Camel  string
	Snake  string
	Kebab  string
	Plural string

	Storage StorageData
	Options ModuleOptions
}

type StorageData struct {
	Driver   string
	Postgres bool
	Mongo    bool
	Redis    bool
}

func NewModuleData(projectName string, cfg *config.Config, name *naming.Name, options ModuleOptions) *ModuleData {
	return &ModuleData{
		ProjectName: projectName,
		ModulePath:  cfg.Module,
		HTTP:        cfg.HTTP,
		Name:        name.Original,
		Pascal:      name.Pascal,
		Camel:       name.Camel,
		Snake:       name.Snake,
		Kebab:       name.Kebab,
		Plural:      name.Plural,
		Storage: StorageData{
			Driver:   cfg.Storage.Driver,
			Postgres: cfg.Storage.Postgres,
			Mongo:    cfg.Storage.Mongo,
			Redis:    cfg.Features.Redis,
		},
		Options: options,
	}
}
