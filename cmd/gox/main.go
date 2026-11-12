// Command gox generates Go services and keeps their wiring current.
package main

import (
	"os"

	"github.com/esubalew-Sintie/gox/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
