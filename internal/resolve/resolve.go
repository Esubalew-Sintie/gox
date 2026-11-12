// Package resolve re-wires constructors by type (ADR 0004). It loads the
// generated project with go/packages, reads every constructor the
// composition root calls, checks that what it returns implements the layer
// interface, and chooses an argument for each parameter from the values the
// Init function has in scope. Nothing is annotated; the signature is the
// contract.
package resolve

import (
	"errors"
	"fmt"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Layer names, used in wiring keys.
const (
	LayerRepository = "repository"
	LayerService    = "service"
	LayerHandler    = "handler"
)

// Wiring records the argument expressions of every constructor call in the
// composition root, keyed by Key(feature, layer).
type Wiring map[string][]string

// Key builds a wiring key.
func Key(feature, layer string) string { return feature + "/" + layer }

// Provider is a value an Init function has in scope, with its static type.
type Provider struct {
	Expr string
	Type types.Type
}

// Constructor is one function the composition root calls.
type Constructor struct {
	Feature   string // snake name
	Layer     string
	Package   string // import path of the package that declares Func
	Func      string // usually New
	IfacePath string // import path of the layer interface
	IfaceName string // name of the layer interface, for example FleetRepository
}

// Universe is one load of the project: every package under the root and
// everything they import, type-checked.
type Universe struct {
	root     map[string]*packages.Package
	imported map[string]*types.Package
}

// Load type-checks every package under root from source (NeedSyntax forces
// that; export data would only carry the imports of the exported API), with
// dependencies from export data. Packages that do not compile are still
// returned; PackageErrors reports their problems.
func Load(root string) (*Universe, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedTypes | packages.NeedImports | packages.NeedSyntax,
		Dir:  root,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages in %s: %w", root, err)
	}
	u := &Universe{root: map[string]*packages.Package{}, imported: map[string]*types.Package{}}
	for _, p := range pkgs {
		u.root[p.PkgPath] = p
		if p.Types != nil {
			u.collect(p.Types)
		}
	}
	return u, nil
}

func (u *Universe) collect(p *types.Package) {
	if _, seen := u.imported[p.Path()]; seen {
		return
	}
	u.imported[p.Path()] = p
	for _, imp := range p.Imports() {
		u.collect(imp)
	}
}

// HasPackage reports whether the package is under the root, compiling or not.
func (u *Universe) HasPackage(path string) bool {
	if u == nil {
		return false
	}
	_, ok := u.root[path]
	return ok
}

// PackageErrors returns the compile errors of a package under the root, or
// nil when it compiled; a package that is not on disk reports one error.
func (u *Universe) PackageErrors(path string) []string {
	p, ok := u.root[path]
	if !ok {
		return []string{"package " + path + " is not in the project"}
	}
	var out []string
	for _, e := range p.Errors {
		out = append(out, e.Error())
	}
	return out
}

// Lookup finds a package-level object anywhere in the universe.
func (u *Universe) Lookup(path, name string) (types.Object, bool) {
	p, ok := u.imported[path]
	if !ok {
		return nil, false
	}
	obj := p.Scope().Lookup(name)
	if obj == nil {
		return nil, false
	}
	return obj, true
}

// Named returns the named type path.name, or false.
func (u *Universe) Named(path, name string) (types.Type, bool) {
	obj, ok := u.Lookup(path, name)
	if !ok {
		return nil, false
	}
	if _, isType := obj.(*types.TypeName); !isType {
		return nil, false
	}
	return obj.Type(), true
}

// Pointer returns *path.name, or false.
func (u *Universe) Pointer(path, name string) (types.Type, bool) {
	t, ok := u.Named(path, name)
	if !ok {
		return nil, false
	}
	return types.NewPointer(t), true
}

// Resolve picks an argument for every parameter of the constructor from the
// providers, and checks the result implements the layer interface.
func (u *Universe) Resolve(c Constructor, providers []Provider) ([]string, error) {
	if errs := u.PackageErrors(c.Package); len(errs) > 0 {
		return nil, fmt.Errorf("%s does not compile:\n  %s", c.Package, strings.Join(errs, "\n  "))
	}
	obj, ok := u.Lookup(c.Package, c.Func)
	if !ok {
		return nil, fmt.Errorf("%s has no function %s", c.Package, c.Func)
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil, fmt.Errorf("%s.%s is not a function", c.Package, c.Func)
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return nil, fmt.Errorf("%s.%s has no signature", c.Package, c.Func)
	}
	if sig.Variadic() {
		return nil, fmt.Errorf("%s.%s is variadic; the composition root cannot call it", c.Package, c.Func)
	}
	if sig.Results().Len() != 1 {
		return nil, fmt.Errorf("%s.%s must return exactly one value, it returns %d", c.Package, c.Func, sig.Results().Len())
	}
	if c.IfaceName != "" {
		iface, ok := u.Named(c.IfacePath, c.IfaceName)
		if !ok {
			return nil, fmt.Errorf("interface %s.%s not found; was it removed from the interface file?", c.IfacePath, c.IfaceName)
		}
		it, ok := iface.Underlying().(*types.Interface)
		if !ok {
			return nil, fmt.Errorf("%s.%s is not an interface", c.IfacePath, c.IfaceName)
		}
		result := sig.Results().At(0).Type()
		if !types.Implements(result, it) {
			return nil, fmt.Errorf("%s.%s returns %s, which does not implement %s.%s", c.Package, c.Func, result, c.IfacePath, c.IfaceName)
		}
	}

	args := make([]string, 0, sig.Params().Len())
	for i := range sig.Params().Len() {
		param := sig.Params().At(i)
		expr, err := pick(param, providers)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", c.Package, c.Func, err)
		}
		args = append(args, expr)
	}
	return args, nil
}

func pick(param *types.Var, providers []Provider) (string, error) {
	var identical, assignable []string
	for _, p := range providers {
		switch {
		case types.Identical(p.Type, param.Type()):
			identical = append(identical, p.Expr)
		case types.AssignableTo(p.Type, param.Type()):
			assignable = append(assignable, p.Expr)
		}
	}
	switch {
	case len(identical) == 1:
		return identical[0], nil
	case len(identical) > 1:
		sort.Strings(identical)
		return "", fmt.Errorf("parameter %s %s has several providers: %s", param.Name(), param.Type(), strings.Join(identical, ", "))
	case len(assignable) == 1:
		return assignable[0], nil
	case len(assignable) > 1:
		sort.Strings(assignable)
		return "", fmt.Errorf("parameter %s %s has several providers: %s", param.Name(), param.Type(), strings.Join(assignable, ", "))
	}
	available := make([]string, 0, len(providers))
	for _, p := range providers {
		available = append(available, fmt.Sprintf("%s %s", p.Expr, p.Type))
	}
	return "", fmt.Errorf("parameter %s %s has no provider; the composition root has: %s", param.Name(), param.Type(), strings.Join(available, ", "))
}

// ErrNotLoaded is returned by Fields when a struct type is unknown.
var ErrNotLoaded = errors.New("type not loaded")

// Fields lists the exported fields of the named struct type with their
// types, for providers such as cfg.Postgres.
func (u *Universe) Fields(path, name string) ([]Provider, error) {
	t, ok := u.Named(path, name)
	if !ok {
		return nil, fmt.Errorf("%w: %s.%s", ErrNotLoaded, path, name)
	}
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return nil, fmt.Errorf("%s.%s is not a struct", path, name)
	}
	var out []Provider
	for i := range st.NumFields() {
		f := st.Field(i)
		if f.Exported() {
			out = append(out, Provider{Expr: f.Name(), Type: f.Type()})
		}
	}
	return out, nil
}
