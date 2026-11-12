// Package plan decides what a gox command would do to a project before
// anything is written. It compares desired files against the disk and the
// manifest and produces one step per path: create, regen, identical, keep,
// adopt, delete, missing, skip or conflict. The result can be printed as a
// dry run or handed to apply.
package plan

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/fsx"
	"github.com/esubalew-Sintie/gox/internal/manifest"
)

// Op is what a step does.
type Op string

// The step operations.
const (
	OpCreate    Op = "create"    // file does not exist; it is written
	OpRegen     Op = "regen"     // tool file is rewritten with new content
	OpModify    Op = "modify"    // codemod file gets gox's declarations appended or removed
	OpIdentical Op = "identical" // tool file already has the desired content
	OpKeep      Op = "keep"      // once or codemod file exists, or was removed by the developer; left alone
	OpAdopt     Op = "adopt"     // edited tool file becomes a once file; content untouched
	OpDelete    Op = "delete"    // file removed
	OpMissing   Op = "missing"   // deletion target already gone; manifest entry dropped
	OpSkip      Op = "skip"      // deletion target was edited; left in place and handed to the developer
	OpConflict  Op = "conflict"  // tool file was edited or is unknown; nothing happens until --force or --adopt
)

// Desired is one file a command wants to exist with the given content.
type Desired struct {
	Path     string
	Class    manifest.Class
	Template string
	Owner    string
	Content  []byte
	Decls    []string // codemod files: the declarations gox owns after this change
}

// Step is one planned operation.
type Step struct {
	Op      Op
	Path    string
	Class   manifest.Class
	Content []byte // content to write for create and regen
	Old     []byte // content on disk when the file exists
	Reason  string
}

// Writes reports whether the step writes a file.
func (s Step) Writes() bool { return s.Op == OpCreate || s.Op == OpRegen || s.Op == OpModify }

// Touches reports whether the step changes the disk.
func (s Step) Touches() bool { return s.Writes() || s.Op == OpDelete }

// Options steer how edited files are handled.
type Options struct {
	Force   bool   // overwrite edited tool files; delete edited deletion targets
	Adopt   bool   // convert edited tool files to once files instead of overwriting
	Version string // gox version recorded in new manifest entries
}

// Plan is the full set of steps plus the manifest and spec that apply writes
// when every step succeeded.
type Plan struct {
	Root     string
	Steps    []Step
	Manifest *manifest.Manifest
	Spec     *manifest.Spec // nil when the intent does not change
}

// Conflicts returns the conflict steps.
func (p *Plan) Conflicts() []Step {
	var out []Step
	for _, s := range p.Steps {
		if s.Op == OpConflict {
			out = append(out, s)
		}
	}
	return out
}

// Changes counts the steps that change the disk.
func (p *Plan) Changes() int {
	n := 0
	for _, s := range p.Steps {
		if s.Touches() {
			n++
		}
	}
	return n
}

// Diff renders a unified diff for a step, empty when nothing changes.
func (p *Plan) Diff(s Step) string {
	switch s.Op {
	case OpCreate:
		return Unified(s.Path, nil, s.Content)
	case OpRegen, OpModify, OpConflict:
		if s.Content == nil {
			return ""
		}
		return Unified(s.Path, s.Old, s.Content)
	case OpDelete:
		return Unified(s.Path, s.Old, nil)
	default:
		return ""
	}
}

// Planner builds plans for one project root.
type Planner struct {
	root     string
	manifest *manifest.Manifest
	opts     Options
}

// New returns a planner. The manifest is not modified; the plan carries a copy.
func New(root string, m *manifest.Manifest, opts Options) *Planner {
	return &Planner{root: root, manifest: m, opts: opts}
}

// Build plans the desired files and the removals.
func (p *Planner) Build(desired []Desired, remove []string) (*Plan, error) {
	result := &Plan{Root: p.root, Manifest: p.manifest.Clone()}
	seen := map[string]bool{}
	var errs []error

	for _, d := range desired {
		clean, err := manifest.CleanPath(d.Path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if seen[clean] {
			errs = append(errs, fmt.Errorf("%s is planned twice", clean))
			continue
		}
		seen[clean] = true
		if !d.Class.Valid() {
			errs = append(errs, fmt.Errorf("%s: unknown class %q", clean, d.Class))
			continue
		}
		d.Path = clean
		step, err := p.planDesired(d, result.Manifest)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		result.Steps = append(result.Steps, step)
	}

	for _, r := range remove {
		clean, err := manifest.CleanPath(r)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if seen[clean] {
			errs = append(errs, fmt.Errorf("%s is both desired and removed", clean))
			continue
		}
		seen[clean] = true
		step, err := p.planRemove(clean, result.Manifest)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		result.Steps = append(result.Steps, step)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	slices.SortFunc(result.Steps, func(a, b Step) int { return strings.Compare(a.Path, b.Path) })
	return result, nil
}

func (p *Planner) planDesired(d Desired, m *manifest.Manifest) (Step, error) {
	disk, exists, err := fsx.ReadIfExists(filepath.Join(p.root, filepath.FromSlash(d.Path)))
	if err != nil {
		return Step{}, err
	}
	entry, known := m.Lookup(d.Path)
	newEntry := manifest.Entry{
		Path: d.Path, Class: d.Class, Template: d.Template, Owner: d.Owner,
		Version: p.opts.Version, Hash: manifest.Hash(d.Content),
	}
	if d.Class == manifest.ClassCodemod {
		newEntry.Decls = d.Decls
	}
	step := Step{Path: d.Path, Class: d.Class, Content: d.Content, Old: disk}

	if d.Class == manifest.ClassCodemod {
		switch {
		case !exists:
			step.Op = OpCreate
		case bytes.Equal(disk, d.Content):
			step.Op = OpIdentical
		default:
			step.Op = OpModify
		}
		return step, m.Set(newEntry)
	}

	if d.Class != manifest.ClassTool {
		switch {
		case exists && known:
			step.Op, step.Reason = OpKeep, "yours"
		case exists:
			step.Op, step.Reason = OpKeep, "exists, not generated by gox"
		case known:
			step.Op, step.Reason = OpKeep, "removed by you; gox gen --force "+d.Path+" recreates it"
		default:
			step.Op = OpCreate
			return step, m.Set(newEntry)
		}
		return step, nil
	}

	if !exists {
		step.Op = OpCreate
		if known {
			step.Reason = "restored"
		}
		return step, m.Set(newEntry)
	}
	diskHash := manifest.Hash(disk)
	if known && diskHash == entry.Hash {
		if diskHash == newEntry.Hash {
			step.Op = OpIdentical
		} else {
			step.Op = OpRegen
		}
		return step, m.Set(newEntry)
	}
	switch {
	case p.opts.Adopt:
		adopted := newEntry
		adopted.Class = manifest.ClassOnce
		adopted.Hash = diskHash
		step.Op, step.Reason, step.Class = OpAdopt, "kept as yours (--adopt)", manifest.ClassOnce
		return step, m.Set(adopted)
	case p.opts.Force:
		step.Op, step.Reason = OpRegen, "overwritten (--force)"
		return step, m.Set(newEntry)
	case known:
		step.Op, step.Reason = OpConflict, "edited since generation; --force overwrites it, --adopt keeps it as yours"
	default:
		step.Op, step.Reason = OpConflict, "exists but was not generated by gox; --force overwrites it, --adopt keeps it as yours"
	}
	return step, nil
}

func (p *Planner) planRemove(path string, m *manifest.Manifest) (Step, error) {
	entry, known := m.Lookup(path)
	if !known {
		return Step{}, fmt.Errorf("cannot remove %s: not in the manifest", path)
	}
	if entry.Class == manifest.ClassCodemod {
		return Step{}, fmt.Errorf("cannot remove %s: codemod files are edited per declaration, not deleted", path)
	}
	disk, exists, err := fsx.ReadIfExists(filepath.Join(p.root, filepath.FromSlash(path)))
	if err != nil {
		return Step{}, err
	}
	step := Step{Path: path, Class: entry.Class, Old: disk}
	if !exists {
		step.Op, step.Reason = OpMissing, "already gone"
		m.Remove(path)
		return step, nil
	}
	if manifest.Hash(disk) == entry.Hash {
		step.Op = OpDelete
		m.Remove(path)
		return step, nil
	}
	if p.opts.Force {
		step.Op, step.Reason = OpDelete, "edited since generation, deleted anyway (--force)"
		m.Remove(path)
		return step, nil
	}
	step.Op, step.Reason = OpSkip, "edited since generation; left in place and now yours (--force deletes it)"
	m.Remove(path)
	return step, nil
}
