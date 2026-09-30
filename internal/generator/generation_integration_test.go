package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/architecture"
	"github.com/esubalew-Sintie/gox/internal/config"
	"github.com/esubalew-Sintie/gox/internal/filesystem"
	"github.com/esubalew-Sintie/gox/internal/injector"
	"github.com/esubalew-Sintie/gox/internal/renderer"
)

func TestGeneratedProjectBuildsForEveryLayout(t *testing.T) {
	templates, err := renderer.LoadTemplates(os.DirFS("../../initiator/templates"))
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	fs := filesystem.New()
	cfg := config.NewConfigManager(fs)
	projectGen := NewProjectGenerator(fs, cfg, renderer.New(templates))
	moduleGen := NewModuleGenerator(fs, cfg, renderer.New(templates), injector.New())

	for _, layout := range []string{architecture.LayeredREST, architecture.FeatureFirst} {
		t.Run(layout, func(t *testing.T) {
			projectName := "sample-" + layout
			projectPath := filepath.Join(".", projectName)
			modulePath := "example.com/" + projectName

			if err := projectGen.Generate(ProjectOptions{
				Name: projectName, Module: modulePath, Layout: layout,
			}); err != nil {
				t.Fatalf("generate project: %v", err)
			}

			if err := moduleGen.Generate(projectPath, ModuleOptions{Name: "account"}); err != nil {
				t.Fatalf("generate account: %v", err)
			}
			if err := moduleGen.Generate(projectPath, ModuleOptions{Name: "profile"}); err != nil {
				t.Fatalf("generate profile: %v", err)
			}

			if err := runCommand(projectPath, "go", "test", "./..."); err != nil {
				t.Fatalf("test generated project: %v", err)
			}

			if err := moduleGen.Remove(projectPath, "account", false); err != nil {
				t.Fatalf("remove module: %v", err)
			}
			if err := runCommand(projectPath, "go", "test", "./..."); err != nil {
				t.Fatalf("test project after removal: %v", err)
			}
		})
	}
}

func TestGeneratedProjectBuildsForSelectedRouter(t *testing.T) {
	templates, err := renderer.LoadTemplates(os.DirFS("../../initiator/templates"))
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	fs := filesystem.New()
	cfg := config.NewConfigManager(fs)
	projectGen := NewProjectGenerator(fs, cfg, renderer.New(templates))
	moduleGen := NewModuleGenerator(fs, cfg, renderer.New(templates), injector.New())

	for _, httpRouter := range []string{"chi", "gin"} {
		t.Run(httpRouter, func(t *testing.T) {
			projectName := "sample-" + httpRouter
			projectPath := filepath.Join(".", projectName)

			if err := projectGen.Generate(ProjectOptions{
				Name:   projectName,
				Module: "example.com/" + projectName,
				Layout: architecture.LayeredREST,
				HTTP:   httpRouter,
			}); err != nil {
				t.Fatalf("generate project: %v", err)
			}
			if err := moduleGen.Generate(projectPath, ModuleOptions{Name: "account"}); err != nil {
				t.Fatalf("generate module: %v", err)
			}
			if err := runCommand(projectPath, "go", "test", "./..."); err != nil {
				t.Fatalf("test generated project: %v", err)
			}
		})
	}
}

func TestGeneratedProjectBuildsWithSelectedCapabilities(t *testing.T) {
	templates, err := renderer.LoadTemplates(os.DirFS("../../initiator/templates"))
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	fs := filesystem.New()
	cfg := config.NewConfigManager(fs)
	projectGen := NewProjectGenerator(fs, cfg, renderer.New(templates))

	projectName := "sample-capabilities"
	if err := projectGen.Generate(ProjectOptions{
		Name:     projectName,
		Module:   "example.com/" + projectName,
		Layout:   architecture.LayeredREST,
		Postgres: true,
		Mongo:    true,
		Redis:    true,
		Kafka:    true,
	}); err != nil {
		t.Fatalf("generate project: %v", err)
	}
	if err := runCommand(projectName, "go", "test", "./..."); err != nil {
		t.Fatalf("test generated project: %v", err)
	}
}
