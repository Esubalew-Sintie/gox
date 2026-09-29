package initiator

import (
	"fmt"
	"os"

	"github.com/esubalew-Sintie/gox/internal/cli"
	"github.com/esubalew-Sintie/gox/internal/generator"
	"github.com/spf13/cobra"
)

// buildRootCommand constructs the full cobra command tree.
func buildRootCommand(d *deps) *cobra.Command {
	root := &cobra.Command{
		Use:   "gox",
		Short: "gox — Go project generator",
		Long: `gox is a CLI-based Go project generator.

It eliminates repetitive application scaffolding while preserving
idiomatic Go architecture and explicit application code.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newCmd(d),
		addCmd(d),
		removeCmd(d),
		generateCmd(d),
		versionCmd(),
		doctorCmd(),
	)

	return root
}

// ─── gox new ───────────────────────────────────────────────────────────────

func newCmd(d *deps) *cobra.Command {
	var (
		modulePath string
		layout     string
		http       string
		db         string
		withOpts   string
		git        bool
	)

	cmd := &cobra.Command{
		Use:   "new <project>",
		Short: "Create a new Go project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := generator.ProjectOptions{}

			if len(args) > 0 {
				opts.Name = args[0]
			}

			opts.Module = modulePath
			opts.Layout = layout
			opts.HTTP = http
			opts.Git = git

			// Resolve db flag
			switch db {
			case "postgres", "postgresql":
				opts.Postgres = true
			case "mongo", "mongodb":
				opts.Mongo = true
			}

			// Resolve --with flag (e.g. redis,kafka)
			if withOpts != "" {
				for _, w := range splitCSV(withOpts) {
					switch w {
					case "redis":
						opts.Redis = true
					case "kafka":
						opts.Kafka = true
					case "postgres", "postgresql":
						opts.Postgres = true
					case "mongo", "mongodb":
						opts.Mongo = true
					}
				}
			}

			_ = http // stored for future multi-framework support

			return cli.RunNew(d.projectGen, opts)
		},
	}

	cmd.Flags().StringVar(&modulePath, "module", "", "Go module path (e.g. github.com/acme/api)")
	cmd.Flags().StringVar(&layout, "layout", "layered-rest", "Architecture: layered-rest, feature-first")
	cmd.Flags().StringVar(&http, "http", "", "HTTP router: net/http, chi, gin")
	cmd.Flags().StringVar(&db, "db", "", "Storage: postgres, mongo")
	cmd.Flags().StringVar(&withOpts, "with", "", "Optional capabilities: redis,kafka,mongo,postgres")
	cmd.Flags().BoolVar(&git, "git", false, "Initialize a Git repository")

	return cmd
}

// ─── gox add ───────────────────────────────────────────────────────────────

func addCmd(d *deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <capability>",
		Short: "Add a capability to an existing project",
		Long: `Add optional infrastructure or capabilities to an existing project.

Examples:
  gox add postgres
  gox add mongo
  gox add redis
  gox add kafka`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.RunAdd(d.capabilityGen, args[0])
		},
	}

	return cmd
}

// ─── gox remove ────────────────────────────────────────────────────────────

func removeCmd(d *deps) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "remove resource <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a generated resource",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "resource" && args[0] != "module" {
				return fmt.Errorf("supported removal targets: resource, module")
			}
			projectPath, err := cli.FindProjectRoot()
			if err != nil {
				return err
			}
			return d.moduleGen.Remove(projectPath, args[1], dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing files")
	return cmd
}

// ─── gox generate ──────────────────────────────────────────────────────────

func generateCmd(d *deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"g"},
		Short:   "Generate application components",
	}

	cmd.AddCommand(generateModuleCmd(d))

	return cmd
}

func generateModuleCmd(d *deps) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "module <name>",
		Aliases: []string{"mod", "resource", "r"},
		Short:   "Generate a complete business resource",
		Long: `Generate a complete business resource including:
  - Repository
  - Service
  - Handler
  - Module wiring
  - Route registration
  - Model and DTO

All files are automatically wired into the application.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.RunGenerate(d.moduleGen, args[0], dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing files")

	return cmd
}

// ─── gox version ───────────────────────────────────────────────────────────

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show gox version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gox %s\n", Version)
		},
	}
}

// ─── gox doctor ────────────────────────────────────────────────────────────

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose project and tooling problems",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.RunDoctor()
		},
	}
}

// ─── helpers ───────────────────────────────────────────────────────────────

func splitCSV(s string) []string {
	var out []string
	start := 0

	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			if part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}

	return out
}

// Execute runs the root command and exits with a non-zero code on error.
func Execute(d *deps) {
	root := buildRootCommand(d)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
