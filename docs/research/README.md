# Research: making gox the standard Go generator for hexagonal and clean architecture

Date: 2026-11-10. Status: for review before any implementation.

Method: five parallel research tracks, each with web sources checked on 2026-11-10. The full track reports are in this folder:

| Track | Report |
|---|---|
| Go scaffolding and generator CLIs | [01-go-generators.md](01-go-generators.md) |
| Hexagonal and clean architecture in Go | [02-hex-clean-layouts.md](02-hex-clean-layouts.md) |
| Generator CLIs in other ecosystems | [03-cross-ecosystem-generators.md](03-cross-ecosystem-generators.md) |
| Library and toolchain defaults for 2026 | [04-tech-defaults.md](04-tech-defaults.md) |
| Safe modification of existing Go source | [05-codemod-mechanics.md](05-codemod-mechanics.md) |

The proposed design and phased plan that follow from this research are in [../plan.md](../plan.md).

## 1. The question

How should gox become the Go CLI where a developer picks architecture, database and infrastructure once, then only writes business logic, with every piece of wiring generated, compiling, and reversible?

## 2. Ten findings

1. **The gap is real.** Go has create-only skeleton tools (go-blueprint, gowebly, gopamin, gonew) and framework-bound resource generators (goctl for go-zero, gf for GoFrame, Kratos, Nunu, Buffalo). Nobody mainstream offers a framework-neutral "add a feature and wire it" generator. That is gox's positioning.
2. **Durable generators partition ownership instead of patching.** sqlc, ent, Goa, oapi-codegen, goctl and gf all regenerate tool-owned files wholesale under a `// Code generated ... DO NOT EDIT.` header and create user-owned files once, never touching them again. Tools that patch hand-written files are where breakage lives: Buffalo inserts routes with no duplicate check, Goca only wires if it recognises `mux.NewRouter()`, Rails `destroy` corrupts routes via regex, NestJS splices text by offset. Kubebuilder, the most mature marker-based tool, ended up adding a PROJECT manifest and a re-scaffold command, which is manifest-driven regeneration by another name.
3. **gox's current injector has two reproducible defects.** Removing an import through `go/ast` and `format.Node` strands the import's comments onto the next import and leaves a blank line (reproduced on 2026-11-10). The presence check is a substring match, so a commented-out line counts as present, the exact bug Rails documented for routes. Replacing the mechanism is worth more than fixing it.
4. **google/wire is archived** (2025-08-25). Kratos and Nunu are built on it. `init()` self-registration (cobra-cli, gf) is discouraged by the Uber and Google style guides and loses compiler cycle detection. gox must emit the explicit constructor graph itself; manual constructor injection is the only safe default.
5. **Hexagonal and clean produce the same Go code.** The real variance across the reference repos is naming (`core/port/service/adapter` versus `entity/usecase/controller/repo`), interface placement (central port package versus consumer-side), core structure (optional CQRS split), and grouping axis (by layer versus by module). One generation model with naming presets and two switches covers both without doubling templates.
6. **The community converged on concrete practices** to bake in: separate transport, domain and storage models with explicit mapping; small consumer-side interfaces only at I/O boundaries; an error-code type mapped to HTTP status in the transport adapter; transactions as a closure on the aggregate repository or a unit of work, never hidden in context; `context.Context` as the first parameter of every port; logging and metrics as decorators applied in the composition root; in-memory fakes plus one shared repository test suite run against every implementation; the dependency rule enforced by a linter.
7. **The standard library is enough for HTTP.** net/http method and wildcard patterns since Go 1.22 reached 46 percent adoption in the 2026 JetBrains survey, up from 32. chi is the zero-cost upgrade when groups and scoped middleware are needed. Fiber is excluded because fasthttp breaks the net/http ecosystem. Echo v5 changed its handler signature.
8. **Data and tooling defaults for 2026** are settled: pgx v5, sqlc as an opt-in, goose for migrations, the MongoDB driver v2 (v1 reached end of life in January 2026), no ORM by default. Config via caarlos0/env, logging via slog, validation via go-playground/validator, mocks via hand-written fakes or mockery v3, integration tests via testcontainers-go, golangci-lint v2, connect-go v2 for RPC, huma or oapi-codegen for OpenAPI. Viper, sqlx, kelseyhightower/envconfig and cleanenv are stalled.
9. **Generator UX has a settled vocabulary** across Rails, Phoenix, NestJS, Angular, Laravel and AdonisJS: `new`, `gen`, `add`, `remove`, `destroy`, `doctor`; a transcript of `create`, `identical`, `skip`, `conflict`; a positional `field:type` DSL; prompt only for what is missing and expose a flag for every prompt; persist defaults in a project file; plan in a virtual tree so dry-run is free; destroy from a manifest; refuse to edit a dirty git tree; let users eject and diff templates; compose big generators from small ones; print tailored next steps; stay minimal by default.
10. **Go 1.26 is the floor and Go 1.27 is already out.** Most default dependencies already declare `go 1.26.0`. Go 1.27 (August 2026) backs `encoding/json` with json/v2, which rejects duplicate keys and invalid UTF-8 by default; generated projects need a test for that before they bump. An `AGENTS.md` and an architecture-lint config are now expected in a generated project.

## 3. Positioning: what exists in Go today

| Tool | Kind | Architecture choice | Wiring | After initial generation |
|---|---|---|---|---|
| go-blueprint | create-only skeleton | one layout, pick router and driver | constructor calls `database.New()` inside | nothing, choices are forgotten |
| goctl (go-zero) | framework-bound | go-zero layout from `.api` DSL | hand-edited ServiceContext | regenerates handlers, routes, types; never touches `logic/` |
| gf (GoFrame) | framework-bound | GoFrame layout | `init()` registration via blank imports | overwrite decided by header comment |
| Kratos | framework-bound | kratos-layout | google/wire ProviderSets, archived | user edits three files per service, reruns wire |
| Nunu | framework-bound | gin, gorm, wire | wire | `create` writes files and wires nothing |
| Buffalo | full-stack framework | Rails-like | splices into `actions/app.go` | no duplicate check, destroy via string replace |
| Goca | hexagonal/clean generator | fixed | patches `main.go` by anchor strings | only works with gorm and gorilla mux |
| scaffold-go | clean generator | fixed | marker comments in `cmd/main.go` | closest analogue to gox today, fx and Fiber bound |
| Encore | infra-from-code | Encore conventions | parser builds the app model | not a scaffolder; criticised for non-Go conventions |
| Goa, ent, sqlc, oapi-codegen, ogen | contract or schema codegen | not applicable | user implements generated interfaces | regenerate `gen/` wholesale, user files untouched |

Two features nobody combines: framework neutrality and automatic, reversible wiring. See [01-go-generators.md](01-go-generators.md) for per-tool evidence and issue trends.

## 4. Hexagonal and clean architecture in Go

**Reference points.** bxcodec/go-clean-arch v4, evrone/go-clean-template, Three Dots Labs' Wild Workouts and their articles, Ben Johnson's standard package layout, ardanlabs/service, Kat Zien's structure examples, the official go.dev module layout guide, and the critique of golang-standards/project-layout by Russ Cox.

**Where the layouts agree.** Binaries under `cmd/`, everything else under `internal/`, `pkg/` only for genuinely exportable code. Inbound adapters call application services or use cases, which call domain code and depend on outbound ports that the core owns. Wiring happens in `main` or a small app package. Plain structs cross boundaries, never database rows or framework types.

**Where they differ in practice.**

| Axis | Hexagonal in Go | Clean in Go |
|---|---|---|
| Names | `core/domain`, `core/port`, `core/service`, `adapter/...` | `entity`, `usecase`, `controller`, `repo` |
| Interface placement | central `port` package | beside the consumer (`usecase/x/contracts.go`) or in the domain |
| Core structure | unspecified by Cockburn | entities versus use cases, often with a CQRS `command`/`query` split |
| Grouping | by layer or by bounded context | by layer or by bounded context |

**Cross-cutting conventions worth generating.** Repository `Update` as a closure that loads, mutates and persists inside one transaction, with an optional unit-of-work port for multi-aggregate writes. A typed error with a code in the domain, mapped to HTTP status in the transport adapter. Syntactic validation in the inbound adapter, invariants in domain constructors. A transport-neutral `Page` type in the use-case layer. Decorators for logging and metrics applied in the composition root. Tests: table-driven domain tests, use-case tests against memory fakes, one shared repository suite run against every adapter, handler tests via `httptest`.

**Pitfalls to design against.** One struct carrying `json`, `gorm` and `validate` tags. Interfaces with one implementation "because the rules say so". Mapping boilerplate, which is exactly what a generator should absorb. Deep nesting such as `internal/core/service/order/...` for small services. Transactions leaking into use cases.

**Decision this supports.** One internal generation model; `--arch hexagonal|clean` as a naming and placement preset; `--cqrs` and `--group-by module|layer` as later switches. Full trees are in [02-hex-clean-layouts.md](02-hex-clean-layouts.md) and the gox-specific versions in [../plan.md](../plan.md).

## 5. How wiring must be generated

| Mechanism | Idempotent | Reversible | Survives user edits | Comments preserved | Trust |
|---|---|---|---|---|---|
| Markers plus text insertion (gox today, kubebuilder, Buffalo, Rails) | only with careful dedupe | fragile | breaks when markers move | outside the region | medium |
| `go/ast` plus `go/printer` | if checked | comments orphaned | reflows | no (golang/go#20744, open since 2017) | low |
| dave/dst | yes | good | yes | yes | medium, single maintainer |
| AST-located byte edits plus `imports.Process` | yes | good | yes | yes | medium-high |
| Tool-owned files regenerated from a manifest | inherent | trivial | detected by hash, refused | not applicable | high |
| Scan the code and regenerate a registry (wire style) | yes | trivial | yes | not applicable | high, but fails when the build is broken |
| `init()` self-registration | not applicable | delete file | yes | not applicable | hidden, discouraged |

**Recommendation.** Regenerate a tool-owned composition root from a manifest; never write to user-owned files after creation; record a content hash per generated file so hand edits to tool-owned files are detected and refused rather than silently overwritten; plan every write in memory, require a clean git tree, write atomically, run `go build` and `go vet`, and roll back on failure. Where a user-owned file must be touched, which the design avoids, use AST-located byte edits and `imports.Process`, never a reprint of the whole file. Templates stay `text/template` plus `imports.Process`, embedded, with a per-project override directory and a drift diff. Details and evidence per tool are in [05-codemod-mechanics.md](05-codemod-mechanics.md).

**Reproduced defect in gox today.** Running the current import-removal technique on an import that carries a doc comment and a trailing comment produces this:

```go
import (
	"context"

	// user module (hand-written note)
	// trailing note
	"example.com/app/internal/module/order"
	"example.com/app/internal/module/payment"
)
```

Both comments now belong to the next import. Removing a middle import leaves a stray blank line.

## 6. CLI user experience

The lessons distilled from Rails, Phoenix, NestJS, Angular, Laravel, .NET Clean Architecture templates, Django, Spring Initializr and AdonisJS are in [03-cross-ecosystem-generators.md](03-cross-ecosystem-generators.md). The ones that shape gox:

- Ask the boundary question (which feature, which transport), decide the layout. Phoenix even warns when you add to a large existing context.
- Every prompt has a flag; `--defaults` and `--no-interactive` always work; the project file records choices so later commands never re-ask.
- Plan in a virtual tree; print a transcript; batch conflicts up front; `--dry-run` shows the diff.
- Destroy is manifest-driven and refuses to delete modified files without `--force`.
- Templates can be ejected per file with fallback to the embedded default, and diffed after upgrades.
- Big generators compose small ones that work alone.
- Generated code is the user's code; print tailored next steps; keep the domain layer dependency-free.

## 7. Technology defaults for generated code

| Concern | Default | Offered alternatives | Why |
|---|---|---|---|
| HTTP router | net/http `ServeMux` | chi v5 | stdlib patterns are sufficient; chi adds groups and scoped middleware at no cost |
| OpenAPI | none by default | huma v2 (code-first), oapi-codegen (spec-first) | opt-in capability; huma keeps spec, validation and docs in sync but is a one-maintainer project |
| Postgres | pgx v5 with SQL written in the adapter | sqlc for generated typed queries | zero extra tooling by default; sqlc is the best-in-class upgrade |
| MySQL, SQLite | database/sql with go-sql-driver/mysql, modernc.org/sqlite | | pure-Go SQLite avoids cgo |
| MongoDB | mongo-go-driver v2, pinned at 2.9.3 or later | | v1 is end of life; two security fixes in 2026 |
| Migrations | goose v3, embedded SQL files | golang-migrate, tern, dbmate, Atlas | library plus CLI, embeds in the binary, freshest cadence |
| gRPC | buf, protoc-gen-go, connect-go v2 | grpc-go | one listener serves REST and RPC |
| Config | caarlos0/env v11 | koanf v2 | one tiny dependency; container-first |
| Logging | log/slog | zerolog or zap as handlers | zero dependencies |
| Validation | go-playground/validator v10 in the HTTP adapter only | | de-facto standard; skipped when huma is selected |
| Dependency injection | generated manual constructors | | wire archived; fx is runtime reflection |
| Mocks | generated in-memory fakes | mockery v3 config | fewer mocks overall; fakes are plain Go |
| Tests | stdlib `testing`; testcontainers-go for DB integration | testify | keeps generated code dependency-light |
| Lint | golangci-lint v2 with depguard rules per layout | go-arch-lint | enforces the dependency rule |
| Task runner, reload, containers | Makefile, air, compose.yaml | Taskfile | zero install |
| Go directive | `go 1.26.0`, no toolchain line | | most defaults already require 1.26 |

Evidence with release dates is in [04-tech-defaults.md](04-tech-defaults.md).

## 8. Risks and things moving fast

- Go 1.27 switched `encoding/json` to json/v2 semantics; generated APIs change behaviour on bump. Generate a test.
- huma has one maintainer; oapi-codegen is volunteer-run with no release cadence; sqlc has a large backlog although commits continue.
- Ent and Atlas are single-vendor; Atlas paywalled its migration linter.
- Minimum Go versions are bumping every six months across the ecosystem.
- Reading constructor signatures with go/types fails exactly when the project does not compile; the design keeps a manifest fallback.
- Template surface grows with presets times databases; the SQL adapter template must be dialect-parameterised rather than duplicated.

## 9. What this changes for gox

| Today | After |
|---|---|
| Marker comments injected into `initiator/*.go` | Tool-owned `*_gen.go` composition root regenerated from a manifest |
| `.gox.yaml` records a few booleans | `gox.yaml` records intent; `.gox/manifest.json` records every generated file with class, template, version and hash |
| Layouts `layered-rest` and `feature-first` | Presets `hexagonal` and `clean` over one model; `feature-first` returns later as `--group-by module` |
| Routers net/http, chi, gin with three template branches | net/http default, chi optional; no gin |
| Repositories panic on database errors and take no context | Ports take `context.Context` first and return errors; `Update` runs in a transaction |
| Handlers decode JSON by hand and validate inline | Generated `httpx` helpers, typed error mapping, validator tags, optional OpenAPI |
| `gox g module` creates files and injects wiring | `gox gen feature` creates user-owned files once and regenerates the root; `destroy`, `check`, `sync`, `templates`, `upgrade` complete the lifecycle |
| Generation verified by `go test ./...` | Virtual-tree plan, dry-run diff, atomic apply, `go build` and `go vet`, rollback on failure |

## 10. Open questions carried into the plan

1. Exact directory and package names for each preset.
2. Whether OpenAPI should be opt-in or on by default.
3. How gox learns about constructor signatures the user has changed: go/types driven `sync` with a manifest fallback is proposed.
4. The field DSL for `gen feature` and the list of supported types.
5. Which capabilities ship in v1.0.
