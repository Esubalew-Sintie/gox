package cli_test

import (
	"strings"
	"testing"
)

// The prompt tests run new with --dry-run, which plans the whole project
// without writing or needing the network, and answer the questions through
// the command's input, which puts the prompter in line mode.

func TestNewAsksForMissingChoices(t *testing.T) {
	parent := t.TempDir()
	// name, module (empty keeps the name), database 1 = postgres, router 2 = stdlib, git n
	out := goxIn(t, "fleet-service\n\n1\n2\nn\n", "new", "--dry-run", "-C", parent)
	for _, want := range []string{
		"Project name:",
		"Empty keeps the project name.",
		"Module path:",
		"Database [memory]:",
		"1. postgres  pgx pool",
		"Router [chi]:",
		"Run git init in the new project? [y/N]",
		"module fleet-service, db postgres, http stdlib",
		"create  internal/storage/postgres/",
		"dry run: nothing written",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "warning: no --module") {
		t.Errorf("warned about the module after asking for it:\n%s", out)
	}
}

func TestNewAsksOnlyForChoicesNotGiven(t *testing.T) {
	parent := t.TempDir()
	out := goxIn(t, "y\n", "new", "fleet", "--module", "example.com/fleet", "--db", "sqlite", "--http", "chi", "--dry-run", "-C", parent)
	if !strings.Contains(out, "Run git init in the new project? [y/N]") || !strings.Contains(out, "module example.com/fleet, db sqlite, http chi") {
		t.Fatalf("output:\n%s", out)
	}
	for _, unwanted := range []string{"Project name", "Module path", "Database [", "Router ["} {
		if strings.Contains(out, unwanted) {
			t.Errorf("asked for %q although it was given:\n%s", unwanted, out)
		}
	}
}

func TestNewDefaultsSkipsQuestions(t *testing.T) {
	parent := t.TempDir()
	for _, flag := range []string{"--defaults", "--no-interactive"} {
		out := goxIn(t, "ignored\n", "new", "fleet", flag, "--dry-run", "-C", parent)
		if strings.Contains(out, "Database [") || !strings.Contains(out, "module fleet, db memory, http chi") || !strings.Contains(out, "warning: no --module given") {
			t.Fatalf("%s output:\n%s", flag, out)
		}
	}
}

func TestNewNeedsANameWithoutQuestions(t *testing.T) {
	if msg := goxFails(t, "new", "--dry-run", "-C", t.TempDir()); !strings.Contains(msg, "a project name is needed") {
		t.Fatalf("error: %s", msg)
	}
}

func TestNewRejectsBadAnswersUntilValid(t *testing.T) {
	parent := t.TempDir()
	out := goxIn(t, "bad name!\nfleet\nnot a module\n\n5\n1\n\n", "new", "--dry-run", "-C", parent)
	if !strings.Contains(out, "invalid name") || !strings.Contains(out, "malformed import path") || !strings.Contains(out, "module fleet, db memory, http chi") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestNewFailsWhenInputEndsBeforeAName(t *testing.T) {
	root := newRootWithInput("")
	root.SetArgs([]string{"new", "--dry-run", "-C", t.TempDir()})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "project name") {
		t.Fatalf("err = %v", err)
	}
}
