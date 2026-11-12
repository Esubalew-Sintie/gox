// Package codemod edits the seven developer-owned files gox is allowed to
// touch (ADR 0005). Every edit is a whole top-level declaration, a map entry
// or a struct field, located with go/ast and applied as a text splice, then
// formatted with goimports. Nothing else in the file is rewritten.
package codemod

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"
)

// Sentinel errors callers test for.
var (
	ErrExists   = errors.New("declaration already exists with different content")
	ErrNotFound = errors.New("declaration not found")
)

// File is one Go source file under edit.
type File struct {
	name string
	src  []byte
	fset *token.FileSet
	file *ast.File
}

// Parse parses src. name is the file path, used in errors.
func Parse(name string, src []byte) (*File, error) {
	f := &File{name: name}
	if err := f.reparse(src); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *File) reparse(src []byte) error {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, f.name, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", f.name, err)
	}
	f.src, f.fset, f.file = src, fset, parsed
	return nil
}

// Bytes returns the current source. Call Format first for canonical output.
func (f *File) Bytes() []byte { return f.src }

// Format runs goimports: gofmt layout plus removal of imports that an edit
// left unused. localPrefix groups the project's own imports last.
func (f *File) Format(localPrefix string) error {
	imports.LocalPrefix = localPrefix
	out, err := imports.Process(f.name, f.src, &imports.Options{Comments: true, TabIndent: true, TabWidth: 8})
	if err != nil {
		return fmt.Errorf("format %s: %w", f.name, err)
	}
	if err := f.reparse(out); err != nil {
		return err
	}
	return f.collapseSingleImport()
}

// collapseSingleImport rewrites `import (\n\t"x"\n)` as `import "x"`, which
// goimports leaves alone, so a file returns to its original bytes when the
// last gox-added import is removed.
func (f *File) collapseSingleImport() error {
	for _, d := range f.file.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		if !gen.Lparen.IsValid() || len(gen.Specs) != 1 || gen.Doc != nil {
			return nil
		}
		spec, ok := gen.Specs[0].(*ast.ImportSpec)
		if !ok || spec.Doc != nil || spec.Comment != nil {
			return nil
		}
		start, end := f.offset(gen.Pos()), f.offset(gen.End())
		single := "import " + string(f.src[f.offset(spec.Pos()):f.offset(spec.End())])
		var buf bytes.Buffer
		buf.Write(f.src[:start])
		buf.WriteString(single)
		buf.Write(f.src[end:])
		return f.reparse(buf.Bytes())
	}
	return nil
}

// DeclNames lists the names of every top-level declaration, with var and
// const blocks contributing each of their names.
func (f *File) DeclNames() []string {
	var names []string
	for _, d := range f.file.Decls {
		names = append(names, declNames(d)...)
	}
	return names
}

// HasDecl reports whether a top-level declaration with the name exists.
func (f *File) HasDecl(name string) bool {
	_, ok := f.findDecl(name)
	return ok
}

// DeclSource returns the formatted source of a declaration, doc comment included.
func (f *File) DeclSource(name string) (string, bool) {
	d, ok := f.findDecl(name)
	if !ok {
		return "", false
	}
	start, end := f.declRange(d)
	return strings.TrimSpace(string(f.src[start:end])), true
}

// AppendDecl adds one top-level declaration at the end of the file. It is a
// no-op when an identical declaration exists and ErrExists when one with the
// same name but different content exists.
func (f *File) AppendDecl(decl string) error {
	return f.insertDecl(decl, len(f.src))
}

// InsertDeclBefore adds a declaration before the named one, for example a
// feature's var block before ErrorMap.
func (f *File) InsertDeclBefore(anchor, decl string) error {
	d, ok := f.findDecl(anchor)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, anchor)
	}
	start, _ := f.declRange(d)
	return f.insertDecl(decl, start)
}

func (f *File) insertDecl(decl string, at int) error {
	names, normalized, err := parseDecl(decl)
	if err != nil {
		return err
	}
	for _, n := range names {
		existing, ok := f.DeclSource(n)
		if !ok {
			continue
		}
		if existing == normalized {
			return nil
		}
		return fmt.Errorf("%w: %s in %s", ErrExists, n, f.name)
	}
	var buf bytes.Buffer
	buf.Write(f.src[:at])
	if at > 0 && !bytes.HasSuffix(f.src[:at], []byte("\n\n")) {
		if bytes.HasSuffix(f.src[:at], []byte("\n")) {
			buf.WriteByte('\n')
		} else {
			buf.WriteString("\n\n")
		}
	}
	buf.WriteString(normalized)
	buf.WriteString("\n\n")
	buf.Write(f.src[at:])
	return f.reparse(buf.Bytes())
}

// RemoveDecl deletes the named declaration with its doc comment. It reports
// false when the declaration does not exist.
func (f *File) RemoveDecl(name string) (bool, error) {
	d, ok := f.findDecl(name)
	if !ok {
		return false, nil
	}
	start, end := f.declRange(d)
	return true, f.cutLines(start, end)
}

// AddImports makes sure the paths are imported.
func (f *File) AddImports(paths ...string) error {
	changed := false
	for _, p := range paths {
		if astutil.AddImport(f.fset, f.file, p) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	var buf bytes.Buffer
	if err := printNode(&buf, f.fset, f.file); err != nil {
		return err
	}
	return f.reparse(buf.Bytes())
}

// AppendMapEntries adds "Key: Value" entries to the composite literal that
// initialises the named map variable, under a comment line such as
// "// Fleet". Entries whose key is already present are skipped.
func (f *File) AppendMapEntries(varName, comment string, entries []string) error {
	lit, err := f.mapLiteral(varName)
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, e := range lit.Elts {
		if k := keyName(e); k != "" {
			present[k] = true
		}
	}
	var add []string
	for _, e := range entries {
		key, _, ok := strings.Cut(e, ":")
		if !ok {
			return fmt.Errorf("map entry %q must be Key: Value", e)
		}
		if !present[strings.TrimSpace(key)] {
			add = append(add, strings.TrimSpace(e))
		}
	}
	if len(add) == 0 {
		return nil
	}
	at := f.offset(lit.Rbrace)
	var buf bytes.Buffer
	buf.Write(f.src[:at])
	if !bytes.HasSuffix(f.src[:at], []byte("\n")) {
		buf.WriteByte('\n')
	}
	if comment != "" {
		buf.WriteString("\n\t" + comment + "\n")
	}
	for _, e := range add {
		buf.WriteString("\t" + strings.TrimSuffix(e, ",") + ",\n")
	}
	buf.Write(f.src[at:])
	return f.reparse(buf.Bytes())
}

// RemoveMapEntries deletes the entries with the given keys from the map
// literal and the comment line directly above the first of them when it is
// exactly comment. It returns how many entries were removed.
func (f *File) RemoveMapEntries(varName, comment string, keys []string) (int, error) {
	lit, err := f.mapLiteral(varName)
	if err != nil {
		return 0, err
	}
	want := map[string]bool{}
	for _, k := range keys {
		want[k] = true
	}
	type span struct{ start, end int }
	var spans []span
	for _, e := range lit.Elts {
		if want[keyName(e)] {
			spans = append(spans, span{f.offset(e.Pos()), f.offset(e.End())})
		}
	}
	if len(spans) == 0 {
		return 0, nil
	}
	if comment != "" {
		if start, end, ok := f.commentLineAbove(spans[0].start, comment); ok {
			spans = append([]span{{start, end}}, spans...)
		}
	}
	for i := len(spans) - 1; i >= 0; i-- {
		if err := f.cutLines(spans[i].start, spans[i].end); err != nil {
			return 0, err
		}
	}
	return len(spans), nil
}

// RemoveVarSpecs deletes the named specs from var blocks, dropping a block
// that becomes empty together with its doc comment. It returns how many
// specs were removed.
func (f *File) RemoveVarSpecs(names []string) (int, error) {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	removed := 0
	for {
		gen, spec := f.findVarSpec(want)
		if spec == nil {
			return removed, nil
		}
		removed++
		if len(gen.Specs) == 1 {
			start, end := f.declRange(gen)
			if err := f.cutLines(start, end); err != nil {
				return removed, err
			}
			continue
		}
		if err := f.cutLines(f.offset(spec.Pos()), f.offset(spec.End())); err != nil {
			return removed, err
		}
	}
}

// AddStructField appends a field to the named struct type.
func (f *File) AddStructField(typeName, field string) error {
	st, err := f.structType(typeName)
	if err != nil {
		return err
	}
	name, _, _ := strings.Cut(strings.TrimSpace(field), " ")
	for _, existing := range st.Fields.List {
		for _, n := range existing.Names {
			if n.Name == name {
				return nil
			}
		}
	}
	at := f.offset(st.Fields.Closing)
	var buf bytes.Buffer
	buf.Write(f.src[:at])
	if !bytes.HasSuffix(f.src[:at], []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.WriteString("\t" + strings.TrimSpace(field) + "\n")
	buf.Write(f.src[at:])
	return f.reparse(buf.Bytes())
}

// RemoveStructField deletes a field from the named struct type.
func (f *File) RemoveStructField(typeName, fieldName string) (bool, error) {
	st, err := f.structType(typeName)
	if err != nil {
		return false, err
	}
	for _, fld := range st.Fields.List {
		for _, n := range fld.Names {
			if n.Name == fieldName {
				return true, f.cutLines(f.offset(fld.Pos()), f.offset(fld.End()))
			}
		}
	}
	return false, nil
}

// cutLines removes the whole lines spanning [start, end) and one following
// blank line when the removal would leave two in a row.
func (f *File) cutLines(start, end int) error {
	ls := bytes.LastIndexByte(f.src[:start], '\n') + 1
	var le int
	if i := bytes.IndexByte(f.src[end:], '\n'); i >= 0 {
		le = end + i + 1
	} else {
		le = len(f.src)
	}
	rest := f.src[le:]
	if bytes.HasPrefix(rest, []byte("\n")) && (ls == 0 || bytes.HasSuffix(f.src[:ls], []byte("\n\n"))) {
		rest = rest[1:]
	}
	var buf bytes.Buffer
	buf.Write(f.src[:ls])
	buf.Write(rest)
	return f.reparse(buf.Bytes())
}

// commentLineAbove returns the span of the line above the line holding
// offset when that line is exactly text; end is the offset of its newline.
func (f *File) commentLineAbove(offset int, text string) (int, int, bool) {
	ls := bytes.LastIndexByte(f.src[:offset], '\n') + 1
	if ls == 0 {
		return 0, 0, false
	}
	prevStart := bytes.LastIndexByte(f.src[:ls-1], '\n') + 1
	line := strings.TrimSpace(string(f.src[prevStart : ls-1]))
	if line == text {
		return prevStart, ls - 1, true
	}
	return 0, 0, false
}

func (f *File) offset(p token.Pos) int { return f.fset.Position(p).Offset }

func (f *File) declRange(d ast.Decl) (int, int) {
	start := d.Pos()
	switch x := d.(type) {
	case *ast.GenDecl:
		if x.Doc != nil {
			start = x.Doc.Pos()
		}
	case *ast.FuncDecl:
		if x.Doc != nil {
			start = x.Doc.Pos()
		}
	}
	return f.offset(start), f.offset(d.End())
}

func (f *File) findDecl(name string) (ast.Decl, bool) {
	for _, d := range f.file.Decls {
		for _, n := range declNames(d) {
			if n == name {
				return d, true
			}
		}
	}
	return nil, false
}

func (f *File) findVarSpec(want map[string]bool) (*ast.GenDecl, *ast.ValueSpec) {
	for _, d := range f.file.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, s := range gen.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, n := range vs.Names {
				if want[n.Name] {
					return gen, vs
				}
			}
		}
	}
	return nil, nil
}

func (f *File) mapLiteral(varName string) (*ast.CompositeLit, error) {
	d, ok := f.findDecl(varName)
	if !ok {
		return nil, fmt.Errorf("%w: var %s in %s", ErrNotFound, varName, f.name)
	}
	gen, ok := d.(*ast.GenDecl)
	if !ok || gen.Tok != token.VAR {
		return nil, fmt.Errorf("%s in %s is not a var", varName, f.name)
	}
	for _, s := range gen.Specs {
		vs, ok := s.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, n := range vs.Names {
			if n.Name != varName || i >= len(vs.Values) {
				continue
			}
			lit, ok := vs.Values[i].(*ast.CompositeLit)
			if !ok {
				return nil, fmt.Errorf("%s in %s is not a composite literal", varName, f.name)
			}
			if _, isMap := lit.Type.(*ast.MapType); !isMap {
				return nil, fmt.Errorf("%s in %s is not a map literal", varName, f.name)
			}
			return lit, nil
		}
	}
	return nil, fmt.Errorf("%w: var %s in %s", ErrNotFound, varName, f.name)
}

func (f *File) structType(typeName string) (*ast.StructType, error) {
	d, ok := f.findDecl(typeName)
	if !ok {
		return nil, fmt.Errorf("%w: type %s in %s", ErrNotFound, typeName, f.name)
	}
	gen, ok := d.(*ast.GenDecl)
	if !ok || gen.Tok != token.TYPE {
		return nil, fmt.Errorf("%s in %s is not a type", typeName, f.name)
	}
	for _, s := range gen.Specs {
		ts, ok := s.(*ast.TypeSpec)
		if !ok || ts.Name.Name != typeName {
			continue
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return nil, fmt.Errorf("%s in %s is not a struct", typeName, f.name)
		}
		return st, nil
	}
	return nil, fmt.Errorf("%w: type %s in %s", ErrNotFound, typeName, f.name)
}

func keyName(e ast.Expr) string {
	kv, ok := e.(*ast.KeyValueExpr)
	if !ok {
		return ""
	}
	switch k := kv.Key.(type) {
	case *ast.Ident:
		return k.Name
	case *ast.SelectorExpr:
		if x, ok := k.X.(*ast.Ident); ok {
			return x.Name + "." + k.Sel.Name
		}
	}
	return ""
}

func declNames(d ast.Decl) []string {
	switch x := d.(type) {
	case *ast.FuncDecl:
		if x.Recv != nil && len(x.Recv.List) > 0 {
			return []string{recvTypeName(x.Recv.List[0].Type) + "." + x.Name.Name}
		}
		return []string{x.Name.Name}
	case *ast.GenDecl:
		var names []string
		for _, s := range x.Specs {
			switch spec := s.(type) {
			case *ast.TypeSpec:
				names = append(names, spec.Name.Name)
			case *ast.ValueSpec:
				for _, n := range spec.Names {
					names = append(names, n.Name)
				}
			}
		}
		return names
	}
	return nil
}

func recvTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	}
	return ""
}

// parseDecl validates a declaration snippet and returns its names and its
// gofmt-normalised source.
func parseDecl(decl string) ([]string, string, error) {
	src := "package p\n\n" + strings.TrimSpace(decl) + "\n"
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "decl.go", src, parser.ParseComments)
	if err != nil {
		return nil, "", fmt.Errorf("parse declaration: %w", err)
	}
	if len(parsed.Decls) != 1 {
		return nil, "", fmt.Errorf("expected exactly one declaration, got %d", len(parsed.Decls))
	}
	var buf bytes.Buffer
	if err := printNode(&buf, fset, parsed); err != nil {
		return nil, "", err
	}
	normalized := strings.TrimSpace(strings.TrimPrefix(buf.String(), "package p"))
	return declNames(parsed.Decls[0]), normalized, nil
}

// Normalize returns the gofmt form of one declaration, which is what
// DeclSource reports, so callers can tell whether a declaration still reads
// as gox wrote it.
func Normalize(decl string) (string, error) {
	_, normalized, err := parseDecl(decl)
	return normalized, err
}
