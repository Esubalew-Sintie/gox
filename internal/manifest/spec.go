// Package manifest reads and writes the two files gox keeps in a project
// root: gox.yaml, the user-editable intent, and .gox/manifest.json, the
// tool state that records every file gox wrote with its ownership class and
// the hash of its content.
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/module"
	"gopkg.in/yaml.v3"

	"github.com/esubalew-Sintie/gox/internal/fsx"
	"github.com/esubalew-Sintie/gox/internal/naming"
)

// SpecFile is the intent file in the project root.
const SpecFile = "gox.yaml"

// SpecVersion is the only gox.yaml schema version this gox reads.
const SpecVersion = 2

// Spec is the content of gox.yaml. It is the only input needed to
// regenerate every tool-owned file.
type Spec struct {
	Version      int           `yaml:"version"`
	Module       string        `yaml:"module"`
	Arch         string        `yaml:"arch"`
	HTTP         string        `yaml:"http"`
	DB           string        `yaml:"db"`
	Capabilities []string      `yaml:"capabilities,omitempty"`
	Features     []FeatureSpec `yaml:"features,omitempty"`
}

// FeatureSpec is one feature as written in gox.yaml. Fields use the DSL
// "name:type" with an optional trailing "?" for nullable.
type FeatureSpec struct {
	Name       string   `yaml:"name"`
	Fields     []string `yaml:"fields,omitempty"`
	Transports []string `yaml:"transports,omitempty"`
}

// Allowed values. The first entry of each list is the default.
var (
	Archs        = []string{"hexagonal"}
	Routers      = []string{"chi", "stdlib"}
	Databases    = []string{"postgres", "mysql", "sqlite", "mongo", "memory"}
	Capabilities = []string{"redis", "kafka", "s3", "jobs", "otel", "grpc", "openapi"}
	Transports   = []string{"http", "grpc"}
)

// ErrSpecNotFound is returned by LoadSpec when the directory has no gox.yaml.
var ErrSpecNotFound = errors.New("gox.yaml not found")

// LoadSpec reads and validates dir/gox.yaml.
func LoadSpec(dir string) (*Spec, error) {
	data, err := os.ReadFile(filepath.Join(dir, SpecFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSpecNotFound
		}
		return nil, fmt.Errorf("read %s: %w", SpecFile, err)
	}
	return ParseSpec(data)
}

// ParseSpec decodes and validates gox.yaml content. Unknown keys are errors,
// so a typo never silently disables a setting.
func ParseSpec(data []byte) (*Spec, error) {
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", SpecFile, err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Encode renders the spec as gox.yaml content.
func (s *Spec) Encode() ([]byte, error) {
	body, err := yaml.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", SpecFile, err)
	}
	header := "# gox project intent. Edit freely, then run `gox check` to see what changes.\n"
	return append([]byte(header), body...), nil
}

// Save writes dir/gox.yaml atomically.
func (s *Spec) Save(dir string) error {
	data, err := s.Encode()
	if err != nil {
		return err
	}
	return fsx.WriteAtomic(filepath.Join(dir, SpecFile), data, 0o644)
}

// Validate reports every structural problem at once.
func (s *Spec) Validate() error {
	var errs []error
	if s.Version != SpecVersion {
		errs = append(errs, fmt.Errorf("version must be %d, got %d", SpecVersion, s.Version))
	}
	if err := module.CheckPath(s.Module); err != nil {
		errs = append(errs, fmt.Errorf("module: %w", err))
	}
	if !slices.Contains(Archs, s.Arch) {
		errs = append(errs, fmt.Errorf("arch must be one of %s, got %q", strings.Join(Archs, ", "), s.Arch))
	}
	if !slices.Contains(Routers, s.HTTP) {
		errs = append(errs, fmt.Errorf("http must be one of %s, got %q", strings.Join(Routers, ", "), s.HTTP))
	}
	if !slices.Contains(Databases, s.DB) {
		errs = append(errs, fmt.Errorf("db must be one of %s, got %q", strings.Join(Databases, ", "), s.DB))
	}
	seenCap := map[string]bool{}
	for _, c := range s.Capabilities {
		if !slices.Contains(Capabilities, c) {
			errs = append(errs, fmt.Errorf("capability %q is unknown; known: %s", c, strings.Join(Capabilities, ", ")))
		}
		if seenCap[c] {
			errs = append(errs, fmt.Errorf("capability %q is listed twice", c))
		}
		seenCap[c] = true
	}
	seenFeature := map[string]string{}
	for i, f := range s.Features {
		name, err := naming.Parse(f.Name)
		if err != nil {
			errs = append(errs, fmt.Errorf("features[%d]: %w", i, err))
			continue
		}
		if prev, dup := seenFeature[name.Snake]; dup {
			errs = append(errs, fmt.Errorf("feature %q duplicates %q", f.Name, prev))
		}
		seenFeature[name.Snake] = f.Name
		for _, tr := range f.Transports {
			if !slices.Contains(Transports, tr) {
				errs = append(errs, fmt.Errorf("feature %q: transport %q is unknown; known: %s", f.Name, tr, strings.Join(Transports, ", ")))
			}
		}
	}
	return errors.Join(errs...)
}

// Feature returns the feature with the given name, compared after
// normalisation so "FleetGroup" and "fleet_group" are the same feature.
func (s *Spec) Feature(name string) (FeatureSpec, bool) {
	want, err := naming.Parse(name)
	if err != nil {
		return FeatureSpec{}, false
	}
	for _, f := range s.Features {
		if n, err := naming.Parse(f.Name); err == nil && n.Snake == want.Snake {
			return f, true
		}
	}
	return FeatureSpec{}, false
}

// AddFeature appends a feature; it fails when a feature of that name exists.
func (s *Spec) AddFeature(f FeatureSpec) error {
	if _, exists := s.Feature(f.Name); exists {
		return fmt.Errorf("feature %q already exists", f.Name)
	}
	s.Features = append(s.Features, f)
	return nil
}

// RemoveFeature deletes the feature and reports whether it was present.
func (s *Spec) RemoveFeature(name string) bool {
	want, err := naming.Parse(name)
	if err != nil {
		return false
	}
	before := len(s.Features)
	s.Features = slices.DeleteFunc(s.Features, func(f FeatureSpec) bool {
		n, err := naming.Parse(f.Name)
		return err == nil && n.Snake == want.Snake
	})
	return len(s.Features) != before
}

// HasCapability reports whether the capability is enabled.
func (s *Spec) HasCapability(name string) bool {
	return slices.Contains(s.Capabilities, name)
}

// Clone returns a deep copy, so a plan can hold the new intent while the
// loaded one stays untouched until apply succeeds.
func (s *Spec) Clone() *Spec {
	c := *s
	c.Capabilities = slices.Clone(s.Capabilities)
	c.Features = make([]FeatureSpec, len(s.Features))
	for i, f := range s.Features {
		c.Features[i] = FeatureSpec{Name: f.Name, Fields: slices.Clone(f.Fields), Transports: slices.Clone(f.Transports)}
	}
	return &c
}
