package render_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/esubalew-Sintie/gox/internal/render"
	"github.com/esubalew-Sintie/gox/internal/templates"
)

func store(files map[string]string) *templates.Store {
	m := fstest.MapFS{}
	for k, v := range files {
		m[k] = &fstest.MapFile{Data: []byte(v)}
	}
	return templates.NewFromFS(m, nil)
}

func TestRenderFuncsAndMissingKey(t *testing.T) {
	r := render.New(store(map[string]string{
		"t.tmpl":   `{{ pascal .Name }} {{ camel .Name }} {{ snake .Name }} {{ plural (snake .Name) }} {{ quote .Name }} {{ backtick }}`,
		"bad.tmpl": `{{ .Missing }}`,
	}), "example.com/x")
	out, err := r.Render("t.tmpl", map[string]string{"Name": "vehicleGroup"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "VehicleGroup vehicleGroup vehicle_group vehicle_groups \"vehicleGroup\" `" {
		t.Fatalf("got %q", out)
	}
	if _, err := r.Render("bad.tmpl", map[string]string{}); err == nil || !strings.Contains(err.Error(), "bad.tmpl") {
		t.Fatalf("missing key must fail and name the template: %v", err)
	}
	if _, err := r.Render("nope.tmpl", nil); err == nil {
		t.Fatal("unknown template must fail")
	}
}

func TestRenderGoFormatsAndFixesImports(t *testing.T) {
	r := render.New(store(map[string]string{
		"main.go.tmpl": "package main\nimport (\n\"os\"\n\"fmt\"\n)\nfunc main(){fmt.Println(\"{{ .Msg }}\"); _ = strings.ToUpper(\"x\")}\n",
	}), "example.com/x")
	out, err := r.RenderGo("main.go.tmpl", "cmd/main.go", map[string]string{"Msg": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, `"os"`) || !strings.Contains(s, `"strings"`) || !strings.Contains(s, "func main() {") {
		t.Fatalf("formatting: %s", s)
	}
	if _, err := render.Format("x.go", []byte("package x\nfunc {")); err == nil {
		t.Fatal("invalid Go must fail")
	}
}
