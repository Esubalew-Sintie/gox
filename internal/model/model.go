// Package model is the in-memory form of a gox project: the spec parsed into
// typed values, the field DSL resolved, and the layout that maps features to
// paths. Templates and planners read the model and never the raw spec.
package model

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/naming"
)

// Project is a validated gox.yaml.
type Project struct {
	Name         string // last element of the module path, used for binary and compose names
	Module       string
	Arch         string
	HTTP         string
	DB           string
	Capabilities []string
	Features     []Feature
}

// Feature is one generated feature with its parsed fields.
type Feature struct {
	Name       naming.Name
	Fields     []Field
	Transports []string
}

// Field is one entity field from the DSL.
type Field struct {
	Name     naming.Name
	Type     FieldType
	Nullable bool
}

// FieldType is a DSL type name.
type FieldType string

// The DSL types of ADR 0008.
const (
	TypeString  FieldType = "string"
	TypeText    FieldType = "text"
	TypeInt     FieldType = "int"
	TypeInt64   FieldType = "int64"
	TypeFloat64 FieldType = "float64"
	TypeBool    FieldType = "bool"
	TypeTime    FieldType = "time"
	TypeUUID    FieldType = "uuid"
)

// FieldTypes lists the DSL types in a stable order.
var FieldTypes = []FieldType{TypeString, TypeText, TypeInt, TypeInt64, TypeFloat64, TypeBool, TypeTime, TypeUUID}

type typeInfo struct {
	goType     string
	importPath string
}

var fieldTypes = map[FieldType]typeInfo{
	TypeString:  {goType: "string"},
	TypeText:    {goType: "string"},
	TypeInt:     {goType: "int"},
	TypeInt64:   {goType: "int64"},
	TypeFloat64: {goType: "float64"},
	TypeBool:    {goType: "bool"},
	TypeTime:    {goType: "time.Time", importPath: "time"},
	TypeUUID:    {goType: "string"},
}

// Reserved field names that gox generates on every entity.
var Reserved = []string{"id", "created_at", "updated_at"}

// GoType is the Go type of the field in the entity, a pointer when nullable.
func (f Field) GoType() string {
	t := fieldTypes[f.Type].goType
	if f.Nullable {
		return "*" + t
	}
	return t
}

// Import is the package the Go type needs, or "".
func (f Field) Import() string {
	return fieldTypes[f.Type].importPath
}

// String renders the field back in DSL form.
func (f Field) String() string {
	s := f.Name.Snake + ":" + string(f.Type)
	if f.Nullable {
		s += "?"
	}
	return s
}

// ParseField parses "name:type", "name:type?" or "name:type:null". The
// last form exists because zsh treats an unquoted ? as a glob.
func ParseField(s string) (Field, error) {
	s = strings.TrimSpace(s)
	namePart, typePart, ok := strings.Cut(s, ":")
	if !ok || namePart == "" || typePart == "" {
		return Field{}, fmt.Errorf("field %q must be name:type", s)
	}
	nullable := strings.HasSuffix(typePart, "?")
	typePart = strings.TrimSuffix(typePart, "?")
	if base, suffix, ok := strings.Cut(typePart, ":"); ok {
		if suffix != "null" {
			return Field{}, fmt.Errorf("field %q: unknown suffix %q; use ? or :null for a nullable field", s, suffix)
		}
		nullable, typePart = true, base
	}
	ft := FieldType(typePart)
	if _, known := fieldTypes[ft]; !known {
		return Field{}, fmt.Errorf("field %q: unknown type %q; known types: %s", s, typePart, joinTypes())
	}
	name, err := naming.Parse(namePart)
	if err != nil {
		return Field{}, fmt.Errorf("field %q: %w", s, err)
	}
	if slices.Contains(Reserved, name.Snake) {
		return Field{}, fmt.Errorf("field %q: %s is generated on every entity and cannot be declared", s, name.Snake)
	}
	return Field{Name: *name, Type: ft, Nullable: nullable}, nil
}

// ParseFields parses a list and rejects duplicate names.
func ParseFields(specs []string) ([]Field, error) {
	fields := make([]Field, 0, len(specs))
	seen := map[string]bool{}
	var errs []error
	for _, s := range specs {
		f, err := ParseField(s)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if seen[f.Name.Snake] {
			errs = append(errs, fmt.Errorf("field %q is declared twice", f.Name.Snake))
			continue
		}
		seen[f.Name.Snake] = true
		fields = append(fields, f)
	}
	return fields, errors.Join(errs...)
}

// FromSpec builds the model from a validated spec. It parses every field and
// defaults transports to http.
func FromSpec(spec *manifest.Spec) (*Project, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	p := &Project{
		Name:         path.Base(spec.Module),
		Module:       spec.Module,
		Arch:         spec.Arch,
		HTTP:         spec.HTTP,
		DB:           spec.DB,
		Capabilities: slices.Clone(spec.Capabilities),
	}
	var errs []error
	for _, fs := range spec.Features {
		f, err := FeatureFromSpec(fs)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		p.Features = append(p.Features, f)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return p, nil
}

// FeatureFromSpec parses one feature.
func FeatureFromSpec(fs manifest.FeatureSpec) (Feature, error) {
	name, err := naming.Parse(fs.Name)
	if err != nil {
		return Feature{}, err
	}
	fields, err := ParseFields(fs.Fields)
	if err != nil {
		return Feature{}, fmt.Errorf("feature %q: %w", fs.Name, err)
	}
	transports := slices.Clone(fs.Transports)
	if len(transports) == 0 {
		transports = []string{"http"}
	}
	return Feature{Name: *name, Fields: fields, Transports: transports}, nil
}

// Feature returns the feature by name, or false.
func (p *Project) Feature(name string) (Feature, bool) {
	want, err := naming.Parse(name)
	if err != nil {
		return Feature{}, false
	}
	for i := range p.Features {
		if p.Features[i].Name.Snake == want.Snake {
			return p.Features[i], true
		}
	}
	return Feature{}, false
}

// Has reports whether a capability is enabled.
func (p *Project) Has(capability string) bool {
	return slices.Contains(p.Capabilities, capability)
}

// Imports returns the extra imports an entity with these fields needs.
func (f Feature) Imports() []string {
	var out []string
	for _, fld := range f.Fields {
		if imp := fld.Import(); imp != "" && !slices.Contains(out, imp) {
			out = append(out, imp)
		}
	}
	slices.Sort(out)
	return out
}

// HasTransport reports whether the feature exposes the transport.
func (f Feature) HasTransport(t string) bool {
	return slices.Contains(f.Transports, t)
}

func joinTypes() string {
	names := make([]string, len(FieldTypes))
	for i, t := range FieldTypes {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}
