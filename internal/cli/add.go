package cli

import (
	"fmt"

	"github.com/esubalew-Sintie/gox/internal/generator"
)

// RunAdd handles `gox add <capability>`.
func RunAdd(gen *generator.CapabilityGenerator, capability string) error {
	if capability == "" {
		return fmt.Errorf(
			"capability name is required\nUsage: gox add <capability>\nSupported: postgres, redis, kafka",
		)
	}

	projectPath, err := FindProjectRoot()
	if err != nil {
		return err
	}

	fmt.Printf("Adding capability: %s\n\n", capability)

	if err := gen.Add(projectPath, generator.CapabilityOptions{Name: capability}); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("✓ %s configured\n", capability)

	return nil
}
