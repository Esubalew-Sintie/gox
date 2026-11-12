package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/esubalew-Sintie/gox/internal/apply"
	"github.com/esubalew-Sintie/gox/internal/generate"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/model"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/version"
)

func destroyCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Remove generated code from the project",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "feature <Name>",
		Short: "Remove a feature and its wiring",
		Long: `Remove a feature: every file gox wrote for it whose content is unchanged,
its declarations in the interface files and the error catalogue, and the
regenerated composition root. Files you edited are left in place and listed;
--force deletes them too.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDestroyFeature(cmd, g, args[0])
		},
	})
	return cmd
}

func runDestroyFeature(cmd *cobra.Command, g *globalFlags, name string) error {
	p, err := loadProject(g.dir)
	if err != nil {
		return err
	}
	f, ok := p.model.Feature(name)
	if !ok {
		return fmt.Errorf("feature %q does not exist in gox.yaml", name)
	}

	var remove []string
	for _, e := range p.manifest.ByOwner(model.FeatureOwner(f)) {
		if e.Class != manifest.ClassCodemod {
			remove = append(remove, e.Path)
		}
	}

	newSpec := p.spec.Clone()
	newSpec.RemoveFeature(name)
	newModel, err := model.FromSpec(newSpec)
	if err != nil {
		return err
	}

	gen := generate.New(p.root, newModel, p.manifest, p.store)
	edits, err := gen.RemoveFeature(f, g.force)
	if err != nil {
		return err
	}
	wiring := p.wiring(gen, g.writer(cmd))
	tools, err := gen.Tool(wiring)
	if err != nil {
		return err
	}
	desired := make([]plan.Desired, 0, len(edits)+len(tools))
	desired = append(desired, edits...)
	desired = append(desired, tools...)

	pl, err := plan.New(p.root, p.manifest, plan.Options{Force: g.force, Adopt: g.adopt, Version: version.Version}).Build(desired, remove)
	if err != nil {
		return err
	}
	pl.Spec = newSpec
	pl.Manifest.Wiring = wiring
	confirm := fmt.Sprintf("Destroy feature %s: %d file(s) will be deleted. Continue?", f.Name.Snake, len(remove))
	return g.run(cmd, p.root, pl, apply.Options{AllowDirty: g.allowDirty}, confirm)
}
