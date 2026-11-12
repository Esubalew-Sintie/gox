package manifest_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/manifest"
)

func validSpec() *manifest.Spec {
	return &manifest.Spec{
		Version: 2, Module: "github.com/acme/fleet", Arch: "hexagonal", HTTP: "chi", DB: "postgres",
		Capabilities: []string{"redis"},
		Features:     []manifest.FeatureSpec{{Name: "fleet", Fields: []string{"name:string"}, Transports: []string{"http"}}},
	}
}

func TestSpecRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if _, err := manifest.LoadSpec(dir); !errors.Is(err, manifest.ErrSpecNotFound) {
		t.Fatalf("missing spec: %v", err)
	}
	s := validSpec()
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := manifest.LoadSpec(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Module != s.Module || len(got.Features) != 1 || got.Features[0].Fields[0] != "name:string" {
		t.Fatalf("round trip lost data: %+v", got)
	}
}

func TestSpecValidateReportsEverything(t *testing.T) {
	s := &manifest.Spec{Version: 1, Module: "not a module", Arch: "onion", HTTP: "gin", DB: "oracle",
		Capabilities: []string{"redis", "redis", "nats"},
		Features:     []manifest.FeatureSpec{{Name: "fleet"}, {Name: "Fleet", Transports: []string{"carrier-pigeon"}}, {Name: ""}}}
	err := s.Validate()
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"version must be 2", "module:", "arch must be", "http must be", "db must be",
		"\"redis\" is listed twice", "\"nats\" is unknown", "duplicates \"fleet\"", "carrier-pigeon", "features[2]"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%s", want, err)
		}
	}
}

func TestSpecUnknownKeyIsAnError(t *testing.T) {
	_, err := manifest.ParseSpec([]byte("version: 2\nmodule: example.com/x\narch: hexagonal\nhttp: chi\ndb: memory\ndatabse: postgres\n"))
	if err == nil || !strings.Contains(err.Error(), "databse") {
		t.Fatalf("unknown key must fail: %v", err)
	}
}

func TestSpecFeatures(t *testing.T) {
	s := validSpec()
	if err := s.AddFeature(manifest.FeatureSpec{Name: "Fleet"}); err == nil {
		t.Fatal("duplicate by normalised name must fail")
	}
	if err := s.AddFeature(manifest.FeatureSpec{Name: "vehicle_group"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Feature("VehicleGroup"); !ok {
		t.Fatal("lookup must normalise")
	}
	clone := s.Clone()
	if !s.RemoveFeature("vehicle-group") || len(s.Features) != 1 || len(clone.Features) != 2 {
		t.Fatalf("remove: %+v clone %+v", s.Features, clone.Features)
	}
}

func TestManifestRoundTripAndOrder(t *testing.T) {
	dir := t.TempDir()
	if _, err := manifest.Load(dir); !errors.Is(err, manifest.ErrNotFound) {
		t.Fatalf("missing manifest: %v", err)
	}
	m := manifest.New("0.2.0-test")
	for _, p := range []string{"internal/service/service.go", "app/app.go", "cmd/main.go"} {
		if err := m.Set(manifest.Entry{Path: p, Class: manifest.ClassOnce, Hash: manifest.Hash([]byte(p)), Owner: "project"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Set(manifest.Entry{Path: "./app//infra.go", Class: manifest.ClassTool, Owner: "project"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(got.Files))
	for _, e := range got.Files {
		paths = append(paths, e.Path)
	}
	want := "app/app.go app/infra.go cmd/main.go internal/service/service.go"
	if strings.Join(paths, " ") != want {
		t.Fatalf("order: %v", paths)
	}
	if e, ok := got.Lookup("./app//infra.go"); !ok || e.Class != manifest.ClassTool {
		t.Fatalf("lookup normalises paths: %+v %v", e, ok)
	}
	if !got.Remove("cmd/main.go") || got.Remove("cmd/main.go") {
		t.Fatal("remove must report presence once")
	}
	if n := len(got.ByOwner("project")); n != 3 {
		t.Fatalf("by owner: %d", n)
	}
}

func TestManifestRejectsBadEntries(t *testing.T) {
	cases := map[string]string{
		"absolute":  `{"gox_version":"x","files":[{"path":"/etc/passwd","class":"tool","sha256":""}]}`,
		"escape":    `{"gox_version":"x","files":[{"path":"../x.go","class":"tool","sha256":""}]}`,
		"class":     `{"gox_version":"x","files":[{"path":"a.go","class":"magic","sha256":""}]}`,
		"duplicate": `{"gox_version":"x","files":[{"path":"a.go","class":"tool","sha256":""},{"path":"./a.go","class":"once","sha256":""}]}`,
	}
	for name, body := range cases {
		if _, err := manifest.Decode([]byte(body)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestHash(t *testing.T) {
	if manifest.Hash([]byte("a")) != "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" {
		t.Fatal("sha256 hex expected")
	}
	dir := t.TempDir()
	if _, err := manifest.HashFile(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing file must fail")
	}
}
