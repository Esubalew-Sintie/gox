# gox

gox creates Go HTTP services and adds features to them with every layer and all of their wiring. It is a code generator, not a framework: everything it writes is ordinary Go that belongs to you, and nothing it generates imports gox.

> **Status.** 0.4 completes the v1.0 scope: the hexagonal layout with chi or the standard library router, five databases (in-memory, Postgres, MySQL, SQLite, MongoDB) with migrations and repository tests against real databases, interactive prompts, and `sync`, `check`, `doctor` and `templates`. Redis, Kafka, S3, jobs, OpenTelemetry, gRPC and external API clients follow in 1.x. The design is in [docs/plan.md](docs/plan.md), the decisions behind it in [docs/adr/](docs/adr/), and the error and response contract in [docs/errors-and-response.md](docs/errors-and-response.md).

## Contents

1. [Install](#1-install)
2. [Five-minute tour](#2-five-minute-tour)
3. [Interactive and scripted use](#3-interactive-and-scripted-use)
4. [Command reference](#4-command-reference)
   - [gox new](#gox-new)
   - [gox gen feature](#gox-gen-feature)
   - [gox gen migration](#gox-gen-migration)
   - [gox destroy feature](#gox-destroy-feature)
   - [gox sync](#gox-sync)
   - [gox check](#gox-check)
   - [gox doctor](#gox-doctor)
   - [gox templates](#gox-templates)
   - [Global flags](#global-flags)
5. [Guides](#5-guides)
   - [Choosing a database](#choosing-a-database)
   - [Running and testing a generated project](#running-and-testing-a-generated-project)
   - [Configuration](#configuration)
   - [The HTTP API a feature gets](#the-http-api-a-feature-gets)
   - [Errors and responses](#errors-and-responses)
   - [Migrations](#migrations)
   - [Changing a constructor and keeping the wiring](#changing-a-constructor-and-keeping-the-wiring)
   - [Customising the templates](#customising-the-templates)
   - [Putting gox in CI](#putting-gox-in-ci)
6. [What a generated project looks like](#6-what-a-generated-project-looks-like)
7. [How gox keeps your code safe](#7-how-gox-keeps-your-code-safe)
8. [Troubleshooting](#8-troubleshooting)
9. [Developing gox](#9-developing-gox)

## 1. Install

Requirements: Go 1.26 or newer. Docker is optional; it is used by `docker compose` for the database and by the generated repository tests.

From a release:

```sh
go install github.com/esubalew-Sintie/gox/cmd/gox@latest
```

From a checkout, which is what you want while gox itself is changing:

```sh
git clone https://github.com/esubalew-Sintie/gox
cd gox
go install ./cmd/gox
```

Check that the binary on your `PATH` is the one you expect:

```sh
gox --version        # gox version 0.4.0-dev
```

If this prints `unknown flag: --version`, an older gox is on your `PATH`; see [Troubleshooting](#8-troubleshooting).

## 2. Five-minute tour

Create a service with Postgres, run it, add a feature, call it, test it.

```sh
gox new fleet-service --module github.com/acme/fleet-service --db postgres --git
cd fleet-service
docker compose up -d postgres
make run
```

The service applies its migrations on start-up and listens on `:8080`. In another shell:

```sh
curl -s localhost:8080/healthz      # {"status":200,"message":"ok","has_retry":false}
curl -s localhost:8080/readyz       # pings the database
```

Add a feature. One command writes the entity, service, repository with its migration, HTTP handler with DTOs and validation, in-memory store, tests, errors, interfaces and the regenerated composition root:

```sh
gox gen feature fleet name:string capacity:int opened_at:time:null
make run
```

```sh
curl -s -X POST localhost:8080/api/v1/fleets -d '{"name":"north","capacity":10}'
curl -s localhost:8080/api/v1/fleets
curl -s localhost:8080/api/v1/fleets/<id>
curl -s -X PUT localhost:8080/api/v1/fleets/<id> -d '{"name":"north","capacity":12}'
curl -s -X DELETE localhost:8080/api/v1/fleets/<id>
curl -s -X POST localhost:8080/api/v1/fleets -d '{}'          # 400 with a details list
curl -s localhost:8080/api/v1/fleets/missing                  # 404
```

Run the tests, including the repository tests against a real Postgres started by testcontainers:

```sh
make test
make lint
```

Everything gox wrote is plain Go under your module. Open `internal/service/fleet/service.go` and change it; gox never rewrites it.

## 3. Interactive and scripted use

On a terminal, `gox new` and `gox gen feature` ask for whatever you did not give on the command line, with the defaults preselected:

```sh
gox new                       # asks name, module path, database, router, git init
gox new orders --db mysql     # asks only module path, router, git init
gox gen feature               # asks the name, then fields one per line until an empty line
gox gen feature vehicle       # asks only the fields
```

Nothing is asked when every choice is given as a flag, when you pass `--defaults` (on `new`) or `--no-interactive`, or when the input is not a terminal. Scripts, Makefiles and CI therefore never block:

```sh
gox new orders --defaults                     # name given, every other choice is the default
gox gen feature vehicle name:string --no-interactive
gox destroy feature vehicle --yes             # confirmations answered yes
```

`ACCESSIBLE=1` asks the same questions as plain numbered lines, for screen readers and simple terminals. `Esc` or `Ctrl-C` in a form aborts without writing anything.

## 4. Command reference

Every mutating command plans first, prints one line per file (`create`, `regen`, `modify`, `identical`, `keep`, `adopt`, `delete`, `conflict`), writes through temporary files, runs `go build ./...` and `go vet ./...`, and restores every file when verification fails. Add `--dry-run` to any of them to see the plan and the diffs without writing.

### gox new

```
gox new [name] [flags]
```

Creates `<name>/` with the hexagonal layout, the composition root, the error and response packages, middleware, tests, `Makefile`, `Dockerfile`, `docker-compose.yml`, `.golangci.yml`, `AGENTS.md`, `config.yaml`, `.env.example`, then runs `go mod tidy` and verifies the tree. The directory must be empty or absent.

| Flag | Default | Meaning |
|---|---|---|
| `--module <path>` | the name | Go module path, for example `github.com/acme/fleet` |
| `--db <name>` | `memory` | `postgres`, `mysql`, `sqlite`, `mongo` or `memory` |
| `--http <name>` | `chi` | `chi`, or `stdlib` for `net/http.ServeMux` with method patterns |
| `--git` | off | run `git init` in the new project |
| `--defaults` | off | take the default for every choice not given as a flag, ask nothing |
| `--with <list>` | none | capabilities (`redis`, `kafka`, `s3`, `jobs`, `otel`, `grpc`, `openapi`); not available yet, the command says so |

```sh
gox new fleet-service --module github.com/acme/fleet-service --db postgres --git
gox new tool --db sqlite --http stdlib --defaults
gox new fleet-service --dry-run --verbose      # every file as a diff, nothing written
```

The command ends with the next steps: `cd`, `make run`, and a `gen feature` example.

### gox gen feature

```
gox gen feature [Name] [field:type ...] [flags]
```

Aliases: `gox g f`, `gox gen module`, `gox gen resource`.

Adds a feature with every layer and regenerates the composition root:

| Written | Where | Class |
|---|---|---|
| entity with `New<Entity>(req)`, `Update(req)` and `Response()` | `internal/domain/entity/<name>.go` | once |
| request and response types, `Validate` on the request | `internal/domain/dto/<name>.go` | once |
| service and its test | `internal/service/<name>/` | once |
| repository for the project's database and its test | `internal/storage/<db>/<name>/` | once |
| migration pair (SQL databases) | `migrations/NNNNNN_create_<plural>.up.sql` and `.down.sql` | once |
| HTTP handler and its test | `internal/handler/http/<name>/` | once |
| in-memory store | `internal/testutil/memstore/<name>.go` | once |
| errors, interfaces | appended to `internal/domain/errors.go`, `internal/storage/repository.go`, `internal/service/service.go`, `internal/handler/handler.go` | codemod |
| wiring | `app/storage.go`, `app/service.go`, `app/handler.go`, `app/routes.go` | tool |

Fields are `name:type`:

| Type | Go | Notes |
|---|---|---|
| `string` | `string` | `TEXT` in Postgres and SQLite, `VARCHAR(255)` in MySQL |
| `text` | `string` | `TEXT` |
| `int`, `int64` | `int`, `int64` | |
| `float64` | `float64` | |
| `bool` | `bool` | |
| `time` | `time.Time` | RFC 3339 in JSON |
| `uuid` | `string` | a string in Go and JSON; `TEXT` in Postgres and SQLite, `CHAR(36)` in MySQL |

A trailing `?` or `:null` makes a field nullable, which maps to a pointer, a nullable column and `omitempty`. Use `:null` in zsh, which treats an unquoted `?` as a glob. Non-nullable fields are `validation.Required` on create. `id`, `created_at` and `updated_at` are generated on every entity and cannot be declared. Names are snake_case in JSON and SQL and PascalCase in Go, whichever form you type.

```sh
gox gen feature fleet name:string capacity:int opened_at:time:null
gox gen feature vehicle plate:string seats:int fleet_id:uuid
gox gen feature                 # interactive
```

A feature that already exists is refused; destroy it first or choose another name.

### gox gen migration

```
gox gen migration <name>
```

Adds an empty golang-migrate pair with the next version number, for SQL databases only. Edit the two files, then `make migrate-up`.

```sh
gox gen migration add_fleet_name_index
# migrations/000003_add_fleet_name_index.up.sql and .down.sql
```

### gox destroy feature

```
gox destroy feature <Name> [flags]
```

Removes everything `gen feature` wrote for the feature: every file whose content is unchanged since gox wrote it, its declarations in the interface files and the error catalogue, and the regenerated composition root. Files you edited are left in place and listed; `--force` deletes them too. The command asks for confirmation; `--yes` answers it.

```sh
gox destroy feature fleet
gox destroy feature fleet --yes --force
```

### gox sync

```
gox sync
```

Reads every constructor the composition root calls, picks an argument for each parameter by type from what the `Init` function has in scope, and regenerates the `app` files. Run it after changing a `New` signature. `gen` and `destroy` run the same resolution. See [Changing a constructor](#changing-a-constructor-and-keeping-the-wiring).

### gox check

```
gox check
```

Regenerates the composition root in memory and compares it with the disk, and checks that every declaration gox owns in the codemod files is still there. Nothing is written. Exit status 1 on drift, so it belongs in CI and in the generated `make check`.

### gox doctor

```
gox doctor
```

Reports the toolchain, `gox.yaml`, the manifest (edited or missing files), template overrides, how every constructor parameter resolves, and whether the project builds and vets.

### gox templates

```
gox templates list
gox templates eject <id>
gox templates diff
```

`list` prints every template id and marks the overridden ones. `eject` copies one into `.gox/templates/<id>` where it takes precedence over the embedded template from then on. `diff` shows how each overridden template differs from the embedded one. See [Customising the templates](#customising-the-templates).

### Global flags

| Flag | Meaning |
|---|---|
| `--dry-run` | print the plan and the diffs without writing anything |
| `-y`, `--yes` | answer yes to confirmations |
| `--force` | overwrite edited tool files, delete edited files on destroy |
| `--adopt` | keep edited tool files as yours instead of overwriting them |
| `--allow-dirty` | run with uncommitted changes in git |
| `--no-interactive` | never ask: take every default, fail when a confirmation is needed |
| `-v`, `--verbose` | print a diff for every change |
| `--no-color` | plain output |
| `-C <dir>`, `--chdir <dir>` | run as if started in `<dir>` |
| `--version` | print the version |

Inside a git repository, mutating commands refuse to run with uncommitted changes unless `--allow-dirty` is given, so that a bad generation is one `git checkout` away from undone.

## 5. Guides

### Choosing a database

| `--db` | Driver | What you get | Needs |
|---|---|---|---|
| `memory` | none | in-memory stores under `internal/testutil/memstore`, wired as the repositories | nothing |
| `postgres` | pgx v5 | connection pool, `WithTx`, error translation, embedded migrations applied on start-up, `cmd/migrate`, repository tests with testcontainers | Docker, or a Postgres at `postgres://app:app@localhost:5432/app` |
| `mysql` | database/sql with go-sql-driver | the same on `database/sql` | Docker, or a MySQL at `app:app@tcp(localhost:3306)/app` |
| `sqlite` | modernc.org/sqlite, no cgo | the same on a file (`data/<name>.db`), WAL mode, busy timeout | nothing |
| `mongo` | official driver v2 | client, collections per feature, optimistic updates on `updated_at` (`ErrConcurrentUpdate` after three attempts), repository tests with testcontainers | Docker, or a MongoDB at `localhost:27017` |

`docker-compose.yml` has a service for the chosen database with a health check; `docker compose up -d <db>` starts it with the credentials in `config.yaml`. `Update` is a locked read-modify-write transaction on SQL databases and an optimistic replace on MongoDB. The database is fixed at `new`; change `db` in `gox.yaml` only on a fresh project.

### Running and testing a generated project

| Command | What it does |
|---|---|
| `make run` | `go run ./cmd`, listens on `:8080` |
| `make build` | binary in `bin/<name>` |
| `make test` | `go test -race ./...`; repository tests start the real database with testcontainers and skip under `-short` |
| `make lint` | golangci-lint with the layer dependency rule |
| `make tidy` | `go mod tidy` |
| `make check` | `gox check` |
| `make migrate-up`, `migrate-down`, `migrate-version` | `cmd/migrate` against the configured database (SQL only) |
| `docker compose up --build` | the service and its database in containers |

`go test -short ./...` runs everything that needs no database.

### Configuration

`config.yaml` at the root holds non-secret defaults. Any value can be overridden with an environment variable named `APP_<SECTION>__<KEY>` (double underscore between levels); `.env.example` lists the common ones and `.env` is git-ignored.

```yaml
server:
  port: 8080
  environment: development
  log_level: info
  read_timeout: 5s
  write_timeout: 10s
  shutdown_timeout: 10s
postgres:
  url: postgres://app:app@localhost:5432/app?sslmode=disable
  max_conns: 10
  auto_migrate: true
```

```sh
APP_SERVER__PORT=9090 APP_POSTGRES__URL=postgres://... make run
```

The `Config` struct is in `internal/config/config.go`. It is yours; gox only appends a block when a capability is added.

### The HTTP API a feature gets

A feature named `fleet` is mounted under `/api/v1/fleets`:

| Method and path | Handler | Success |
|---|---|---|
| `POST /api/v1/fleets` | `Create` | 201 with the entity |
| `GET /api/v1/fleets?page=1&limit=20` | `List` | 200 with `data` list and `meta` paging |
| `GET /api/v1/fleets/{id}` | `Get` | 200 |
| `PUT /api/v1/fleets/{id}` | `Update` | 200 |
| `DELETE /api/v1/fleets/{id}` | `Delete` | 200 with a message and no data |

`GET /healthz` always answers; `GET /readyz` pings the database. Request bodies go through `request.Decode`, which enforces a 1 MiB limit, rejects unknown fields and runs the DTO's `Validate` method (ozzo-validation). Middleware: request logging and panic recovery are applied to every route; an `APIKey` middleware is there to wrap the routes that need it until you have real authentication.

### Errors and responses

Every response is one envelope:

```json
{"status": 200, "message": "ok", "data": {...}, "meta": {...}, "has_retry": false}
{"status": 404, "error": {"type": "not_found", "message": "fleet not found"}, "has_retry": false}
{"status": 400, "error": {"type": "validation", "message": "validation failed",
  "details": [{"field": "name", "description": "cannot be blank"}]}, "has_retry": false}
```

Every error is an `errors.New` sentinel in `internal/domain/errors.go` with one line in `ErrorMap` (HTTP status) and one in `ErrorTypeMap` (type). `gen feature` adds `Err<Feature>NotFound`, `Err<Feature>Invalid` and `Err<Feature>Conflict`; add your own next to them. Wrap with `fmt.Errorf("...: %w", err)` or `domain.Wrap(entry, cause)`; `response.Error` resolves through the wrap chain and returns a fixed 500 for anything it does not know. Types: `validation`, `bad_request`, `unauthorized`, `forbidden`, `not_found`, `timeout`, `conflict`, `payload_too_large`, `rate_limit`, `unavailable`, `external_service`, `internal`. The full contract is in [docs/errors-and-response.md](docs/errors-and-response.md).

### Migrations

SQL projects get `migrations/` with golang-migrate pairs embedded in the binary. `000001_init` comes with `new`, `gen feature` adds `create_<plural>`, and `gen migration` adds an empty pair. With `auto_migrate: true` the service applies pending migrations on start-up; the same code runs from the command line:

```sh
make migrate-up
make migrate-version
make migrate-down          # one step back
```

### Changing a constructor and keeping the wiring

The signature of a `New` is its contract with the composition root. Give the fleet service a configuration dependency:

```go
func New(repo storage.FleetRepository, server config.ServerConfig, log *slog.Logger) *Service
```

Update the tests that call it, then:

```sh
gox check      # exit 1: app/service.go drifted
gox sync       # regenerates it
```

gox type-checks the project, picks each argument by type from what the `Init` function has in scope, and regenerates the `app` files:

```go
Fleet: fleetsvc.New(s.Fleet, cfg.Server, log),
```

What an `Init` function has in scope: `InitStorage` sees the infrastructure (`i.Postgres` and friends) and the logger; `InitService` sees the storage interfaces, config and logger; `InitHandler` sees the services, config and logger. A parameter with no provider, or with more than one assignable candidate, is reported with the candidates and nothing is written. `gen feature` and `destroy feature` run the same resolution, so adding a feature never undoes your wiring.

### Customising the templates

```sh
gox templates list                               # ids like feature/service.go.tmpl
gox templates eject feature/service.go.tmpl      # copied to .gox/templates/feature/service.go.tmpl
# edit it
gox templates diff                               # your changes against the embedded version
gox gen feature vehicle name:string              # uses your template
```

Templates are Go `text/template` with helpers `pascal`, `camel`, `snake`, `kebab`, `plural`, `lower`, `upper`, `title`, `quote`, `join`, `backtick`, `repeat`. Overrides live in the project, so a team shares them through git. Delete the file under `.gox/templates/` to go back to the embedded one.

### Putting gox in CI

For a generated project, install gox and run the check next to the tests:

```yaml
- run: go install github.com/esubalew-Sintie/gox/cmd/gox@latest
- run: gox check
- run: go test -race ./...        # -short when CI has no Docker
```

`gox check` fails when someone edited a tool file or removed a generated declaration by hand. Fix it with `gox sync`, or keep the edit with `--adopt`.

## 6. What a generated project looks like

```
app/                      composition root: app.go knows the order, one Init* file per concern
cmd/main.go               calls app.Initialize
cmd/migrate/              up, down, version (SQL databases)
externalapi/              external API clients, interfaces in externalapi.go
internal/
  config/config.go        Config struct and loader (config.yaml, then APP_ environment)
  domain/entity/          one file per entity, plain structs
  domain/dto/             request and response types, one file per feature
  domain/errors.go        every error with ErrorMap and ErrorTypeMap; id.go, page.go
  service/                service.go interfaces; one implementation package per feature
  handler/http/           handler.go interfaces; middleware/, request/, response/; one package per feature
  storage/                repository.go and cache.go interfaces; <db>/ with pool, tx, migrate, errors and one package per feature
  testutil/dbtest/        starts the real database for repository tests
  testutil/memstore/      in-memory stores, one per feature
migrations/               golang-migrate pairs, embedded (SQL databases)
config.yaml, .env.example, Makefile, Dockerfile, docker-compose.yml, .golangci.yml, AGENTS.md
gox.yaml                  what gox generates from
.gox/manifest.json        what gox wrote
```

Dependency rule, enforced by the generated `.golangci.yml`: `internal/domain` imports only the standard library; implementations depend on interfaces, never on other implementations; `app` is the only package that knows concrete types.

Startup: `app.Initialize` calls `InitConfig`, `InitLogger`, `InitInfra`, `InitExternalAPI`, `InitStorage`, `InitCache`, `InitService`, `InitHandler`, `InitRoutes`, then serves. Request: router and middleware, then the feature handler decodes and validates the `dto` request and hands it to the service, which builds the entity with `entity.New<Feature>(req)` and calls the repository; the handler writes `out.Response()` through `response.Success` or `response.Error`.

The generated `AGENTS.md` explains all of this to anyone, human or agent, who works on the service.

## 7. How gox keeps your code safe

Every file gox writes is recorded in `.gox/manifest.json` with its ownership class and a hash:

| Class | Meaning |
|---|---|
| `once` | written one time, then yours; never rewritten |
| `tool` | regenerated on every run under a `// Code generated by gox. DO NOT EDIT.` header; an edited tool file stops the command until you pass `--force` or `--adopt` |
| `codemod` | yours, but gox appends and removes its own declarations in exactly seven files: `internal/storage/repository.go`, `internal/storage/cache.go`, `internal/service/service.go`, `internal/handler/handler.go`, `externalapi/externalapi.go`, `internal/domain/errors.go`, `internal/config/config.go` |

Every mutating command plans in memory first and prints what it will do. Writes go through temporary files, then `go build` and `go vet` run; on failure every file is restored and the command fails. `destroy` deletes only files whose content is unchanged and lists the rest. `gox.yaml` is the intent (module, router, database, features and their fields); the manifest is the record.

## 8. Troubleshooting

**`gox new` produced `initiator/`, `internal/constant` and `internal/module`.** That is the old 0.1 generator, still on your `PATH`. `gox --version` prints `unknown flag` for it. Reinstall from the checkout with `go install ./cmd/gox` and delete what the old binary wrote.

**`zsh: no matches found: opened_at:time?`** zsh treats an unquoted `?` as a glob. Write `opened_at:time:null` or quote the argument.

**`missing dot in first path element`** came from gox 0.3 when `--module` was omitted; 0.4 accepts a bare name. On older binaries pass `--module`.

**`N file(s) were edited since gox wrote them; nothing was written`.** A `tool` file was edited by hand. Run the command with `--adopt` to keep your version as yours, or `--force` to regenerate it.

**`git working tree has uncommitted changes`.** Commit or stash, or pass `--allow-dirty`.

**`confirmation needed; pass --yes`.** You ran `destroy` with `--no-interactive` or without a terminal.

**`generated code did not verify`.** `go build` or `go vet` failed on the result and every file was restored; the output above the message says why. The usual cause after `sync` is a test that still calls the old constructor signature.

**Repository tests fail with a Docker error.** testcontainers needs a reachable Docker daemon. Run `go test -short ./...` to skip them.

**`sync` reports a parameter with no provider.** The constructor asks for a type nothing in that `Init` scope provides. Either take an interface from the layer below, or add the provider to the `Init` by hand and `--adopt` the file.

## 9. Developing gox

```sh
make all          # tidy check, build, vet, lint, race tests
make test         # go test -race -timeout 30m ./...
go test -short ./...
```

The end-to-end test generates one project per database and runs its tests; it needs network for `go mod tidy` and Docker for the Postgres, MySQL and Mongo containers. Without Docker those repository tests are skipped. `go test -short` skips the whole end-to-end test. After changing gox, reinstall it with `go install ./cmd/gox` before trying it on a project.

Layout of this repository:

```
cmd/gox/              main
internal/cli/         cobra commands and the end-to-end test
internal/prompt/      interactive questions (charmbracelet/huh)
internal/generate/    which files a project, a feature or the wiring needs
internal/templates/   the embedded templates and the .gox/templates override
internal/codemod/     the append and remove edits on the seven codemod files
internal/resolve/     type-checks the project and resolves constructor arguments
internal/plan/        the virtual tree and the step list
internal/apply/       snapshot, atomic write, verify, rollback
internal/manifest/    gox.yaml and .gox/manifest.json
internal/model/       project, feature and field model, the field DSL
internal/db/          SQL dialects
docs/                 plan, ADRs, error and response contract
```
