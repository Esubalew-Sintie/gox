package db_test

import (
	"strings"
	"testing"

	"github.com/esubalew-Sintie/gox/internal/db"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/model"
)

func fleet(t *testing.T) model.Feature {
	t.Helper()
	f, err := model.FeatureFromSpec(manifest.FeatureSpec{Name: "fleet", Fields: []string{"name:string", "capacity:int", "opened_at:time?"}})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPostgresStatements(t *testing.T) {
	d, ok := db.For("postgres")
	if !ok || !d.SQL || d.Package != "postgres" {
		t.Fatalf("dialect: %+v %v", d, ok)
	}
	s := d.Statements(fleet(t))
	want := map[string]string{
		"insert": "INSERT INTO fleets (id, name, capacity, opened_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)",
		"select": "SELECT id, name, capacity, opened_at, created_at, updated_at FROM fleets WHERE id = $1",
		"lock":   "SELECT id, name, capacity, opened_at, created_at, updated_at FROM fleets WHERE id = $1 FOR UPDATE",
		"list":   "SELECT id, name, capacity, opened_at, created_at, updated_at FROM fleets ORDER BY created_at, id LIMIT $1 OFFSET $2",
		"update": "UPDATE fleets SET name = $1, capacity = $2, opened_at = $3, updated_at = $4 WHERE id = $5",
		"delete": "DELETE FROM fleets WHERE id = $1",
	}
	got := map[string]string{"insert": s.Insert, "select": s.Select, "lock": s.SelectForUpdate, "list": s.List, "update": s.Update, "delete": s.Delete}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s:\n got %s\nwant %s", k, got[k], w)
		}
	}
	for _, line := range []string{"id TEXT PRIMARY KEY", "capacity BIGINT NOT NULL", "opened_at TIMESTAMPTZ NULL", "updated_at TIMESTAMPTZ NOT NULL"} {
		if !strings.Contains(s.CreateTable, line) {
			t.Errorf("create table missing %q:\n%s", line, s.CreateTable)
		}
	}
	if s.DropTable != "DROP TABLE IF EXISTS fleets;" || len(s.Columns) != 6 {
		t.Errorf("drop/columns: %s %v", s.DropTable, s.Columns)
	}
}

func TestOtherDialects(t *testing.T) {
	f := fleet(t)
	my, _ := db.For("mysql")
	if s := my.Statements(f); s.Update != "UPDATE fleets SET name = ?, capacity = ?, opened_at = ?, updated_at = ? WHERE id = ?" || !strings.Contains(s.CreateTable, "name VARCHAR(255) NOT NULL") || !strings.HasSuffix(s.SelectForUpdate, " FOR UPDATE") {
		t.Errorf("mysql: %+v", s)
	}
	lite, _ := db.For("sqlite")
	if s := lite.Statements(f); s.SelectForUpdate != s.Select || !strings.Contains(s.CreateTable, "capacity INTEGER NOT NULL") || !strings.Contains(s.CreateTable, "opened_at DATETIME NULL") {
		t.Errorf("sqlite: %+v", s)
	}
	mongo, ok := db.For("mongo")
	if !ok || mongo.SQL || mongo.Package != "mongo" {
		t.Errorf("mongo: %+v", mongo)
	}
	mem, ok := db.For("memory")
	if !ok || mem.SQL || mem.Package != "" {
		t.Errorf("memory: %+v", mem)
	}
	if _, ok := db.For("oracle"); ok {
		t.Error("unknown dialect must not resolve")
	}
}
