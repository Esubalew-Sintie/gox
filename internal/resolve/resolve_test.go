package resolve_test

import (
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/resolve"
)

// module writes a small project that mirrors the generated layout.
func module(t *testing.T, serviceNew string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                            "module example.com/t\n\ngo 1.26\n",
		"internal/config/config.go":         "package config\n\ntype Config struct {\n\tServer ServerConfig\n\tunexported int\n}\n\ntype ServerConfig struct{ Port int }\n",
		"internal/storage/repository.go":    "package storage\n\ntype FleetRepository interface{ Get() string }\n\ntype VehicleRepository interface{ Get() string }\n",
		"internal/service/service.go":       "package service\n\ntype FleetService interface{ Do() }\n",
		"internal/service/fleet/service.go": "package fleet\n\nimport (\n\t\"log/slog\"\n\n\t\"example.com/t/internal/config\"\n\t\"example.com/t/internal/storage\"\n)\n\nvar (\n\t_ = slog.Default\n\t_ config.Config\n\t_ storage.FleetRepository\n)\n\ntype Service struct{}\n\nfunc (s *Service) Do() {}\n\n" + serviceNew,
		"internal/service/broken/broken.go": "package broken\n\nfunc New() *Thing { return nil }\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func providers(t *testing.T, u *resolve.Universe) []resolve.Provider {
	t.Helper()
	logger, ok := u.Pointer("log/slog", "Logger")
	if !ok {
		t.Fatal("slog not loaded")
	}
	cfg, ok := u.Pointer("example.com/t/internal/config", "Config")
	if !ok {
		t.Fatal("config not loaded")
	}
	fleetRepo, _ := u.Named("example.com/t/internal/storage", "FleetRepository")
	vehicleRepo, _ := u.Named("example.com/t/internal/storage", "VehicleRepository")
	out := []resolve.Provider{
		{Expr: "log", Type: logger},
		{Expr: "cfg", Type: cfg},
		{Expr: "s.Fleet", Type: fleetRepo},
		{Expr: "s.Vehicle", Type: vehicleRepo},
	}
	fields, err := u.Fields("example.com/t/internal/config", "Config")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fields {
		out = append(out, resolve.Provider{Expr: "cfg." + f.Expr, Type: f.Type})
	}
	return out
}

func ctor() resolve.Constructor {
	return resolve.Constructor{
		Feature: "fleet", Layer: resolve.LayerService,
		Package: "example.com/t/internal/service/fleet", Func: "New",
		IfacePath: "example.com/t/internal/service", IfaceName: "FleetService",
	}
}

func TestResolvesEditedConstructor(t *testing.T) {
	root := module(t, "func New(repo storage.FleetRepository, server config.ServerConfig, cfg *config.Config, log *slog.Logger) *Service { return &Service{} }\n")
	u, err := resolve.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	args, err := u.Resolve(ctor(), providers(t, u))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args, ", "); got != "s.Fleet, cfg.Server, cfg, log" {
		t.Fatalf("args: %s", got)
	}
	if errs := u.PackageErrors("example.com/t/internal/service/broken"); len(errs) == 0 {
		t.Fatal("broken package must report errors")
	}
	if _, err := u.Resolve(resolve.Constructor{Package: "example.com/t/internal/service/broken", Func: "New"}, nil); err == nil || !strings.Contains(err.Error(), "does not compile") {
		t.Fatalf("broken: %v", err)
	}
}

func TestResolveErrors(t *testing.T) {
	cases := map[string]struct {
		newFunc string
		want    string
	}{
		"ambiguous": {"func New(repo interface{ Get() string }, log *slog.Logger) *Service { return &Service{} }\n", "several providers: s.Fleet, s.Vehicle"},
		"missing":   {"func New(repo storage.FleetRepository, port int) *Service { return &Service{} }\n", "has no provider"},
		"result":    {"func New() *config.Config { return nil }\n", "does not implement"},
		"two":       {"func New() (*Service, error) { return nil, nil }\n", "exactly one value"},
		"variadic":  {"func New(opts ...int) *Service { return nil }\n", "variadic"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := module(t, c.newFunc)
			u, err := resolve.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = u.Resolve(ctor(), providers(t, u))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
}

func TestLookups(t *testing.T) {
	root := module(t, "func New() *Service { return &Service{} }\n")
	u, err := resolve.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := u.Named("example.com/t/internal/storage", "Nope"); ok {
		t.Fatal("unknown name must not resolve")
	}
	if _, ok := u.Pointer("example.com/t/missing", "X"); ok {
		t.Fatal("unknown package must not resolve")
	}
	fields, err := u.Fields("example.com/t/internal/config", "Config")
	if err != nil || len(fields) != 1 || fields[0].Expr != "Server" {
		t.Fatalf("fields: %v %v", fields, err)
	}
	if _, err := u.Fields("example.com/t/internal/storage", "FleetRepository"); err == nil {
		t.Fatal("interface is not a struct")
	}
	if _, ok := u.Lookup("log/slog", "Logger"); !ok {
		t.Fatal("transitive import must be reachable")
	}
	if t2, _ := u.Named("log/slog", "Logger"); t2 == nil || t2.(*types.Named).Obj().Name() != "Logger" {
		t.Fatal("named type")
	}
}
