package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/esubalew-Sintie/gox/internal/generator"
)

// RunNew handles the `gox new` command.
func RunNew(gen *generator.ProjectGenerator, opts generator.ProjectOptions) error {
	// If options already supplied (non-interactive), use them directly.
	if opts.Name != "" && opts.Module != "" {
		return runNewWithOpts(gen, opts)
	}

	// Interactive flow
	if opts.Name == "" {
		if err := survey.AskOne(&survey.Input{
			Message: "Project name:",
		}, &opts.Name, survey.WithValidator(survey.Required)); err != nil {
			return fmt.Errorf("prompt project name: %w", err)
		}
	}

	if opts.Module == "" {
		defaultModule := "github.com/your-org/" + opts.Name

		if err := survey.AskOne(&survey.Input{
			Message: "Go module path:",
			Default: defaultModule,
		}, &opts.Module, survey.WithValidator(survey.Required)); err != nil {
			return fmt.Errorf("prompt module path: %w", err)
		}
	}

	// HTTP framework (stored for future use; currently informational)
	var framework string
	if err := survey.AskOne(&survey.Select{
		Message: "HTTP framework:",
		Options: []string{"net/http (standard library)", "chi", "gin"},
		Default: "net/http (standard library)",
	}, &framework); err != nil {
		return fmt.Errorf("prompt http framework: %w", err)
	}

	// Database
	var db string
	if err := survey.AskOne(&survey.Select{
		Message: "Database:",
		Options: []string{"None", "PostgreSQL", "MySQL"},
		Default: "None",
	}, &db); err != nil {
		return fmt.Errorf("prompt database: %w", err)
	}
	opts.Postgres = strings.EqualFold(db, "postgresql")

	// Redis
	var redisStr string
	if err := survey.AskOne(&survey.Select{
		Message: "Redis:",
		Options: []string{"No", "Yes"},
		Default: "No",
	}, &redisStr); err != nil {
		return fmt.Errorf("prompt redis: %w", err)
	}
	opts.Redis = strings.EqualFold(redisStr, "yes")

	// Kafka
	var kafkaStr string
	if err := survey.AskOne(&survey.Select{
		Message: "Kafka:",
		Options: []string{"No", "Yes"},
		Default: "No",
	}, &kafkaStr); err != nil {
		return fmt.Errorf("prompt kafka: %w", err)
	}
	opts.Kafka = strings.EqualFold(kafkaStr, "yes")

	// Git
	var gitStr string
	if err := survey.AskOne(&survey.Select{
		Message: "Initialize Git repository?",
		Options: []string{"Yes", "No"},
		Default: "Yes",
	}, &gitStr); err != nil {
		return fmt.Errorf("prompt git: %w", err)
	}
	opts.Git = strings.EqualFold(gitStr, "yes")

	return runNewWithOpts(gen, opts)
}

func runNewWithOpts(gen *generator.ProjectGenerator, opts generator.ProjectOptions) error {
	fmt.Printf("\nCreating project: %s\n\n", opts.Name)

	if err := gen.Generate(opts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("✓ Project created\n")

	if opts.Postgres {
		fmt.Println("✓ PostgreSQL configured")
	}

	if opts.Git {
		fmt.Println("✓ Git initialized")
	}

	fmt.Println("✓ Dependencies installed")
	fmt.Println()
	fmt.Printf("Project %q created successfully.\n", opts.Name)
	fmt.Printf("\n  cd %s\n", opts.Name)
	fmt.Printf("  gox g module <name>\n\n")

	os.Exit(0)
	return nil
}
