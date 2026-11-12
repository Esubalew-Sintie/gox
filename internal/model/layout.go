package model

import (
	"path"

	"github.com/esubalew-Sintie/gox/internal/naming"
)

// Layout maps the model to the paths of the tree in docs/plan.md section 3.1
// (ADR 0003). v1.0 has one layout; a clean preset later only changes the
// three layer directory names.
type Layout struct {
	DomainDir  string // internal/domain
	ServiceDir string // internal/service
	HandlerDir string // internal/handler
	StorageDir string // internal/storage
}

// Hexagonal is the v1.0 layout.
var Hexagonal = Layout{
	DomainDir:  "internal/domain",
	ServiceDir: "internal/service",
	HandlerDir: "internal/handler",
	StorageDir: "internal/storage",
}

// Fixed paths that do not depend on the layer names.
const (
	AppDir         = "app"
	CmdMain        = "cmd/main.go"
	ExternalAPIDir = "externalapi"
	ConfigFile     = "internal/config/config.go"
	ConfigYAML     = "config.yaml"
	MigrationsDir  = "migrations"
	TestutilDir    = "internal/testutil"
	MemstoreDir    = "internal/testutil/memstore"
)

// Codemods lists the codemod files in the order of ADR 0005.
func (l Layout) Codemods() []string {
	return []string{
		l.RepositoryInterfaces(),
		l.CacheInterfaces(),
		l.ServiceInterfaces(),
		l.HandlerInterfaces(),
		l.ExternalAPIInterfaces(),
		l.ErrorsFile(),
		ConfigFile,
	}
}

// Interface and catalogue files.

func (l Layout) RepositoryInterfaces() string  { return path.Join(l.StorageDir, "repository.go") }
func (l Layout) CacheInterfaces() string       { return path.Join(l.StorageDir, "cache.go") }
func (l Layout) PublisherInterfaces() string   { return path.Join(l.StorageDir, "publisher.go") }
func (l Layout) ServiceInterfaces() string     { return path.Join(l.ServiceDir, "service.go") }
func (l Layout) HandlerInterfaces() string     { return path.Join(l.HandlerDir, "handler.go") }
func (l Layout) ExternalAPIInterfaces() string { return path.Join(ExternalAPIDir, "externalapi.go") }
func (l Layout) ErrorsFile() string            { return path.Join(l.DomainDir, "errors.go") }
func (l Layout) PageFile() string              { return path.Join(l.DomainDir, "page.go") }

// HTTP support packages.

func (l Layout) HTTPDir() string       { return path.Join(l.HandlerDir, "http") }
func (l Layout) MiddlewareDir() string { return path.Join(l.HTTPDir(), "middleware") }
func (l Layout) RequestDir() string    { return path.Join(l.HTTPDir(), "request") }
func (l Layout) ResponseDir() string   { return path.Join(l.HTTPDir(), "response") }

// DBDir is the adapter directory for the selected database, for example
// internal/storage/postgres, which also holds the pool, tx and Translate.
func (l Layout) DBDir(db string) string { return path.Join(l.StorageDir, db) }

// Per-feature directories and files.

func (l Layout) FeatureDomainDir(f Feature) string  { return path.Join(l.DomainDir, f.Name.Snake) }
func (l Layout) FeatureServiceDir(f Feature) string { return path.Join(l.ServiceDir, f.Name.Snake) }
func (l Layout) FeatureHandlerDir(f Feature) string { return path.Join(l.HTTPDir(), f.Name.Snake) }
func (l Layout) FeatureRepositoryDir(f Feature, db string) string {
	return path.Join(l.DBDir(db), f.Name.Snake)
}
func (l Layout) FeatureCacheFile(f Feature) string {
	return path.Join(l.StorageDir, "redis", f.Name.Snake+"_cache.go")
}
func (l Layout) FeatureMemstoreFile(f Feature) string {
	return path.Join(MemstoreDir, f.Name.Snake+".go")
}

// ProviderDir is the folder of one external API client.
func (l Layout) ProviderDir(provider naming.Name) string {
	return path.Join(ExternalAPIDir, provider.Snake)
}

// Migration returns the golang-migrate pair for a version and name.
func Migration(version int, name string) (up, down string) {
	base := path.Join(MigrationsDir, fmtVersion(version)+"_"+name)
	return base + ".up.sql", base + ".down.sql"
}

// ImportPath joins the module path and a project-relative directory.
func ImportPath(module, dir string) string {
	return path.Join(module, dir)
}

// OwnerProject is the manifest owner tag of project-level files; features,
// capabilities and clients have their own tags below.
const OwnerProject = "project"

// FeatureOwner is the manifest owner tag of a feature's files.
func FeatureOwner(f Feature) string { return "feature:" + f.Name.Snake }

// CapabilityOwner is the manifest owner tag of a capability's files.
func CapabilityOwner(name string) string { return "capability:" + name }

// ClientOwner is the manifest owner tag of an external API client's files.
func ClientOwner(provider naming.Name) string { return "client:" + provider.Snake }

func fmtVersion(v int) string {
	s := ""
	for i := 0; i < 6; i++ {
		s = string(rune('0'+v%10)) + s
		v /= 10
	}
	return s
}
