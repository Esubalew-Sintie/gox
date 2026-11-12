// Package generate turns the project model into the files gox wants on
// disk: the once files of a new project, the tool files of the composition
// root, the once files of a feature, and the codemod edits that add a
// feature to the interface files and the error catalogue or remove it again.
// It never writes; the plan and apply packages do.
package generate

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/esubalew-Sintie/gox/internal/codemod"
	"github.com/esubalew-Sintie/gox/internal/db"
	"github.com/esubalew-Sintie/gox/internal/fsx"
	"github.com/esubalew-Sintie/gox/internal/manifest"
	"github.com/esubalew-Sintie/gox/internal/model"
	"github.com/esubalew-Sintie/gox/internal/naming"
	"github.com/esubalew-Sintie/gox/internal/plan"
	"github.com/esubalew-Sintie/gox/internal/render"
	"github.com/esubalew-Sintie/gox/internal/resolve"
	"github.com/esubalew-Sintie/gox/internal/templates"
)

// GoVersion is the go directive of generated projects.
const GoVersion = "1.26"

// Generator renders files for one project model.
type Generator struct {
	root     string
	project  *model.Project
	manifest *manifest.Manifest
	layout   model.Layout
	renderer *render.Renderer
}

// New returns a generator. root is the project directory, which may not
// exist yet; the manifest is read for the declarations gox already owns.
func New(root string, p *model.Project, m *manifest.Manifest, store *templates.Store) *Generator {
	return &Generator{
		root:     root,
		project:  p,
		manifest: m,
		layout:   model.Hexagonal,
		renderer: render.New(store, p.Module),
	}
}

type fileSpec struct {
	template string
	path     string
	class    manifest.Class
}

func (g *Generator) projectFiles() []fileSpec {
	l := g.layout
	files := []fileSpec{
		{"project/go.mod.tmpl", "go.mod", manifest.ClassOnce},
		{"project/cmd_main.go.tmpl", model.CmdMain, manifest.ClassOnce},
		{"project/app_app.go.tmpl", path.Join(model.AppDir, "app.go"), manifest.ClassOnce},
		{"project/app_config.go.tmpl", path.Join(model.AppDir, "config.go"), manifest.ClassOnce},
		{"project/app_logger.go.tmpl", path.Join(model.AppDir, "logger.go"), manifest.ClassOnce},
		{"project/config.yaml.tmpl", model.ConfigYAML, manifest.ClassOnce},
		{"project/env.example.tmpl", ".env.example", manifest.ClassOnce},
		{"project/gitignore.tmpl", ".gitignore", manifest.ClassOnce},
		{"project/AGENTS.md.tmpl", "AGENTS.md", manifest.ClassOnce},
		{"project/README.md.tmpl", "README.md", manifest.ClassOnce},
		{"project/Makefile.tmpl", "Makefile", manifest.ClassOnce},
		{"project/Dockerfile.tmpl", "Dockerfile", manifest.ClassOnce},
		{"project/docker-compose.yml.tmpl", "docker-compose.yml", manifest.ClassOnce},
		{"project/golangci.yml.tmpl", ".golangci.yml", manifest.ClassOnce},
		{"project/domain_page.go.tmpl", l.PageFile(), manifest.ClassOnce},
		{"project/domain_id.go.tmpl", path.Join(l.DomainDir, "id.go"), manifest.ClassOnce},
		{"project/response_envelope.go.tmpl", path.Join(l.ResponseDir(), "envelope.go"), manifest.ClassOnce},
		{"project/response_response.go.tmpl", path.Join(l.ResponseDir(), "response.go"), manifest.ClassOnce},
		{"project/request_request.go.tmpl", path.Join(l.RequestDir(), "request.go"), manifest.ClassOnce},
		{"project/request_page.go.tmpl", path.Join(l.RequestDir(), "page.go"), manifest.ClassOnce},
		{"project/middleware_logger.go.tmpl", path.Join(l.MiddlewareDir(), "logger.go"), manifest.ClassOnce},
		{"project/middleware_recovery.go.tmpl", path.Join(l.MiddlewareDir(), "recovery.go"), manifest.ClassOnce},
		{"project/middleware_auth.go.tmpl", path.Join(l.MiddlewareDir(), "auth.go"), manifest.ClassOnce},
		{"project/testutil_fixture.go.tmpl", path.Join(model.TestutilDir, "fixture.go"), manifest.ClassOnce},
		{"project/memstore_store.go.tmpl", path.Join(model.MemstoreDir, "store.go"), manifest.ClassOnce},
		{"project/domain_errors.go.tmpl", l.ErrorsFile(), manifest.ClassCodemod},
		{"project/config_config.go.tmpl", model.ConfigFile, manifest.ClassCodemod},
		{"project/storage_repository.go.tmpl", l.RepositoryInterfaces(), manifest.ClassCodemod},
		{"project/storage_cache.go.tmpl", l.CacheInterfaces(), manifest.ClassCodemod},
		{"project/service_service.go.tmpl", l.ServiceInterfaces(), manifest.ClassCodemod},
		{"project/handler_handler.go.tmpl", l.HandlerInterfaces(), manifest.ClassCodemod},
		{"project/externalapi.go.tmpl", l.ExternalAPIInterfaces(), manifest.ClassCodemod},
	}
	return append(files, g.databaseFiles()...)
}

// databaseFiles are the project files of the selected database: the client
// in app/, the shared storage package, the test harness, and for SQL
// databases the migrations and the migrate command.
func (g *Generator) databaseFiles() []fileSpec {
	l := g.layout
	info := dbInfoFor(g.project.DB)
	once := manifest.ClassOnce
	var files []fileSpec
	if info.IsSQL {
		up, down := model.Migration(1, "init")
		files = append(files,
			fileSpec{"db/migrations_embed.go.tmpl", path.Join(model.MigrationsDir, "embed.go"), once},
			fileSpec{"db/migration_init_up.sql.tmpl", up, once},
			fileSpec{"db/migration_init_down.sql.tmpl", down, once},
			fileSpec{"db/cmd_migrate.go.tmpl", "cmd/migrate/main.go", once},
		)
	}
	pkg := info.DBPackage
	dir := l.DBDir(pkg)
	dbtest := path.Join(model.TestutilDir, "dbtest")
	switch db.Kind(g.project.DB) {
	case db.Postgres:
		files = append(files,
			fileSpec{"db/postgres_app.go.tmpl", path.Join(model.AppDir, "postgres.go"), once},
			fileSpec{"db/postgres_pool.go.tmpl", path.Join(dir, "postgres.go"), once},
			fileSpec{"db/postgres_tx.go.tmpl", path.Join(dir, "tx.go"), once},
			fileSpec{"db/postgres_errors.go.tmpl", path.Join(dir, "errors.go"), once},
			fileSpec{"db/postgres_migrate.go.tmpl", path.Join(dir, "migrate.go"), once},
			fileSpec{"db/postgres_dbtest.go.tmpl", path.Join(dbtest, "postgres.go"), once},
		)
	case db.MySQL, db.SQLite:
		files = append(files,
			fileSpec{"db/" + pkg + "_app.go.tmpl", path.Join(model.AppDir, pkg+".go"), once},
			fileSpec{"db/" + pkg + "_open.go.tmpl", path.Join(dir, pkg+".go"), once},
			fileSpec{"db/" + pkg + "_tx.go.tmpl", path.Join(dir, "tx.go"), once},
			fileSpec{"db/" + pkg + "_errors.go.tmpl", path.Join(dir, "errors.go"), once},
			fileSpec{"db/" + pkg + "_migrate.go.tmpl", path.Join(dir, "migrate.go"), once},
			fileSpec{"db/" + pkg + "_dbtest.go.tmpl", path.Join(dbtest, pkg+".go"), once},
		)
	case db.Mongo:
		files = append(files,
			fileSpec{"db/mongo_app.go.tmpl", path.Join(model.AppDir, "mongo.go"), once},
			fileSpec{"db/mongo_connect.go.tmpl", path.Join(dir, "mongo.go"), once},
			fileSpec{"db/mongo_errors.go.tmpl", path.Join(dir, "errors.go"), once},
			fileSpec{"db/mongo_dbtest.go.tmpl", path.Join(dbtest, "mongo.go"), once},
		)
	case db.Memory:
	}
	return files
}

func toolFiles() []fileSpec {
	var out []fileSpec
	for _, name := range []string{"infra", "external_api", "cache", "storage", "service", "handler", "routes"} {
		out = append(out, fileSpec{"app/" + name + ".go.tmpl", path.Join(model.AppDir, name+".go"), manifest.ClassTool})
	}
	return out
}

func (g *Generator) featureFiles(f model.Feature, migration int) []fileSpec {
	l := g.layout
	once := manifest.ClassOnce
	files := []fileSpec{
		{"feature/domain.go.tmpl", path.Join(l.FeatureDomainDir(f), f.Name.Snake+".go"), manifest.ClassOnce},
		{"feature/service.go.tmpl", path.Join(l.FeatureServiceDir(f), "service.go"), manifest.ClassOnce},
		{"feature/service_test.go.tmpl", path.Join(l.FeatureServiceDir(f), "service_test.go"), manifest.ClassOnce},
		{"feature/handler.go.tmpl", path.Join(l.FeatureHandlerDir(f), "handler.go"), manifest.ClassOnce},
		{"feature/dto.go.tmpl", path.Join(l.FeatureHandlerDir(f), "dto.go"), manifest.ClassOnce},
		{"feature/handler_test.go.tmpl", path.Join(l.FeatureHandlerDir(f), "handler_test.go"), manifest.ClassOnce},
		{"feature/memstore.go.tmpl", l.FeatureMemstoreFile(f), manifest.ClassOnce},
	}
	info := dbInfoFor(g.project.DB)
	repoDir := l.FeatureRepositoryDir(f, info.DBPackage)
	switch db.Kind(g.project.DB) {
	case db.Postgres:
		files = append(files,
			fileSpec{"feature/postgres_repository.go.tmpl", path.Join(repoDir, "repository.go"), once},
			fileSpec{"feature/postgres_repository_test.go.tmpl", path.Join(repoDir, "repository_test.go"), once},
		)
	case db.MySQL, db.SQLite:
		files = append(files,
			fileSpec{"feature/sql_repository.go.tmpl", path.Join(repoDir, "repository.go"), once},
			fileSpec{"feature/sql_repository_test.go.tmpl", path.Join(repoDir, "repository_test.go"), once},
		)
	case db.Mongo:
		files = append(files,
			fileSpec{"feature/mongo_repository.go.tmpl", path.Join(repoDir, "repository.go"), once},
			fileSpec{"feature/mongo_repository_test.go.tmpl", path.Join(repoDir, "repository_test.go"), once},
		)
	case db.Memory:
	}
	if info.IsSQL {
		up, down := model.Migration(migration, "create_"+f.Name.Plural)
		files = append(files,
			fileSpec{"feature/migration_up.sql.tmpl", up, once},
			fileSpec{"feature/migration_down.sql.tmpl", down, once},
		)
	}
	return files
}

// Project renders every file of a new project except the tool files and
// the features: the once files and the initial codemod files.
func (g *Generator) Project() ([]plan.Desired, error) {
	data := g.projectData()
	var out []plan.Desired
	for _, spec := range g.projectFiles() {
		d, err := g.render(spec, data, model.OwnerProject)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// Tool renders the composition-root files for the whole model, calling
// every constructor with the arguments in wiring; a feature and layer not
// in wiring gets the default arguments.
func (g *Generator) Tool(wiring resolve.Wiring) ([]plan.Desired, error) {
	data := g.projectData()
	for i := range data.Features {
		data.Features[i].setArgs(g, wiring)
	}
	var out []plan.Desired
	for _, spec := range toolFiles() {
		d, err := g.render(spec, data, model.OwnerProject)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// Feature renders the once files of a feature and the codemod edits that
// register it in the interface files and the error catalogue.
func (g *Generator) Feature(f model.Feature) ([]plan.Desired, error) {
	fd := g.featureData(f)
	migration, err := NextMigration(g.root)
	if err != nil {
		return nil, err
	}
	var out []plan.Desired
	for _, spec := range g.featureFiles(f, migration) {
		d, err := g.render(spec, fd, model.FeatureOwner(f))
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	edits, err := g.featureEdits(fd, false, false)
	if err != nil {
		return nil, err
	}
	return append(out, edits...), nil
}

// RemoveFeature returns the codemod edits that take a feature out of the
// interface files and the error catalogue. Unless force is set, an interface
// the developer changed is refused with an error naming it.
func (g *Generator) RemoveFeature(f model.Feature, force bool) ([]plan.Desired, error) {
	return g.featureEdits(g.featureData(f), true, force)
}

func (g *Generator) render(spec fileSpec, data any, owner string) (plan.Desired, error) {
	var content []byte
	var err error
	if strings.HasSuffix(spec.path, ".go") {
		content, err = g.renderer.RenderGo(spec.template, spec.path, data)
	} else {
		content, err = g.renderer.Render(spec.template, data)
	}
	if err != nil {
		return plan.Desired{}, err
	}
	return plan.Desired{Path: spec.path, Class: spec.class, Template: spec.template, Owner: owner, Content: content}, nil
}

// featureEdits computes the four codemod files after adding (or removing)
// the feature. Each edit starts from the file as it is on disk.
func (g *Generator) featureEdits(fd featureData, remove, force bool) ([]plan.Desired, error) {
	l := g.layout
	pascal := fd.Name.Pascal
	domainPkg := model.ImportPath(g.project.Module, l.DomainDir)
	featurePkg := model.ImportPath(g.project.Module, l.FeatureDomainDir(fd.Feature))

	type edit struct {
		path    string
		names   []string
		snippet string
		imports []string
	}
	edits := []edit{
		{l.RepositoryInterfaces(), []string{pascal + "Repository"}, fd.repositoryInterface(), []string{"context", domainPkg, featurePkg}},
		{l.ServiceInterfaces(), []string{pascal + "Service"}, fd.serviceInterface(), []string{"context", domainPkg, featurePkg}},
		{l.HandlerInterfaces(), []string{pascal + "Handler"}, fd.handlerInterface(), []string{"net/http"}},
	}

	var out []plan.Desired
	for _, e := range edits {
		d, err := g.codemod(e.path, e.names, remove, func(cf *codemod.File) error {
			if remove {
				return removeDecl(cf, e.path, e.names[0], e.snippet, force)
			}
			if err := cf.AddImports(e.imports...); err != nil {
				return err
			}
			return cf.AppendDecl(e.snippet)
		})
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}

	names := fd.errorNames()
	d, err := g.codemod(l.ErrorsFile(), names, remove, func(cf *codemod.File) error {
		comment := "// " + pascal
		if remove {
			if _, err := cf.RemoveMapEntries("ErrorMap", comment, names); err != nil {
				return err
			}
			if _, err := cf.RemoveMapEntries("ErrorTypeMap", comment, names); err != nil {
				return err
			}
			_, err := cf.RemoveVarSpecs(names)
			return err
		}
		if err := cf.InsertDeclBefore("ErrorMap", fd.errorBlock()); err != nil {
			return err
		}
		if err := cf.AppendMapEntries("ErrorMap", comment, fd.statusEntries()); err != nil {
			return err
		}
		return cf.AppendMapEntries("ErrorTypeMap", comment, fd.typeEntries())
	})
	if err != nil {
		return nil, err
	}
	return append(out, d), nil
}

func removeDecl(cf *codemod.File, file, name, snippet string, force bool) error {
	current, ok := cf.DeclSource(name)
	if !ok {
		return nil
	}
	if !force {
		expected, err := codemod.Normalize(snippet)
		if err != nil {
			return err
		}
		if current != expected {
			return fmt.Errorf("%s in %s was changed since gox wrote it; remove it by hand or pass --force", name, file)
		}
	}
	_, err := cf.RemoveDecl(name)
	return err
}

// codemod reads the file from disk, applies fn and returns the desired
// content with the declarations gox owns afterwards.
func (g *Generator) codemod(rel string, names []string, remove bool, fn func(*codemod.File) error) (plan.Desired, error) {
	src, exists, err := fsx.ReadIfExists(filepath.Join(g.root, filepath.FromSlash(rel)))
	if err != nil {
		return plan.Desired{}, err
	}
	if !exists {
		return plan.Desired{}, fmt.Errorf("%s is missing; is %s a gox project?", rel, g.root)
	}
	cf, err := codemod.Parse(rel, src)
	if err != nil {
		return plan.Desired{}, err
	}
	if err := fn(cf); err != nil {
		return plan.Desired{}, fmt.Errorf("%s: %w", rel, err)
	}
	if err := cf.Format(g.project.Module); err != nil {
		return plan.Desired{}, err
	}
	var decls []string
	template := ""
	if entry, ok := g.manifest.Lookup(rel); ok {
		decls = slices.Clone(entry.Decls)
		template = entry.Template
	}
	if remove {
		decls = slices.DeleteFunc(decls, func(n string) bool { return slices.Contains(names, n) })
	} else {
		for _, n := range names {
			if !slices.Contains(decls, n) {
				decls = append(decls, n)
			}
		}
	}
	slices.Sort(decls)
	return plan.Desired{Path: rel, Class: manifest.ClassCodemod, Template: template, Owner: model.OwnerProject, Content: cf.Bytes(), Decls: decls}, nil
}

// Template data.

type projectData struct {
	dbInfo
	Name         string
	Module       string
	Arch         string
	HTTP         string
	Chi          bool
	Capabilities []string
	GoVersion    string
	Features     []featureData
}

type featureData struct {
	model.Feature
	dbInfo
	Module      string
	Title       string // "Vehicle group"
	PluralTitle string // "Vehicle groups"
	Route       string // "/vehicle-groups"
	Fields      []fieldData
	Required    []fieldData
	SQL         db.Statements
	InsertArgs  string // e.ID, e.Name, ..., e.CreatedAt, e.UpdatedAt
	UpdateArgs  string // e.Name, ..., e.UpdatedAt, e.ID
	ScanArgs    string // &e.ID, &e.Name, ..., &e.UpdatedAt
	Chi         bool
	ListPath    string // path of the collection in handler tests
	ItemPrefix  string // prefix of one item's path in handler tests
	// Arguments of the constructor calls in the composition root.
	RepositoryArgs string
	ServiceArgs    string
	HandlerArgs    string
}

func (fd *featureData) setArgs(g *Generator, wiring resolve.Wiring) {
	pick := func(layer string, setter *string) {
		c := g.constructor(fd.Feature, layer)
		args, ok := wiring[resolve.Key(fd.Name.Snake, layer)]
		if !ok {
			args = g.DefaultArgs(c)
		}
		*setter = strings.Join(args, ", ")
	}
	pick(resolve.LayerRepository, &fd.RepositoryArgs)
	pick(resolve.LayerService, &fd.ServiceArgs)
	pick(resolve.LayerHandler, &fd.HandlerArgs)
}

// dbInfo is what templates need to know about the selected database.
type dbInfo struct {
	DB         string // as in gox.yaml
	DBPackage  string // package under internal/storage; "" for memory
	DBTitle    string // "Postgres", "MySQL", "SQLite", "MongoDB", "memory"
	InfraField string // field on app.Infra and config.Config
	IsSQL      bool
	Postgres   bool
	MySQL      bool
	SQLite     bool
	Mongo      bool
	Memory     bool
}

func dbInfoFor(kind string) dbInfo {
	d, _ := db.For(kind)
	info := dbInfo{DB: kind, DBPackage: d.Package, IsSQL: d.SQL}
	switch d.Kind {
	case db.Postgres:
		info.DBTitle, info.InfraField, info.Postgres = "Postgres", "Postgres", true
	case db.MySQL:
		info.DBTitle, info.InfraField, info.MySQL = "MySQL", "MySQL", true
	case db.SQLite:
		info.DBTitle, info.InfraField, info.SQLite = "SQLite", "SQLite", true
	case db.Mongo:
		info.DBTitle, info.InfraField, info.Mongo = "MongoDB", "Mongo", true
	case db.Memory:
		info.DBTitle, info.Memory = "memory", true
	}
	return info
}

type fieldData struct {
	model.Field
	Required   bool
	Sample     string // Go literal for tests
	JSONSample string // JSON literal for tests
}

func (g *Generator) projectData() projectData {
	p := g.project
	data := projectData{
		dbInfo: dbInfoFor(p.DB),
		Name:   p.Name, Module: p.Module, Arch: p.Arch, HTTP: p.HTTP, Chi: p.HTTP == "chi",
		Capabilities: p.Capabilities, GoVersion: GoVersion,
	}
	for i := range p.Features {
		data.Features = append(data.Features, g.featureData(p.Features[i]))
	}
	return data
}

func (g *Generator) featureData(f model.Feature) featureData {
	route := "/" + strings.ReplaceAll(f.Name.Plural, "_", "-")
	fd := featureData{
		Feature:     f,
		dbInfo:      dbInfoFor(g.project.DB),
		Module:      g.project.Module,
		Title:       humanize(f.Name.Snake),
		PluralTitle: humanize(f.Name.Plural),
		Route:       route,
		Chi:         g.project.HTTP == "chi",
		ListPath:    "/",
		ItemPrefix:  "/",
	}
	if !fd.Chi {
		fd.ListPath = "/api/v1" + route
		fd.ItemPrefix = "/api/v1" + route + "/"
	}
	fd.setArgs(g, nil)
	insert := []string{"e.ID"}
	var update []string
	scan := []string{"&e.ID"}
	for _, fld := range f.Fields {
		d := fieldData{Field: fld, Required: isRequired(fld), Sample: sample(fld), JSONSample: jsonSample(fld)}
		fd.Fields = append(fd.Fields, d)
		if d.Required {
			fd.Required = append(fd.Required, d)
		}
		insert = append(insert, "e."+fld.Name.Pascal)
		update = append(update, "e."+fld.Name.Pascal)
		scan = append(scan, "&e."+fld.Name.Pascal)
	}
	insert = append(insert, "e.CreatedAt", "e.UpdatedAt")
	update = append(update, "e.UpdatedAt", "e.ID")
	scan = append(scan, "&e.CreatedAt", "&e.UpdatedAt")
	fd.InsertArgs = strings.Join(insert, ", ")
	fd.UpdateArgs = strings.Join(update, ", ")
	fd.ScanArgs = strings.Join(scan, ", ")
	if d, ok := db.For(g.project.DB); ok && d.SQL {
		fd.SQL = d.Statements(f)
	}
	return fd
}

func isRequired(f model.Field) bool {
	if f.Nullable {
		return false
	}
	return f.Type == model.TypeString || f.Type == model.TypeText || f.Type == model.TypeUUID
}

func sample(f model.Field) string {
	switch f.Type {
	case model.TypeString:
		return `"sample"`
	case model.TypeText:
		return `"sample text"`
	case model.TypeInt, model.TypeInt64:
		return "1"
	case model.TypeFloat64:
		return "1.5"
	case model.TypeBool:
		return "true"
	case model.TypeTime:
		return "time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)"
	case model.TypeUUID:
		return `"00000000-0000-0000-0000-000000000001"`
	}
	return "nil"
}

func jsonSample(f model.Field) string {
	switch f.Type {
	case model.TypeTime:
		return `"2026-01-01T00:00:00Z"`
	case model.TypeInt, model.TypeInt64, model.TypeFloat64, model.TypeBool, model.TypeString, model.TypeText, model.TypeUUID:
		return sample(f)
	}
	return "nil"
}

func humanize(snake string) string {
	s := strings.ReplaceAll(snake, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Snippets appended to the codemod files.

func (fd featureData) errorNames() []string {
	p := fd.Name.Pascal
	return []string{"Err" + p + "NotFound", "Err" + p + "Invalid", "Err" + p + "Conflict"}
}

func (fd featureData) errorBlock() string {
	p := fd.Name.Pascal
	return fmt.Sprintf("// %s errors.\nvar (\n\tErr%sNotFound = errors.New(%q)\n\tErr%sInvalid = errors.New(%q)\n\tErr%sConflict = errors.New(%q)\n)",
		p, p, fd.Title+" not found.", p, "Invalid "+strings.ToLower(fd.Title)+".", p, fd.Title+" already exists.")
}

func (fd featureData) statusEntries() []string {
	n := fd.errorNames()
	return []string{n[0] + ": http.StatusNotFound", n[1] + ": http.StatusBadRequest", n[2] + ": http.StatusConflict"}
}

func (fd featureData) typeEntries() []string {
	n := fd.errorNames()
	return []string{n[0] + ": ErrorTypeNotFound", n[1] + ": ErrorTypeValidation", n[2] + ": ErrorTypeConflict"}
}

func (fd featureData) repositoryInterface() string {
	p, pkg := fd.Name.Pascal, fd.Name.Snake
	return fmt.Sprintf(`// %sRepository stores %s.
type %sRepository interface {
	Create(ctx context.Context, e *%s.%s) error
	Get(ctx context.Context, id string) (*%s.%s, error)
	List(ctx context.Context, page domain.PageRequest) ([]*%s.%s, int64, error)
	Update(ctx context.Context, id string, fn func(*%s.%s) error) (*%s.%s, error)
	Delete(ctx context.Context, id string) error
}`, p, strings.ToLower(fd.PluralTitle), p, pkg, p, pkg, p, pkg, p, pkg, p, pkg, p)
}

func (fd featureData) serviceInterface() string {
	p, pkg := fd.Name.Pascal, fd.Name.Snake
	return fmt.Sprintf(`// %sService is the use-case API of the %s feature.
type %sService interface {
	Create(ctx context.Context, in %s.CreateInput) (*%s.%s, error)
	Get(ctx context.Context, id string) (*%s.%s, error)
	List(ctx context.Context, page domain.PageRequest) ([]*%s.%s, int64, error)
	Update(ctx context.Context, id string, in %s.UpdateInput) (*%s.%s, error)
	Delete(ctx context.Context, id string) error
}`, p, pkg, p, pkg, pkg, p, pkg, p, pkg, p, pkg, pkg, p)
}

func (fd featureData) handlerInterface() string {
	p := fd.Name.Pascal
	return fmt.Sprintf(`// %sHandler serves the %s HTTP API.
type %sHandler interface {
	Create(w http.ResponseWriter, r *http.Request)
	Get(w http.ResponseWriter, r *http.Request)
	List(w http.ResponseWriter, r *http.Request)
	Update(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)
	Routes() http.Handler
}`, p, strings.ToLower(fd.Title), p)
}

// FeatureName parses a feature name argument.
func FeatureName(arg string) (naming.Name, error) {
	n, err := naming.Parse(arg)
	if err != nil {
		return naming.Name{}, err
	}
	return *n, nil
}

// NextMigration returns the version after the highest one in migrations/,
// or 1 when the directory is empty or missing.
func NextMigration(root string) (int, error) {
	entries, err := os.ReadDir(filepath.Join(root, model.MigrationsDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 1, nil
		}
		return 0, fmt.Errorf("read %s: %w", model.MigrationsDir, err)
	}
	highest := 0
	for _, e := range entries {
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		highest = max(highest, v)
	}
	return highest + 1, nil
}

var migrationName = regexp.MustCompile(`^(\d{6})_.*\.(up|down)\.sql$`)

// Migration renders an empty golang-migrate pair for gen migration.
func (g *Generator) Migration(version int, name string) ([]plan.Desired, error) {
	n, err := naming.Parse(name)
	if err != nil {
		return nil, err
	}
	up, down := model.Migration(version, n.Snake)
	upBody := fmt.Sprintf("-- %s: write the forward migration here.\n", n.Snake)
	downBody := fmt.Sprintf("-- %s: write the statements that undo the up migration here.\n", n.Snake)
	return []plan.Desired{
		{Path: up, Class: manifest.ClassOnce, Owner: model.OwnerProject, Content: []byte(upBody)},
		{Path: down, Class: manifest.ClassOnce, Owner: model.OwnerProject, Content: []byte(downBody)},
	}, nil
}
