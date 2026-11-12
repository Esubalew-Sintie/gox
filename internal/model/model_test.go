package model_test

import (
	"strings"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/model"
	"github.com/esubalew-Sintie/gox/internal/naming"
)

func TestParseField(t *testing.T) {
	cases := []struct {
		in       string
		wantGo   string
		wantNull bool
		wantErr  string
	}{
		{in: "name:string", wantGo: "string"},
		{in: "capacity:int", wantGo: "int"},
		{in: "opened_at:time?", wantGo: "*time.Time", wantNull: true},
		{in: "ownerId:uuid", wantGo: "string"},
		{in: "closed_at:time:null", wantGo: "*time.Time", wantNull: true},
		{in: "closed_at:time:maybe", wantErr: "unknown suffix"},
		{in: "name", wantErr: "must be name:type"},
		{in: "name:decimal", wantErr: "unknown type"},
		{in: "id:int", wantErr: "generated on every entity"},
		{in: "created_at:time", wantErr: "generated on every entity"},
		{in: "na me:string", wantGo: "string"}, // naming splits words; "na me" is two words, valid
	}
	for _, c := range cases {
		f, err := model.ParseField(c.in)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%q: want error %q, got %v", c.in, c.wantErr, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if f.GoType() != c.wantGo || f.Nullable != c.wantNull {
			t.Errorf("%q: got %s nullable=%v", c.in, f.GoType(), f.Nullable)
		}
	}
	f, _ := model.ParseField("ownerId:uuid")
	if f.Name.Snake != "owner_id" || f.Name.Pascal != "OwnerId" || f.String() != "owner_id:uuid" {
		t.Fatalf("naming: %+v %s", f.Name, f.String())
	}
}

func TestParseFieldsRejectsDuplicates(t *testing.T) {
	_, err := model.ParseFields([]string{"name:string", "Name:text", "x:bogus"})
	if err == nil || !strings.Contains(err.Error(), "declared twice") || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("got %v", err)
	}
}

func TestFromSpec(t *testing.T) {
	spec := &manifest.Spec{Version: 2, Module: "github.com/acme/fleet-service", Arch: "hexagonal", HTTP: "chi", DB: "postgres",
		Capabilities: []string{"redis"},
		Features:     []manifest.FeatureSpec{{Name: "fleet", Fields: []string{"name:string", "opened_at:time?"}}}}
	p, err := model.FromSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "fleet-service" || !p.Has("redis") || p.Has("kafka") {
		t.Fatalf("project: %+v", p)
	}
	f, ok := p.Feature("Fleet")
	if !ok || len(f.Fields) != 2 || !f.HasTransport("http") || f.Imports()[0] != "time" {
		t.Fatalf("feature: %+v %v", f, ok)
	}
	spec.Features[0].Fields = append(spec.Features[0].Fields, "id:int")
	if _, err := model.FromSpec(spec); err == nil || !strings.Contains(err.Error(), "feature \"fleet\"") {
		t.Fatalf("field errors must name the feature: %v", err)
	}
}

func TestLayoutPaths(t *testing.T) {
	l := model.Hexagonal
	name, _ := naming.Parse("fleet")
	f := model.Feature{Name: *name}
	checks := map[string]string{
		l.FeatureEntityFile(f):                "internal/domain/entity/fleet.go",
		l.FeatureServiceDir(f):                "internal/service/fleet",
		l.FeatureHandlerDir(f):                "internal/handler/http/fleet",
		l.FeatureRepositoryDir(f, "postgres"): "internal/storage/postgres/fleet",
		l.FeatureCacheFile(f):                 "internal/storage/redis/fleet_cache.go",
		l.FeatureMemstoreFile(f):              "internal/testutil/memstore/fleet.go",
		l.ErrorsFile():                        "internal/domain/errors.go",
		l.MiddlewareDir():                     "internal/handler/http/middleware",
		l.DBDir("postgres"):                   "internal/storage/postgres",
	}
	for got, want := range checks {
		if got != want {
			t.Errorf("got %s want %s", got, want)
		}
	}
	if len(l.Codemods()) != 7 || l.Codemods()[6] != model.ConfigFile {
		t.Fatalf("codemods: %v", l.Codemods())
	}
	up, down := model.Migration(2, "create_fleets")
	if up != "migrations/000002_create_fleets.up.sql" || down != "migrations/000002_create_fleets.down.sql" {
		t.Fatalf("migration: %s %s", up, down)
	}
	if model.ImportPath("github.com/acme/fleet", l.EntityDir()) != "github.com/acme/fleet/internal/domain/entity" {
		t.Fatal("import path")
	}
	if model.FeatureOwner(f) != "feature:fleet" {
		t.Fatal("owner tag")
	}
}
