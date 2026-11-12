package prompt

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func lines(t *testing.T, input string) (*Prompter, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	return WithMode(Lines, strings.NewReader(input), &out), &out
}

func TestNewPicksMode(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	if got := New(devNull, io.Discard).Mode(); got != None {
		t.Fatalf("file that is not a terminal: mode %s, want none", got)
	}
	if got := New(strings.NewReader(""), io.Discard).Mode(); got != Lines {
		t.Fatalf("reader that is not a file: mode %s, want lines", got)
	}
}

func TestFormReadsOneAnswerPerQuestion(t *testing.T) {
	p, out := lines(t, "example.com/fleet\n1\ny\n")
	module, db, git := "fleet", "memory", false
	err := p.Form().
		Input(&module, "Module path", "", "", nil).
		Select(&db, "Database", "", []Option{{Value: "postgres", Description: "pgx"}, {Value: "memory"}}).
		Confirm(&git, "Run git init?", "").
		Run()
	if err != nil {
		t.Fatal(err)
	}
	if module != "example.com/fleet" || db != "postgres" || !git {
		t.Fatalf("got module=%q db=%q git=%v", module, db, git)
	}
	for _, want := range []string{"Module path [fleet]:", "Database [memory]:", "1. postgres  pgx", "2. memory", "Run git init? [y/N]"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestFormEmptyLinesKeepDefaults(t *testing.T) {
	p, _ := lines(t, "\n\n\n")
	module, db, git := "fleet", "memory", true
	err := p.Form().
		Input(&module, "Module path", "", "", func(s string) error {
			if s == "" {
				return errors.New("empty")
			}
			return nil
		}).
		Select(&db, "Database", "", []Option{{Value: "postgres"}, {Value: "memory"}}).
		Confirm(&git, "Run git init?", "").
		Run()
	if err != nil {
		t.Fatal(err)
	}
	if module != "fleet" || db != "memory" || !git {
		t.Fatalf("got module=%q db=%q git=%v", module, db, git)
	}
}

func TestFormRejectsThenAccepts(t *testing.T) {
	p, out := lines(t, "bad name\nfleet\n")
	var name string
	validate := func(s string) error {
		if strings.Contains(s, " ") {
			return errors.New("no spaces")
		}
		return nil
	}
	if err := p.Input(&name, "Feature name", "", "", validate); err != nil {
		t.Fatal(err)
	}
	if name != "fleet" {
		t.Fatalf("name = %q", name)
	}
	if !strings.Contains(out.String(), "no spaces") {
		t.Fatalf("validation error not shown:\n%s", out.String())
	}
}

func TestFormFailsWhenInputEndsBeforeValidAnswer(t *testing.T) {
	p, _ := lines(t, "")
	var name string
	err := p.Input(&name, "Feature name", "", "", func(s string) error {
		if s == "" {
			return errors.New("a name is needed")
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "feature name: a name is needed") {
		t.Fatalf("err = %v", err)
	}
}

func TestConfirmWithoutPrompts(t *testing.T) {
	cases := []struct {
		input string
		def   bool
		want  bool
	}{
		{"y\n", false, true},
		{"yes\n", false, true},
		{"n\n", true, false},
		{"\n", true, true},
		{"", false, false},
	}
	for _, c := range cases {
		var out bytes.Buffer
		p := WithMode(None, strings.NewReader(c.input), &out)
		got, err := p.Confirm("Continue?", c.def)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("input %q default %v: got %v", c.input, c.def, got)
		}
		if !strings.Contains(out.String(), "Continue? [") {
			t.Errorf("question not printed: %q", out.String())
		}
	}
}

func TestNoneModeSkipsForms(t *testing.T) {
	p := WithMode(None, strings.NewReader("postgres\n"), io.Discard)
	db := "memory"
	if err := p.Form().Select(&db, "Database", "", []Option{{Value: "postgres"}, {Value: "memory"}}).Run(); err != nil {
		t.Fatal(err)
	}
	if db != "memory" || p.Asks() {
		t.Fatalf("db = %q asks = %v", db, p.Asks())
	}
}

func TestLineReaderSplitsLines(t *testing.T) {
	r := &lineReader{br: bufioReader("ab\ncd")}
	buf := make([]byte, 16)
	n, err := r.Read(buf)
	if err != nil || string(buf[:n]) != "ab\n" {
		t.Fatalf("first read: %q %v", buf[:n], err)
	}
	n, err = r.Read(buf)
	if err != nil || string(buf[:n]) != "cd" {
		t.Fatalf("second read: %q %v", buf[:n], err)
	}
	if _, err = r.Read(buf); !errors.Is(err, io.EOF) {
		t.Fatalf("third read: %v", err)
	}
}

func bufioReader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }
