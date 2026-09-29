package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/filesystem"
)

func TestExecutorRollsBackCreatedFilesAfterFailure(t *testing.T) {
	projectPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectPath, "blocked"), []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}

	plan := NewGenerationPlan()
	if err := plan.AddCreate("created.go", []byte("package example")); err != nil {
		t.Fatal(err)
	}
	if err := plan.AddCreate(filepath.Join("blocked", "cannot-write.go"), []byte("package example")); err != nil {
		t.Fatal(err)
	}

	executor := NewExecutor(filesystem.New())
	if err := executor.Execute(projectPath, plan); err == nil {
		t.Fatal("expected execution failure")
	}

	if _, err := os.Stat(filepath.Join(projectPath, "created.go")); !os.IsNotExist(err) {
		t.Fatalf("created file remained after rollback: %v", err)
	}
}
