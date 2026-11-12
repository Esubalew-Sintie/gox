package cli

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/esubalew-Sintie/gox/internal/plan"
)

func templatesCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "List, eject and diff the templates gox generates from",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List every template id; overridden ones are marked",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				p, err := loadProject(g.dir)
				if err != nil {
					return err
				}
				ids, err := p.store.List()
				if err != nil {
					return err
				}
				overridden, err := p.store.Overridden()
				if err != nil {
					return err
				}
				for _, id := range ids {
					mark := " "
					if slices.Contains(overridden, id) {
						mark = "*"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", mark, id)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d template(s), %d overridden in .gox/templates\n", len(ids), len(overridden))
				return nil
			},
		},
		&cobra.Command{
			Use:   "eject <id>",
			Short: "Copy a template into .gox/templates so you can edit it",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				p, err := loadProject(g.dir)
				if err != nil {
					return err
				}
				target, err := p.store.Eject(args[0], p.root)
				if err != nil {
					return err
				}
				g.writer(cmd).Step("create", target, "edit it; gox uses it from now on")
				return nil
			},
		},
		&cobra.Command{
			Use:   "diff",
			Short: "Show how every overridden template differs from the embedded one",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				p, err := loadProject(g.dir)
				if err != nil {
					return err
				}
				overridden, err := p.store.Overridden()
				if err != nil {
					return err
				}
				if len(overridden) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no overrides in .gox/templates")
					return nil
				}
				out := g.writer(cmd)
				for _, id := range overridden {
					embedded, override, err := p.store.Both(id)
					if err != nil {
						return err
					}
					d := plan.Unified(id, embedded, override)
					if d == "" {
						out.Step("identical", id, "override equals the embedded template")
						continue
					}
					out.Raw(d)
				}
				return nil
			},
		},
	)
	return cmd
}
