// Package db describes each supported database for the templates: whether
// it speaks SQL, how it writes placeholders and column types, which locking
// clause it has, and the statements a feature repository needs. The
// generator renders the statements once so every SQL template receives the
// same argument order regardless of dialect.
package db

import (
	"fmt"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/model"
)

// Kind is a database name as written in gox.yaml.
type Kind string

// The supported databases.
const (
	Memory   Kind = "memory"
	Postgres Kind = "postgres"
	MySQL    Kind = "mysql"
	SQLite   Kind = "sqlite"
	Mongo    Kind = "mongo"
)

// Dialect is what the templates need to know about a database.
type Dialect struct {
	Kind      Kind
	SQL       bool   // false for mongo and memory
	Package   string // package name under internal/storage, "" for memory
	ForUpdate string // locking clause appended to a SELECT inside a transaction
	idType    string
	timeType  string
	types     map[model.FieldType]string
	holder    func(n int) string
}

var dialects = map[Kind]Dialect{
	Memory: {Kind: Memory},
	Postgres: {
		Kind: Postgres, SQL: true, Package: "postgres", ForUpdate: " FOR UPDATE",
		idType: "TEXT", timeType: "TIMESTAMPTZ",
		types: map[model.FieldType]string{
			model.TypeString: "TEXT", model.TypeText: "TEXT", model.TypeInt: "BIGINT", model.TypeInt64: "BIGINT",
			model.TypeFloat64: "DOUBLE PRECISION", model.TypeBool: "BOOLEAN", model.TypeTime: "TIMESTAMPTZ", model.TypeUUID: "TEXT",
		},
		holder: func(n int) string { return fmt.Sprintf("$%d", n) },
	},
	MySQL: {
		Kind: MySQL, SQL: true, Package: "mysql", ForUpdate: " FOR UPDATE",
		idType: "VARCHAR(32)", timeType: "DATETIME(6)",
		types: map[model.FieldType]string{
			model.TypeString: "VARCHAR(255)", model.TypeText: "TEXT", model.TypeInt: "BIGINT", model.TypeInt64: "BIGINT",
			model.TypeFloat64: "DOUBLE", model.TypeBool: "BOOLEAN", model.TypeTime: "DATETIME(6)", model.TypeUUID: "CHAR(36)",
		},
		holder: func(int) string { return "?" },
	},
	SQLite: {
		Kind: SQLite, SQL: true, Package: "sqlite", ForUpdate: "",
		idType: "TEXT", timeType: "DATETIME",
		types: map[model.FieldType]string{
			model.TypeString: "TEXT", model.TypeText: "TEXT", model.TypeInt: "INTEGER", model.TypeInt64: "INTEGER",
			model.TypeFloat64: "REAL", model.TypeBool: "INTEGER", model.TypeTime: "DATETIME", model.TypeUUID: "TEXT",
		},
		holder: func(int) string { return "?" },
	},
	Mongo: {Kind: Mongo, Package: "mongo"},
}

// For returns the dialect for a gox.yaml db value.
func For(kind string) (Dialect, bool) {
	d, ok := dialects[Kind(kind)]
	return d, ok
}

// Statements are the SQL texts of one feature repository. Argument order is
// the same for every dialect: insert takes id, the fields, created_at and
// updated_at; update takes the fields, updated_at and the id last; list
// takes limit then offset.
type Statements struct {
	Table           string
	Columns         []string // id, the fields in order, created_at, updated_at
	Insert          string
	Select          string
	SelectForUpdate string
	List            string
	Count           string
	Update          string
	Delete          string
	CreateTable     string
	DropTable       string
}

// Statements renders the SQL of a feature for this dialect.
func (d Dialect) Statements(f model.Feature) Statements {
	table := f.Name.Plural
	cols := []string{"id"}
	for _, fld := range f.Fields {
		cols = append(cols, fld.Name.Snake)
	}
	cols = append(cols, "created_at", "updated_at")
	colList := strings.Join(cols, ", ")

	holders := make([]string, len(cols))
	for i := range cols {
		holders[i] = d.holder(i + 1)
	}

	var sets []string
	n := 1
	for _, fld := range f.Fields {
		sets = append(sets, fld.Name.Snake+" = "+d.holder(n))
		n++
	}
	sets = append(sets, "updated_at = "+d.holder(n))
	n++
	idHolder := d.holder(n)

	selectSQL := fmt.Sprintf("SELECT %s FROM %s WHERE id = %s", colList, table, d.holder(1))

	var defs []string
	defs = append(defs, fmt.Sprintf("    id %s PRIMARY KEY", d.idType))
	for _, fld := range f.Fields {
		null := " NOT NULL"
		if fld.Nullable {
			null = " NULL"
		}
		defs = append(defs, fmt.Sprintf("    %s %s%s", fld.Name.Snake, d.types[fld.Type], null))
	}
	defs = append(defs, fmt.Sprintf("    created_at %s NOT NULL", d.timeType), fmt.Sprintf("    updated_at %s NOT NULL", d.timeType))

	return Statements{
		Table:           table,
		Columns:         cols,
		Insert:          fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, colList, strings.Join(holders, ", ")),
		Select:          selectSQL,
		SelectForUpdate: selectSQL + d.ForUpdate,
		List:            fmt.Sprintf("SELECT %s FROM %s ORDER BY created_at, id LIMIT %s OFFSET %s", colList, table, d.holder(1), d.holder(2)),
		Count:           fmt.Sprintf("SELECT COUNT(*) FROM %s", table),
		Update:          fmt.Sprintf("UPDATE %s SET %s WHERE id = %s", table, strings.Join(sets, ", "), idHolder),
		Delete:          fmt.Sprintf("DELETE FROM %s WHERE id = %s", table, d.holder(1)),
		CreateTable:     fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);", table, strings.Join(defs, ",\n")),
		DropTable:       fmt.Sprintf("DROP TABLE IF EXISTS %s;", table),
	}
}
