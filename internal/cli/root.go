// Package cli is the cobra command tree of gox. Commands load the project,
// build a plan, print it, and hand it to apply; every mutating command
// honours --dry-run, --force, --adopt and --allow-dirty.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/esubalew-Sintie/gox/internal/apply"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/transcript"
	"github.com/esubalew-Sintie/gox/internal/version"
)

type globalFlags struct {
	dir           string
	dryRun        bool
	yes           bool
	force         bool
	adopt         bool
	allowDirty    bool
	noInteractive bool
	verbose       bool
	noColor       bool
}

// NewRoot builds the command tree. Tests call it with their own args and
// output streams.
func NewRoot() *cobra.Command {
	g := &globalFlags{}
	root := &cobra.Command{
		Use:           "gox",
		Short:         "Generate Go services with explicit wiring",
		Long:          "gox creates a Go service in a hexagonal layout and adds features with every layer and all of their wiring. Everything it writes is ordinary Go that belongs to you.",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.StringVarP(&g.dir, "chdir", "C", ".", "run as if started in this directory")
	pf.BoolVar(&g.dryRun, "dry-run", false, "print the plan and the diffs without writing anything")
	pf.BoolVarP(&g.yes, "yes", "y", false, "answer yes to confirmations")
	pf.BoolVar(&g.force, "force", false, "overwrite edited tool files and delete edited files")
	pf.BoolVar(&g.adopt, "adopt", false, "keep edited tool files as yours instead of overwriting them")
	pf.BoolVar(&g.allowDirty, "allow-dirty", false, "run with uncommitted changes in git")
	pf.BoolVar(&g.noInteractive, "no-interactive", false, "never prompt; fail when a confirmation is needed")
	pf.BoolVarP(&g.verbose, "verbose", "v", false, "print a diff for every change")
	pf.BoolVar(&g.noColor, "no-color", false, "plain output")

	root.AddCommand(newCmd(g), genCmd(g), destroyCmd(g), syncCmd(g), checkCmd(g), doctorCmd(g), templatesCmd(g))
	return root
}

// Execute runs gox with os.Args and returns the exit code.
func Execute() int {
	root := NewRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func (g *globalFlags) writer(cmd *cobra.Command) *transcript.Writer {
	color := !g.noColor
	if f, ok := cmd.OutOrStdout().(*os.File); ok {
		color = color && term.IsTerminal(int(f.Fd()))
	} else {
		color = false
	}
	return transcript.New(cmd.OutOrStdout(), color)
}

// run prints the plan, stops on conflicts or dry run, asks for confirmation
// when confirm is not empty, and applies.
func (g *globalFlags) run(cmd *cobra.Command, root string, p *plan.Plan, opts apply.Options, confirm string) error {
	out := g.writer(cmd)
	printPlan(out, p, g.dryRun || g.verbose)

	if conflicts := p.Conflicts(); len(conflicts) > 0 {
		out.Error("%d file(s) were edited since gox wrote them; nothing was written", len(conflicts))
		return apply.ErrConflicts
	}
	if g.dryRun {
		out.Note("dry run: nothing written")
		return nil
	}
	if confirm != "" && !g.yes {
		if g.noInteractive {
			return errors.New("confirmation needed; pass --yes")
		}
		ok, err := ask(cmd.InOrStdin(), cmd.OutOrStdout(), confirm)
		if err != nil {
			return err
		}
		if !ok {
			out.Note("aborted")
			return nil
		}
	}

	applier := &apply.Applier{Root: root}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := applier.Apply(ctx, p, opts); err != nil {
		var verr *apply.VerifyError
		if errors.As(err, &verr) {
			out.Error("%s failed; every file was restored", verr.Command)
			out.Raw(verr.Output)
			return errors.New("generated code did not verify")
		}
		return err
	}
	changes := p.Changes()
	switch {
	case opts.SkipVerify:
		out.Note("%d change(s) written", changes)
	default:
		out.Note("%d change(s) written and verified with go build and go vet", changes)
	}
	return nil
}

func printPlan(out *transcript.Writer, p *plan.Plan, withDiff bool) {
	for _, s := range p.Steps {
		out.Step(string(s.Op), s.Path, s.Reason)
		if withDiff && (s.Writes() || s.Op == plan.OpDelete) {
			out.Raw(p.Diff(s))
		}
	}
	for _, s := range p.Conflicts() {
		if withDiff {
			out.Raw(p.Diff(s))
		}
	}
}

func ask(in io.Reader, out io.Writer, question string) (bool, error) {
	fmt.Fprintf(out, "%s [y/N] ", question)
	var answer string
	if _, err := fmt.Fscanln(in, &answer); err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "unexpected newline") {
		return false, fmt.Errorf("read answer: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}
