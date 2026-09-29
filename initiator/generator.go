package initiator

import (
	"io/fs"

	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"github.com/esubalew-Sintie/gox/internal/generator"
	"github.com/esubalew-Sintie/gox/internal/injector"
	"github.com/esubalew-Sintie/gox/internal/renderer"
)

// deps holds all wired generator dependencies.
type deps struct {
	projectGen    *generator.ProjectGenerator
	moduleGen     *generator.ModuleGenerator
	capabilityGen *generator.CapabilityGenerator
}

// buildDeps wires all generator dependencies.
func buildDeps(fsys fs.FS) (*deps, error) {
	tmpl, err := renderer.LoadTemplates(fsys)
	if err != nil {
		return nil, err
	}

	rend := renderer.New(tmpl)
	inject := injector.New()
	fsOps := filesystem.New()
	cfgMgr := config.NewConfigManager(fsOps)

	return &deps{
		projectGen:    generator.NewProjectGenerator(fsOps, cfgMgr, rend),
		moduleGen:     generator.NewModuleGenerator(fsOps, cfgMgr, rend, inject),
		capabilityGen: generator.NewCapabilityGenerator(fsOps, cfgMgr, rend),
	}, nil
}