package generator

import (
	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/naming"
)

// ModuleData is the template data fed to all module templates.
type ModuleData struct {
	ProjectName string
	ModulePath  string

	Name   string
	Pascal string
	Camel  string
	Snake  string
	Kebab  string
	Plural string

	Storage StorageData
	Options ModuleOptions
}

// StorageData reflects the project's storage capabilities.
type StorageData struct {
	Postgres bool
	Redis    bool
}

// NewModuleData constructs ModuleData from project config and naming.
func NewModuleData(
	projectName string,
	cfg *config.Config,
	name *naming.Name,
	options ModuleOptions,
) *ModuleData {
	return &ModuleData{
		ProjectName: projectName,
		ModulePath:  cfg.Module,

		Name:   name.Original,
		Pascal: name.Pascal,
		Camel:  name.Camel,
		Snake:  name.Snake,
		Kebab:  name.Kebab,
		Plural: name.Plural,

		Storage: StorageData{
			Postgres: cfg.Storage.Postgres,
			Redis:    cfg.Features.Redis,
		},

		Options: options,
	}
}