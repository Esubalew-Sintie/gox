package initiator

import (
	"embed"
)

// templateFS embeds all templates. The "templates" entry here is a symlink
// to ../templates (the repo root templates directory).
//
//go:embed all:templates
var templateFS embed.FS