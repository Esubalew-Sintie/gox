package codemod_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/codemod"
)

const interfaces = `// Package storage declares the repository interfaces.
package storage

import "context"

// Ping is kept by the developer.
type Ping interface {
	Ping(ctx context.Context) error
}
`

const fleetRepo = `// FleetRepository stores fleets.
type FleetRepository interface {
	Create(ctx context.Context, f *fleet.Fleet) error
	Get(ctx context.Context, id string) (*fleet.Fleet, error)
}`

func parse(t *testing.T, src string) *codemod.File {
	t.Helper()
	f, err := codemod.Parse("x.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func formatted(t *testing.T, f *codemod.File) string {
	t.Helper()
	if err := f.Format("example.com/x"); err != nil {
		t.Fatal(err)
	}
	return string(f.Bytes())
}

func TestAppendAndRemoveDeclRoundTrips(t *testing.T) {
	f := parse(t, interfaces)
	if err := f.AddImports("example.com/x/internal/domain/fleet"); err != nil {
		t.Fatal(err)
	}
	if err := f.AppendDecl(fleetRepo); err != nil {
		t.Fatal(err)
	}
	after := formatted(t, f)
	if !strings.Contains(after, "type FleetRepository interface") || !strings.Contains(after, `"example.com/x/internal/domain/fleet"`) {
		t.Fatalf("append:\n%s", after)
	}
	if !f.HasDecl("FleetRepository") || !f.HasDecl("Ping") || len(f.DeclNames()) != 2 {
		t.Fatalf("names: %v", f.DeclNames())
	}
	// Idempotent append, conflicting append.
	if err := f.AppendDecl(fleetRepo); err != nil {
		t.Fatalf("identical append must be a no-op: %v", err)
	}
	if err := f.AppendDecl("type FleetRepository interface{ Other() }"); !errors.Is(err, codemod.ErrExists) {
		t.Fatalf("different content must be ErrExists: %v", err)
	}

	removed, err := f.RemoveDecl("FleetRepository")
	if err != nil || !removed {
		t.Fatalf("remove: %v %v", removed, err)
	}
	if got := formatted(t, f); got != interfaces {
		t.Fatalf("round trip differs:\n%s\n---\n%s", got, interfaces)
	}
	if removed, _ := f.RemoveDecl("FleetRepository"); removed {
		t.Fatal("second remove must report false")
	}
}

const catalogue = `package domain

import (
	"errors"
	"net/http"
)

// General errors.
var (
	ErrNotFound = errors.New("Data not found.")
	ErrConflict = errors.New("This data already exists.")
)

// ErrorMap maps errors to HTTP status codes.
var ErrorMap = map[error]int{
	ErrNotFound: http.StatusNotFound,
	ErrConflict: http.StatusConflict,
}

// ErrorTypeMap maps errors to error types.
var ErrorTypeMap = map[error]ErrorType{
	ErrNotFound: ErrorTypeNotFound,
	ErrConflict: ErrorTypeConflict,
}

// Lookup is unchanged by codemods.
func Lookup(err error) (int, bool) { return ErrorMap[err], true }
`

func TestCatalogueRoundTrip(t *testing.T) {
	f := parse(t, catalogue)
	block := "// Fleet errors.\nvar (\n\tErrFleetNotFound = errors.New(\"Fleet not found.\")\n\tErrFleetInvalid  = errors.New(\"Invalid fleet.\")\n)"
	if err := f.InsertDeclBefore("ErrorMap", block); err != nil {
		t.Fatal(err)
	}
	if err := f.AppendMapEntries("ErrorMap", "// Fleet", []string{"ErrFleetNotFound: http.StatusNotFound", "ErrFleetInvalid: http.StatusBadRequest"}); err != nil {
		t.Fatal(err)
	}
	if err := f.AppendMapEntries("ErrorTypeMap", "// Fleet", []string{"ErrFleetNotFound: ErrorTypeNotFound", "ErrFleetInvalid: ErrorTypeValidation"}); err != nil {
		t.Fatal(err)
	}
	after := formatted(t, f)
	for _, want := range []string{"// Fleet errors.\nvar (", "ErrFleetInvalid  = errors.New", "\t// Fleet\n\tErrFleetNotFound: http.StatusNotFound,\n\tErrFleetInvalid:  http.StatusBadRequest,\n}", "ErrFleetInvalid:  ErrorTypeValidation,\n}"} {
		if !strings.Contains(after, want) {
			t.Fatalf("missing %q in:\n%s", want, after)
		}
	}
	if idx := strings.Index(after, "// Fleet errors."); idx > strings.Index(after, "var ErrorMap") {
		t.Fatalf("block must precede ErrorMap:\n%s", after)
	}
	// Appending again changes nothing.
	if err := f.AppendMapEntries("ErrorMap", "// Fleet", []string{"ErrFleetNotFound: http.StatusNotFound"}); err != nil {
		t.Fatal(err)
	}
	if got := formatted(t, f); got != after {
		t.Fatalf("second append changed the file:\n%s", got)
	}

	// A developer adds an entry of their own to the fleet block; it must survive.
	if err := f.AppendMapEntries("ErrorMap", "", []string{"ErrFleetDeveloper: http.StatusTeapot"}); err != nil {
		t.Fatal(err)
	}

	names := []string{"ErrFleetNotFound", "ErrFleetInvalid"}
	if n, err := f.RemoveMapEntries("ErrorMap", "// Fleet", names); err != nil || n != 3 {
		t.Fatalf("remove map entries: %d %v", n, err)
	}
	if n, err := f.RemoveMapEntries("ErrorTypeMap", "// Fleet", names); err != nil || n != 3 {
		t.Fatalf("remove type entries: %d %v", n, err)
	}
	if n, err := f.RemoveVarSpecs(names); err != nil || n != 2 {
		t.Fatalf("remove specs: %d %v", n, err)
	}
	got := formatted(t, f)
	if strings.Contains(got, "Fleet errors") || strings.Contains(got, "// Fleet\n") || !strings.Contains(got, "ErrFleetDeveloper: http.StatusTeapot,") {
		t.Fatalf("after removal:\n%s", got)
	}
	// Remove the developer's entry too and the file is back to the original bytes.
	if _, err := f.RemoveMapEntries("ErrorMap", "", []string{"ErrFleetDeveloper"}); err != nil {
		t.Fatal(err)
	}
	if got := formatted(t, f); got != catalogue {
		t.Fatalf("round trip differs:\n%s\n---\n%s", got, catalogue)
	}
}

const config = `package config

import "time"

// Config is the whole configuration.
type Config struct {
	Server ServerConfig ` + "`koanf:\"server\"`" + `
}

// ServerConfig is the HTTP server.
type ServerConfig struct {
	Port    int           ` + "`koanf:\"port\"`" + `
	Timeout time.Duration ` + "`koanf:\"timeout\"`" + `
}
`

func TestStructFieldsRoundTrip(t *testing.T) {
	f := parse(t, config)
	if err := f.AppendDecl("// RedisConfig is the redis client.\ntype RedisConfig struct {\n\tAddr string `koanf:\"addr\"`\n}"); err != nil {
		t.Fatal(err)
	}
	if err := f.AddStructField("Config", "Redis RedisConfig `koanf:\"redis\"`"); err != nil {
		t.Fatal(err)
	}
	if err := f.AddStructField("Config", "Redis RedisConfig `koanf:\"redis\"`"); err != nil {
		t.Fatalf("second add must be a no-op: %v", err)
	}
	after := formatted(t, f)
	if !strings.Contains(after, "\tRedis  RedisConfig  `koanf:\"redis\"`") || !strings.Contains(after, "type RedisConfig struct") {
		t.Fatalf("add field:\n%s", after)
	}
	if removed, err := f.RemoveStructField("Config", "Redis"); err != nil || !removed {
		t.Fatalf("remove field: %v %v", removed, err)
	}
	if removed, err := f.RemoveDecl("RedisConfig"); err != nil || !removed {
		t.Fatalf("remove type: %v %v", removed, err)
	}
	if got := formatted(t, f); got != config {
		t.Fatalf("round trip differs:\n%s\n---\n%s", got, config)
	}
	if err := f.AddStructField("Nope", "X int"); !errors.Is(err, codemod.ErrNotFound) {
		t.Fatalf("unknown type: %v", err)
	}
}

func TestMethodsAndErrors(t *testing.T) {
	f := parse(t, "package p\n\ntype T struct{}\n\nfunc (t *T) M() {}\n\nfunc F() {}\n")
	if !f.HasDecl("T.M") || !f.HasDecl("F") {
		t.Fatalf("names: %v", f.DeclNames())
	}
	if _, err := codemod.Parse("bad.go", []byte("package p\nfunc {")); err == nil {
		t.Fatal("invalid source must fail")
	}
	if err := f.AppendDecl("not a decl"); err == nil {
		t.Fatal("invalid declaration must fail")
	}
	if err := f.AppendDecl("func A() {}\nfunc B() {}"); err == nil {
		t.Fatal("two declarations must fail")
	}
	if _, err := f.RemoveMapEntries("T", "", []string{"x"}); err == nil {
		t.Fatal("non-map must fail")
	}
}
