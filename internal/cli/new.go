package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/esubalew-Sintie/gox/internal/generator"
)

func RunNew(gen *generator.ProjectGenerator, opts generator.ProjectOptions) error {
	if opts.Name != "" && opts.Module != "" {
		return runNewWithOpts(gen, opts)
	}

	if opts.Name == "" {
		if err := survey.AskOne(&survey.Input{Message: "Project name:"}, &opts.Name, survey.WithValidator(survey.Required)); err != nil {
			return fmt.Errorf("prompt project name: %w", err)
		}
	}
	if opts.Module == "" {
		defaultModule := "github.com/your-org/" + opts.Name
		if err := survey.AskOne(&survey.Input{Message: "Go module path:", Default: defaultModule}, &opts.Module, survey.WithValidator(survey.Required)); err != nil {
			return fmt.Errorf("prompt module path: %w", err)
		}
	}

	var router string
	if err := survey.AskOne(&survey.Select{
		Message: "HTTP router:",
		Options: []string{"net/http", "chi", "gin"},
		Default: "net/http",
	}, &router); err != nil {
		return fmt.Errorf("prompt http router: %w", err)
	}
	opts.HTTP = router

	var database string
	if err := survey.AskOne(&survey.Select{
		Message: "Persistence database:",
		Options: []string{"Memory", "PostgreSQL", "MongoDB"},
		Default: "Memory",
	}, &database); err != nil {
		return fmt.Errorf("prompt persistence database: %w", err)
	}
	opts.Database = strings.ToLower(database)
	switch opts.Database {
	case "postgresql":
		opts.Database = "postgres"
	case "mongodb":
		opts.Database = "mongo"
	}

	var capabilities []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Infrastructure capabilities:",
		Options: []string{"Redis", "Kafka"},
	}, &capabilities); err != nil {
		return fmt.Errorf("prompt infrastructure capabilities: %w", err)
	}
	for _, capability := range capabilities {
		switch capability {
		case "Redis":
			opts.Redis = true
		case "Kafka":
			opts.Kafka = true
		}
	}

	var git string
	if err := survey.AskOne(&survey.Select{Message: "Initialize Git repository?", Options: []string{"Yes", "No"}, Default: "Yes"}, &git); err != nil {
		return fmt.Errorf("prompt git: %w", err)
	}
	opts.Git = strings.EqualFold(git, "yes")

	return runNewWithOpts(gen, opts)
}

func runNewWithOpts(gen *generator.ProjectGenerator, opts generator.ProjectOptions) error {
	fmt.Printf("\nCreating project: %s\n\n", opts.Name)
	if err := gen.Generate(opts); err != nil {
		return err
	}
	fmt.Println("\nProject created")
	fmt.Printf("\n  cd %s\n", opts.Name)
	fmt.Println("  gox g resource <name>")
	os.Exit(0)
	return nil
}
