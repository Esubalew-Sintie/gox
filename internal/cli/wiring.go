package cli

import (
	"github.com/esubalew-Sintie/gox/internal/generate"
	"github.com/esubalew-Sintie/gox/internal/resolve"
	"github.com/esubalew-Sintie/gox/internal/transcript"
)

// wiring resolves the composition root's constructor calls against the
// project on disk, keeping the manifest's wiring for anything that does not
// resolve, and reports every fallback.
func (p *project) wiring(gen *generate.Generator, out *transcript.Writer) resolve.Wiring {
	u, err := resolve.Load(p.root)
	if err != nil {
		out.Warn("could not load the project for wiring: %v; keeping the recorded wiring", err)
		u = nil
	}
	wiring, warnings := gen.Wiring(u, p.manifest.Wiring)
	for _, w := range warnings {
		out.Warn("%s", w)
	}
	return wiring
}
