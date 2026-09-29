package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindProjectRoot walks upward from cwd until it finds a .gox.yaml file.
// Returns the absolute path of the project root or an error if not found.
func FindProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	dir := cwd

	for {
		candidate := filepath.Join(dir, ".gox.yaml")

		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return "", fmt.Errorf(
		"no gox project found\nRun \"gox new <project>\" first",
	)
}
