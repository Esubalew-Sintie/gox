package generate

import (
	"fmt"
	"go/types"
	"sort"

	"github.com/esubalew-Sintie/gox/internal/db"
	"github.com/esubalew-Sintie/gox/internal/model"
	"github.com/esubalew-Sintie/gox/internal/resolve"
)

// constructor describes the call the composition root makes for one layer
// of a feature.
func (g *Generator) constructor(f model.Feature, layer string) resolve.Constructor {
	l := g.layout
	mod := g.project.Module
	pascal := f.Name.Pascal
	c := resolve.Constructor{Feature: f.Name.Snake, Layer: layer, Func: "New"}
	switch layer {
	case resolve.LayerRepository:
		c.IfacePath, c.IfaceName = model.ImportPath(mod, l.StorageDir), pascal+"Repository"
		if g.project.DB == string(db.Memory) {
			c.Package, c.Func = model.ImportPath(mod, model.MemstoreDir), "New"+pascal+"Store"
		} else {
			c.Package = model.ImportPath(mod, l.FeatureRepositoryDir(f, dbInfoFor(g.project.DB).DBPackage))
		}
	case resolve.LayerService:
		c.Package = model.ImportPath(mod, l.FeatureServiceDir(f))
		c.IfacePath, c.IfaceName = model.ImportPath(mod, l.ServiceDir), pascal+"Service"
	case resolve.LayerHandler:
		c.Package = model.ImportPath(mod, l.FeatureHandlerDir(f))
		c.IfacePath, c.IfaceName = model.ImportPath(mod, l.HandlerDir), pascal+"Handler"
	}
	return c
}

// Constructors lists every constructor call of the composition root.
func (g *Generator) Constructors() []resolve.Constructor {
	var out []resolve.Constructor
	for i := range g.project.Features {
		for _, layer := range []string{resolve.LayerRepository, resolve.LayerService, resolve.LayerHandler} {
			out = append(out, g.constructor(g.project.Features[i], layer))
		}
	}
	return out
}

// DefaultArgs are the arguments gox writes before any resolution: the
// feature's own repository or service, the infrastructure client and the
// logger.
func (g *Generator) DefaultArgs(c resolve.Constructor) []string {
	pascal := ""
	if f, ok := g.project.Feature(c.Feature); ok {
		pascal = f.Name.Pascal
	}
	switch c.Layer {
	case resolve.LayerRepository:
		info := dbInfoFor(g.project.DB)
		if info.Memory {
			return []string{}
		}
		return []string{"i." + info.InfraField, "log"}
	case resolve.LayerService:
		return []string{"s." + pascal, "log"}
	case resolve.LayerHandler:
		return []string{"s." + pascal, "log"}
	}
	return nil
}

// Providers lists the values the Init function of a layer has in scope,
// typed from the loaded project. Providers whose type is not loaded are
// left out.
func (g *Generator) Providers(u *resolve.Universe, layer string) []resolve.Provider {
	l := g.layout
	mod := g.project.Module
	var out []resolve.Provider
	logger, hasLog := u.Pointer("log/slog", "Logger")
	infra := func() {
		info := dbInfoFor(g.project.DB)
		var t types.Type
		var ok bool
		switch db.Kind(g.project.DB) {
		case db.Postgres:
			t, ok = u.Pointer("github.com/jackc/pgx/v5/pgxpool", "Pool")
		case db.MySQL, db.SQLite:
			t, ok = u.Pointer("database/sql", "DB")
		case db.Mongo:
			t, ok = u.Pointer("go.mongodb.org/mongo-driver/v2/mongo", "Database")
		case db.Memory:
		}
		if ok {
			out = append(out, resolve.Provider{Expr: "i." + info.InfraField, Type: t})
		}
	}
	switch layer {
	case resolve.LayerRepository:
		infra()
	case resolve.LayerService:
		for i := range g.project.Features {
			f := g.project.Features[i]
			if t, ok := u.Named(model.ImportPath(mod, l.StorageDir), f.Name.Pascal+"Repository"); ok {
				out = append(out, resolve.Provider{Expr: "s." + f.Name.Pascal, Type: t})
			}
		}
		infra()
		configPath := model.ImportPath(mod, "internal/config")
		if t, ok := u.Pointer(configPath, "Config"); ok {
			out = append(out, resolve.Provider{Expr: "cfg", Type: t})
		}
		if fields, err := u.Fields(configPath, "Config"); err == nil {
			for _, f := range fields {
				out = append(out, resolve.Provider{Expr: "cfg." + f.Expr, Type: f.Type})
			}
		}
	case resolve.LayerHandler:
		for i := range g.project.Features {
			f := g.project.Features[i]
			if t, ok := u.Named(model.ImportPath(mod, l.ServiceDir), f.Name.Pascal+"Service"); ok {
				out = append(out, resolve.Provider{Expr: "s." + f.Name.Pascal, Type: t})
			}
		}
	}
	if hasLog {
		out = append(out, resolve.Provider{Expr: "log", Type: logger})
	}
	return out
}

// Wiring resolves every constructor against the loaded project. A
// constructor whose package is not in the project yet gets its defaults
// silently; one whose package exists but does not resolve keeps the
// previous wiring, or the defaults, and is reported in warnings.
func (g *Generator) Wiring(u *resolve.Universe, previous resolve.Wiring) (resolve.Wiring, []string) {
	wiring := resolve.Wiring{}
	var warnings []string
	for _, c := range g.Constructors() {
		key := resolve.Key(c.Feature, c.Layer)
		if u == nil || !u.HasPackage(c.Package) {
			if prev, ok := previous[key]; ok {
				wiring[key] = prev
			} else {
				wiring[key] = g.DefaultArgs(c)
			}
			continue
		}
		args, err := u.Resolve(c, g.Providers(u, c.Layer))
		if err != nil {
			if prev, ok := previous[key]; ok {
				wiring[key] = prev
				warnings = append(warnings, fmt.Sprintf("%s: %v; keeping the previous wiring", key, err))
			} else {
				wiring[key] = g.DefaultArgs(c)
				warnings = append(warnings, fmt.Sprintf("%s: %v; using the default wiring", key, err))
			}
			continue
		}
		wiring[key] = args
	}
	sort.Strings(warnings)
	return wiring, warnings
}
