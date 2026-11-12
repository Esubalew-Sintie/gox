package transcript_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/transcript"
)

func TestStepsAlignAndColor(t *testing.T) {
	var buf bytes.Buffer
	w := transcript.New(&buf, false)
	w.Step("create", "app/app.go", "")
	w.Step("conflict", "app/infra.go", "edited since generation")
	w.Warn("one %s", "thing")
	w.Raw("diff text")
	got := buf.String()
	want := "      create  app/app.go\n    conflict  app/infra.go  (edited since generation)\nwarning: one thing\ndiff text\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}

	buf.Reset()
	transcript.New(&buf, true).Step("delete", "x.go", "")
	if !strings.HasPrefix(buf.String(), "\033[31m") || !strings.HasSuffix(buf.String(), "\033[0m\n") {
		t.Fatalf("color: %q", buf.String())
	}
}
