// Package transcript is the console output of gox commands: one line per
// planned step in the style of Rails generators, plus notes and warnings.
package transcript

import (
	"fmt"
	"io"
	"strings"
)

// Writer prints a transcript.
type Writer struct {
	out   io.Writer
	color bool
}

// New returns a writer. Color adds ANSI codes.
func New(out io.Writer, color bool) *Writer {
	return &Writer{out: out, color: color}
}

const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiBold   = "\033[1m"
)

// Step prints one operation line: the verb right-aligned in a column, the
// path, and an optional reason in parentheses.
func (w *Writer) Step(verb, path, reason string) {
	line := fmt.Sprintf("%12s  %s", verb, path)
	if reason != "" {
		line += "  (" + reason + ")"
	}
	w.println(w.paint(verb, line))
}

// Note prints an informational line.
func (w *Writer) Note(format string, args ...any) {
	w.println(fmt.Sprintf(format, args...))
}

// Warn prints a warning line.
func (w *Writer) Warn(format string, args ...any) {
	w.println(w.wrap(ansiYellow, "warning: "+fmt.Sprintf(format, args...)))
}

// Error prints an error line.
func (w *Writer) Error(format string, args ...any) {
	w.println(w.wrap(ansiRed+ansiBold, "error: "+fmt.Sprintf(format, args...)))
}

// Raw prints text as is, for diffs and tool output.
func (w *Writer) Raw(text string) {
	if text == "" {
		return
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	_, _ = io.WriteString(w.out, text)
}

func (w *Writer) println(s string) {
	_, _ = fmt.Fprintln(w.out, s)
}

func (w *Writer) paint(verb, line string) string {
	switch verb {
	case "create":
		return w.wrap(ansiGreen, line)
	case "regen", "modify", "adopt":
		return w.wrap(ansiYellow, line)
	case "delete":
		return w.wrap(ansiRed, line)
	case "conflict":
		return w.wrap(ansiRed+ansiBold, line)
	case "identical", "keep", "missing":
		return w.wrap(ansiDim, line)
	default:
		return line
	}
}

func (w *Writer) wrap(code, s string) string {
	if !w.color {
		return s
	}
	return code + s + ansiReset
}
