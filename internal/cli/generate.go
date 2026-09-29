package cli

import (
	"fmt"

	"github.com/esubalew-Sintie/gox/internal/generator"
)

// RunGenerate handles `gox generate module <name>`.
func RunGenerate(gen *generator.ModuleGenerator, name string, dryRun bool) error {
	if name == "" {
		return fmt.Errorf("module name is required\nUsage: gox generate module <name>")
	}

	projectPath, err := FindProjectRoot()
	if err != nil {
		return err
	}

	opts := generator.ModuleOptions{
		Name:   name,
		DryRun: dryRun,
	}

	fmt.Printf("Generating module: %s\n\n", name)

	if err := gen.Generate(projectPath, opts); err != nil {
		return err
	}

	if !dryRun {
		fmt.Println()
		fmt.Println("✓ Files generated")
		fmt.Println("✓ Dependencies wired")
		fmt.Println("✓ Routes registered")
		fmt.Println("✓ Code formatted")
		fmt.Println()
		fmt.Printf("Module %q created successfully.\n", name)
	}

	return nil
}
