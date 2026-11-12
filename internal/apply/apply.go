// Package apply executes a plan: it refuses on conflicts and dirty git trees,
// snapshots every file it will touch, writes through temporary files and
// renames, verifies the project with go build and go vet, and on any failure
// restores the snapshot so the project is exactly as it was.
package apply

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/fsx"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/transcript"
)

// Runner executes external commands; tests substitute it.
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) ([]byte, error)
}

// ExecRunner runs commands with os/exec.
type ExecRunner struct{}

// Run executes name with args in dir and returns the combined output.
func (ExecRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// Options steer one apply.
type Options struct {
	AllowDirty bool // skip the git cleanliness check
	SkipVerify bool // skip go build and go vet; for trees that are not Go modules yet
	Tidy       bool // run go mod tidy before verifying, for new projects and new dependencies
}

// Applier applies plans to one project root.
type Applier struct {
	Root   string
	Runner Runner
	Out    *transcript.Writer // optional
}

// Sentinel errors a caller can test for.
var (
	ErrConflicts = errors.New("plan has conflicts; resolve them or pass --force or --adopt")
	ErrDirty     = errors.New("git working tree has uncommitted changes; commit, stash, or pass --allow-dirty")
)

// VerifyError reports a failed go build or go vet, with the tool output.
type VerifyError struct {
	Command string
	Output  string
	Err     error
}

func (e *VerifyError) Error() string {
	return fmt.Sprintf("%s failed; project restored: %v\n%s", e.Command, e.Err, e.Output)
}

func (e *VerifyError) Unwrap() error { return e.Err }

// Apply executes the plan. On error the project is restored and the error
// says why. The manifest and, when present, the spec are written last.
func (a *Applier) Apply(ctx context.Context, p *plan.Plan, opts Options) (err error) {
	if len(p.Conflicts()) > 0 {
		return ErrConflicts
	}
	if a.Runner == nil {
		a.Runner = ExecRunner{}
	}
	if !opts.AllowDirty {
		if err := a.checkClean(ctx); err != nil {
			return err
		}
	}

	snap := newSnapshot(a.Root)
	for _, s := range p.Steps {
		if s.Touches() {
			if err := snap.record(s.Path); err != nil {
				return err
			}
		}
	}
	if err := snap.record(manifest.Path); err != nil {
		return err
	}
	if p.Spec != nil {
		if err := snap.record(manifest.SpecFile); err != nil {
			return err
		}
	}
	if opts.Tidy {
		for _, rel := range []string{"go.mod", "go.sum"} {
			if err := snap.record(rel); err != nil {
				return err
			}
		}
	}
	defer func() {
		if err != nil {
			if rbErr := snap.restore(); rbErr != nil {
				err = errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
			}
		}
	}()

	for _, s := range p.Steps {
		if err := a.step(s); err != nil {
			return err
		}
	}

	if !opts.SkipVerify {
		if err := a.verify(ctx, opts.Tidy); err != nil {
			return err
		}
	}

	data, err := p.Manifest.Encode()
	if err != nil {
		return err
	}
	if err := fsx.WriteAtomic(filepath.Join(a.Root, filepath.FromSlash(manifest.Path)), data, 0o644); err != nil {
		return err
	}
	if p.Spec != nil {
		if err := p.Spec.Save(a.Root); err != nil {
			return err
		}
	}
	return nil
}

func (a *Applier) step(s plan.Step) error {
	full := filepath.Join(a.Root, filepath.FromSlash(s.Path))
	switch s.Op {
	case plan.OpCreate, plan.OpRegen, plan.OpModify:
		if err := fsx.WriteAtomic(full, s.Content, 0o644); err != nil {
			return err
		}
	case plan.OpDelete:
		if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("delete %s: %w", s.Path, err)
		}
		fsx.RemoveEmptyParents(a.Root, full)
	case plan.OpIdentical, plan.OpKeep, plan.OpAdopt, plan.OpMissing, plan.OpSkip, plan.OpConflict:
		// nothing on disk
	}
	if a.Out != nil {
		a.Out.Step(string(s.Op), s.Path, s.Reason)
	}
	return nil
}

func (a *Applier) verify(ctx context.Context, tidy bool) error {
	exists, err := fsx.Exists(filepath.Join(a.Root, "go.mod"))
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	commands := [][]string{{"build", "./..."}, {"vet", "./..."}}
	if tidy {
		commands = append([][]string{{"mod", "tidy"}}, commands...)
	}
	for _, args := range commands {
		out, err := a.Runner.Run(ctx, a.Root, "go", args...)
		if err != nil {
			return &VerifyError{Command: "go " + strings.Join(args, " "), Output: string(out), Err: err}
		}
	}
	return nil
}

func (a *Applier) checkClean(ctx context.Context) error {
	if _, err := a.Runner.Run(ctx, a.Root, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil // not a repository: nothing to protect
	}
	out, err := a.Runner.Run(ctx, a.Root, "git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return fmt.Errorf("git status: %w\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		return ErrDirty
	}
	return nil
}

// snapshot remembers the state of every path apply touches so it can be put back.
type snapshot struct {
	root  string
	files map[string]*original
	order []string
	dirs  []string // directories that did not exist before, deepest last
}

type original struct {
	content []byte
	existed bool
	mode    os.FileMode
}

func newSnapshot(root string) *snapshot {
	return &snapshot{root: root, files: map[string]*original{}}
}

func (s *snapshot) record(rel string) error {
	if _, done := s.files[rel]; done {
		return nil
	}
	full := filepath.Join(s.root, filepath.FromSlash(rel))
	data, exists, err := fsx.ReadIfExists(full)
	if err != nil {
		return err
	}
	o := &original{content: data, existed: exists, mode: 0o644}
	if exists {
		if info, err := os.Stat(full); err == nil {
			o.mode = info.Mode().Perm()
		}
	}
	s.files[rel] = o
	s.order = append(s.order, rel)
	// Remember parent directories that will be created, so rollback can remove them.
	for dir := filepath.Dir(full); len(dir) > len(s.root); dir = filepath.Dir(dir) {
		exists, err := fsx.Exists(dir)
		if err != nil {
			return err
		}
		if exists {
			break
		}
		s.dirs = append(s.dirs, dir)
	}
	return nil
}

func (s *snapshot) restore() error {
	var errs []error
	for _, rel := range s.order {
		o := s.files[rel]
		full := filepath.Join(s.root, filepath.FromSlash(rel))
		if o.existed {
			if err := fsx.WriteAtomic(full, o.content, o.mode); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %s: %w", rel, err))
		}
	}
	// Deepest directories were appended last per file; remove in reverse so children go first.
	for i := len(s.dirs) - 1; i >= 0; i-- {
		_ = os.Remove(s.dirs[i]) // only succeeds when empty, which is the point
	}
	return errors.Join(errs...)
}
