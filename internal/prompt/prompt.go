// Package prompt asks the questions a gox command needs when its flags leave
// a choice open. On a terminal the questions are a form drawn by
// charmbracelet/huh. When ACCESSIBLE is set, or when the input is not a file
// at all (a test buffer), they are plain lines. When the input is a file that
// is not a terminal (a pipe, /dev/null, CI) nothing is asked and every choice
// keeps its default.
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

// ErrAborted is returned when the user leaves a form with Esc or Ctrl-C.
var ErrAborted = errors.New("aborted")

// Mode is how a Prompter asks.
type Mode int

const (
	// None asks nothing: choices keep their defaults and a confirmation
	// reads one line from the input, so a piped "y" still answers it.
	None Mode = iota
	// Lines asks one plain question at a time on the output and reads the
	// answers line by line from the input.
	Lines
	// Screen draws a form on the terminal.
	Screen
)

func (m Mode) String() string {
	switch m {
	case Lines:
		return "lines"
	case Screen:
		return "screen"
	default:
		return "none"
	}
}

// Prompter asks questions on a pair of streams.
type Prompter struct {
	mode Mode
	in   io.Reader
	out  io.Writer
}

// New picks the mode from the streams: Screen when in is a terminal with a
// size and the ACCESSIBLE environment variable is empty, Lines when in is a
// terminal with ACCESSIBLE set or without a size, or is not a file at all,
// None when in is a file that is not a terminal.
func New(in io.Reader, out io.Writer) *Prompter {
	mode := Lines
	if f, ok := in.(*os.File); ok {
		fd := int(f.Fd())
		switch width, _, err := term.GetSize(fd); {
		case !term.IsTerminal(fd):
			mode = None
		case os.Getenv("ACCESSIBLE") == "" && err == nil && width > 0:
			mode = Screen
		}
	}
	return WithMode(mode, in, out)
}

// WithMode returns a prompter in the given mode.
func WithMode(mode Mode, in io.Reader, out io.Writer) *Prompter {
	if mode == Lines {
		in = &lineReader{br: bufio.NewReader(in)}
	}
	return &Prompter{mode: mode, in: in, out: out}
}

// Mode returns how the prompter asks.
func (p *Prompter) Mode() Mode { return p.mode }

// Asks reports whether choices are prompted for.
func (p *Prompter) Asks() bool { return p.mode != None }

// Confirm asks a yes/no question and returns the answer.
func (p *Prompter) Confirm(title string, def bool) (bool, error) {
	if p.mode == None {
		return readYesNo(p.in, p.out, title, def)
	}
	v := def
	if err := p.Form().Confirm(&v, title, "").Run(); err != nil {
		return false, err
	}
	return v, nil
}

// Input asks one free-text question. See Form.Input.
func (p *Prompter) Input(dst *string, title, description, placeholder string, validate func(string) error) error {
	return p.Form().Input(dst, title, description, placeholder, validate).Run()
}

// Option is one choice of a Select.
type Option struct {
	Value       string
	Description string
}

// Form is a set of questions asked together: on one screen in Screen mode,
// one after another in Lines mode, not at all in None mode.
type Form struct {
	p      *Prompter
	fields []formField
	checks []func() error
}

// formField is one question with the description printed above it in Lines
// mode, where huh shows only titles.
type formField struct {
	field       huh.Field
	description string
}

// Form starts an empty form.
func (p *Prompter) Form() *Form { return &Form{p: p} }

// Input adds a free-text question. dst holds the default and receives the
// answer; placeholder is shown in Screen mode when the default is empty.
// validate, when not nil, rejects answers; in Lines mode an empty line keeps
// the default.
func (f *Form) Input(dst *string, title, description, placeholder string, validate func(string) error) *Form {
	label := strings.ToLower(title)
	if f.p.mode == Lines {
		title = withHint(title, *dst)
	}
	in := huh.NewInput().Title(title).Description(description).Placeholder(placeholder).Value(dst)
	if validate != nil {
		def := *dst
		in.Validate(func(s string) error {
			if f.p.mode == Lines && s == "" && def != "" {
				return nil
			}
			return validate(s)
		})
		f.checks = append(f.checks, func() error {
			if err := validate(*dst); err != nil {
				return fmt.Errorf("%s: %w", label, err)
			}
			return nil
		})
	}
	f.fields = append(f.fields, formField{in, description})
	return f
}

// Select adds a single-choice question. dst holds the default value and
// receives the chosen one.
func (f *Form) Select(dst *string, title, description string, options []Option) *Form {
	if f.p.mode == Lines {
		title = withHint(title, *dst)
	}
	width := 0
	for _, o := range options {
		width = max(width, len(o.Value))
	}
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		label := o.Value
		if o.Description != "" {
			label = fmt.Sprintf("%-*s  %s", width, o.Value, o.Description)
		}
		opts[i] = huh.NewOption(label, o.Value)
	}
	sel := huh.NewSelect[string]().Title(title).Description(description).Options(opts...).Value(dst)
	f.fields = append(f.fields, formField{sel, description})
	return f
}

// Confirm adds a yes/no question. dst holds the default and receives the
// answer.
func (f *Form) Confirm(dst *bool, title, description string) *Form {
	c := huh.NewConfirm().Title(title).Description(description).Affirmative("Yes").Negative("No").Value(dst)
	f.fields = append(f.fields, formField{c, description})
	return f
}

// Run asks the questions. It returns ErrAborted when the user leaves the
// form, and the first validation error when an answer is still invalid
// afterwards, which happens when the input ends early.
func (f *Form) Run() error {
	if len(f.fields) == 0 || f.p.mode == None {
		return nil
	}
	if f.p.mode == Lines {
		for _, ff := range f.fields {
			if ff.description != "" {
				fmt.Fprintln(f.p.out, ff.description)
			}
			form := huh.NewForm(huh.NewGroup(ff.field)).WithAccessible(true).WithInput(f.p.in).WithOutput(f.p.out)
			if err := form.Run(); err != nil {
				return err
			}
		}
	} else {
		fields := make([]huh.Field, len(f.fields))
		for i, ff := range f.fields {
			fields[i] = ff.field
		}
		if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return ErrAborted
			}
			return err
		}
	}
	for _, check := range f.checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

// withHint is the Lines-mode title: the default in brackets and a colon,
// which huh's single-line prompts do not add themselves.
func withHint(title, hint string) string {
	if hint != "" {
		title += " [" + hint + "]"
	}
	return title + ":"
}

func readYesNo(in io.Reader, out io.Writer, question string, def bool) (bool, error) {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	fmt.Fprintf(out, "%s %s ", question, hint)
	var answer string
	if _, err := fmt.Fscanln(in, &answer); err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "unexpected newline") {
		return false, fmt.Errorf("read answer: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return def, nil
	}
}

// lineReader hands out at most one line per Read. huh reads every answer
// through a fresh bufio.Scanner, which would otherwise swallow the lines
// meant for the questions after it.
type lineReader struct {
	br      *bufio.Reader
	pending []byte
	err     error
}

func (l *lineReader) Read(p []byte) (int, error) {
	if len(l.pending) == 0 {
		if l.err != nil {
			return 0, l.err
		}
		line, err := l.br.ReadBytes('\n')
		l.pending, l.err = line, err
		if len(line) == 0 {
			return 0, err
		}
	}
	n := copy(p, l.pending)
	l.pending = l.pending[n:]
	return n, nil
}
