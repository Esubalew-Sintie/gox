package initiator

import (
	"fmt"
	"io/fs"
	"os"
)

// Start is the composition root for the gox CLI.
// It loads templates, wires generators, and runs the command.
func Start() {
	fsys, err := templateSubFS()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error loading templates:", err)
		os.Exit(1)
	}

	d, err := buildDeps(fsys)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error initializing gox:", err)
		os.Exit(1)
	}

	Execute(d)
}

// templateSubFS returns a sub-filesystem rooted at the templates directory.
func templateSubFS() (fs.FS, error) {
	sub, err := fs.Sub(templateFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("sub templates FS: %w", err)
	}

	return sub, nil
}
