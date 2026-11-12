# gox v2: the ports layout

Status: proposal, 2026-11-12. Supersedes sections 3 to 6 of `docs/plan.md` once accepted. The engine (manifest, planner, apply, codemod, resolution, prompts, CLI) is unchanged; this document is about what gox generates and which choices a project owner has.

Sources: the project owner's production service (`initiator` layout, ozzo validation, the error maps and the envelope), the prabogo template (`~/comp/prabogo`: `model`, `domain`, `port/inbound`, `port/outbound`, `adapter/inbound/<kind>`, `adapter/outbound/<kind>`, run modes, `utils`), and gox 0.4 (pure generator, manifest ownership, generated composition root, typed contracts, five databases).

## 1. Principles

Kept from gox:

1. The generated service imports nothing of gox. No runtime, no reflection container, no framework package.
2. Every file is `once`, `tool` or `codemod`, recorded with a hash. `once` is yours forever; `tool` is regenerated; `codemod` gets whole declarations appended and removed, nothing else.
3. The composition root is generated from the constructors by type. `gox sync` re-wires, `gox check` fails CI on drift.
4. Every port is typed and takes `context.Context`. Every error is a sentinel in one catalogue with a status and a type. Every HTTP response is one envelope.
5. One command writes a whole feature; one command removes it.

Taken from prabogo:

6. The folder vocabulary: `model`, `domain`, `port/inbound`, `port/outbound`, `adapter/inbound/<kind>`, `adapter/outbound/<kind>`, `utils/<topic>`.
7. The entity embeds its input, so one struct serves the HTTP body, the message payload, the command argument and the database row.
8. Several ways in and out as equals: HTTP, consumer, command inbound; database, cache, publisher, external API outbound. One binary with run modes.
9. Operations as part of the scaffold: telemetry, secret masking, request identity, auth strategies, a CI workflow, a security note.

Rejected from prabogo: `any` typed ports, nil ports for unset drivers, runtime driver switching by environment, package globals holding connections, a root level `utils` outside `internal`, string errors, `Request` and `Response` envelopes, registries that construct adapters on every call, the framework runtime.

## 2. Choices and defaults

A project's stack is written in `gox.yaml` at `new` and changed later with `add`, `remove` and `set`. Every choice has a default so `gox new fleet --defaults` is a complete, runnable service.

| Choice | Options | Default | Chosen by | Changed later by |
|---|---|---|---|---|
| Router | `chi`, `stdlib`, `none` | `chi` | `new --http`, prompt | not changeable once a feature exists; `none` for workers and CLIs |
| Database | `postgres`, `mysql`, `sqlite`, `mongo`, `memory` | `memory` | `new --db`, prompt | not changeable; `memory` projects can `set db` while no feature exists |
| Logging | `slog-json`, `slog-text`, `zap`, `zerolog` | `slog-json` | `new --log`, prompt | `set log <driver>`; only `app/logger.go` changes, see 5.1 |
| Telemetry | `none`, `otel` | `none` | `new --with otel` | `add otel`, `remove otel` |
| Cache | `none`, `redis` | `none` | `new --with redis` | `add redis`, `remove redis`; then `gen cache <feature>` |
| Broker | `none`, `kafka`, `rabbitmq` | `none` | `new --with kafka` | `add kafka`; then `gen publisher`, `gen consumer` |
| Auth | `none`, `apikey`, `jwt` | `none` | `new --auth` | `add auth --driver jwt`, `remove auth` |
| Object storage | `none`, `s3` | `none` | `new --with s3` | `add s3` |
| Jobs | `none`, `ticker` | `none` | `new --with jobs` | `add jobs`; then `gen job <name>` |
| Validation | `ozzo` | `ozzo` | fixed in v2 | |
| Config | koanf, `config.yaml` plus `APP_` environment | fixed | | |
| Git | init or not | ask | `new --git` | |

The interactive `new` asks, in one form: name, module, database, router, logging, capabilities (a multi-select of otel, redis, kafka, auth, s3, jobs), the auth driver when auth is ticked, and git. Everything given as a flag is not asked; `--defaults` asks nothing.

`gox.yaml` after `gox new fleet --db postgres --log zap --with otel,redis --auth jwt`:

```yaml
version: 3
module: github.com/acme/fleet
http: chi
db: postgres
log: zap
telemetry: otel
cache: redis
auth: jwt
features: []
```

## 3. The tree

Ownership: `once` = yours after creation, `tool` = regenerated, `codemod` = yours with gox-owned declarations. `[otel]`, `[redis]`, `[kafka]`, `[auth]` mark files that exist only with that capability.

```
cmd/
  main.go                           once   func main() { os.Exit(app.Run(os.Args[1:])) }; never changes
app/                                       composition root
  app.go                            once   Init order, graceful shutdown in reverse order
  config.go                         once   InitConfig
  logger.go                         tool   InitLogger for the chosen log driver, masking handler always on
  infra.go                          tool   InitInfra: database, redis, kafka, otel exporters
  external_api.go                   tool   InitExternalAPI: one client per provider
  storage.go                        tool   InitStorage: repositories
  cache.go                          tool   InitCache [redis]
  domain.go                         tool   InitDomain: use cases
  handler.go                        tool   InitHandler: HTTP handlers
  routes.go                         tool   mounts handlers under /api/v1, healthz, readyz, auth middleware [auth]
  consumers.go                      tool   InitConsumers [kafka]
  commands.go                       tool   InitCommands, when any command exists
  modes.go                          tool   Run(args): parses the mode; http | consume <name> | command <name> [args] | migrate up|down|version
internal/
  config/config.go                  codemod   Config struct; add appends a block per capability
  model/
    errors.go                       codemod   ErrorType, sentinels, ErrorMap, ErrorTypeMap, Lookup, Wrap, ValidationError
    page.go                         once      PageRequest, PageMeta
    <feature>.go                    once      Entity{ID; Input; CreatedAt; UpdatedAt}, Input with Validate, Filter
  domain/
    <feature>/domain.go             once      use cases; business rules; returns catalogue errors
    <feature>/domain_test.go        once      against memstore and the fakes
  port/
    inbound/<feature>.go            codemod   <Feature>HTTP, <Feature>Consumer, <Feature>Command, as generated
    outbound/<feature>.go           codemod   <Feature>Repository, <Feature>Cache, <Feature>Publisher
    outbound/transaction.go         once      Transactor interface (SQL databases)
    outbound/<provider>.go          codemod   external API client interface, one file per provider
  adapter/
    inbound/http/
      middleware/                   once      request id, logger, recovery, auth_apikey.go, auth_jwt.go [auth]
      request/                      once      Decode with size limit, unknown fields, Validate; ParsePage; ParseFilter
      response/                     once      the envelope; Success, Error, WithMeta
      <feature>/handler.go          once      the five handlers and Routes()
      <feature>/handler_test.go     once      httptest against the domain with memstore
    inbound/consumer/<feature>/     once      consumer.go, consumer_test.go [kafka]
    inbound/command/<feature>/      once      <name>.go
    outbound/postgres/              once      postgres.go (pool), tx.go (Transactor, context-carried tx), errors.go (Translate), migrate.go
    outbound/postgres/<feature>/    once      repository.go, repository_test.go (testcontainers)
    outbound/mysql|sqlite|mongo/    once      the same shape for the chosen database
    outbound/redis/                 once      redis.go (client) [redis]
    outbound/redis/<feature>/       once      cache.go, cache_test.go
    outbound/kafka/                 once      kafka.go (client), publisher and consumer helpers [kafka]
    outbound/kafka/<feature>/       once      publisher.go
    outbound/externalapi/<provider>/ once     client.go, client_test.go
  utils/
    log/                            once      New(driver, level) *slog.Logger; FromContext
    masking/                        once      sensitive keys, Mask for maps, slices, structs
    activity/                       once      request id, action, principal in context
    id/                             once      NewID, SecureToken
    tracing/                        tool      Start, RecordError: no-op without otel, OTel API with it
    metrics/                        tool      HTTP, consumer, repository instruments: no-op without otel
    jwt/                            once      JWKS client, Validate [auth jwt]
  testutil/
    memstore/<feature>.go           once      in-memory repository, and cache fake when redis
    dbtest/                         once      container start for the chosen database
    brokertest/                     once      in-memory publisher and consumer fakes [kafka]
migrations/                         once      embedded golang-migrate pairs (SQL databases)
docs/openapi.yaml                   tool      from the inputs and routes, when gen openapi has been run
.github/workflows/ci.yml            once      check, lint, test with the database as a service container
config.yaml .env.example Makefile Dockerfile docker-compose.yml .golangci.yml AGENTS.md README.md SECURITY.md
gox.yaml .gox/manifest.json
```

Differences from gox 0.4: `service/` becomes `domain/`; `domain/entity` and `domain/dto` become `model/`; the per-layer interface files `storage/repository.go`, `service/service.go`, `handler/handler.go` become per-feature port files; `storage/<db>` becomes `adapter/outbound/<db>`; `handler/http` becomes `adapter/inbound/http`; `externalapi/` becomes `adapter/outbound/externalapi/` with interfaces in `port/outbound/<provider>.go`; `cmd/migrate` folds into the `migrate` run mode; `utils/` is new.

## 4. One feature, file by file

`gox gen feature fleet name:string capacity:int opened_at:time:null` on a Postgres project writes:

| File | Content |
|---|---|
| `internal/model/fleet.go` | `type Fleet struct { ID string; FleetInput; CreatedAt, UpdatedAt time.Time }`; `type FleetInput struct { Name string; Capacity int; OpenedAt *time.Time }` with json tags and `Validate` (required on non-nullable fields); `type FleetFilter struct { IDs []string }` plus one `[]string` per `uuid` field, for lookups by foreign key |
| `internal/model/errors.go` | appends `ErrFleetNotFound`, `ErrFleetInvalid`, `ErrFleetConflict` and their two map lines |
| `internal/port/outbound/fleet.go` | `FleetRepository { Create(ctx, *model.Fleet) error; Get(ctx, id) (*model.Fleet, error); List(ctx, page, filter) ([]*model.Fleet, int64, error); Update(ctx, id, fn func(*model.Fleet) error) (*model.Fleet, error); Delete(ctx, id) error }` |
| `internal/port/inbound/fleet.go` | `FleetHTTP { Create, Get, List, Update, Delete http.HandlerFunc-shaped; Routes() http.Handler }` |
| `internal/domain/fleet/domain.go` | `New(repo port.FleetRepository, log *slog.Logger) *Domain`; `Create` does `e := &model.Fleet{FleetInput: in}; stamp; repo.Create`; `Update` does `e.FleetInput = in` inside the repository's update; `List` passes page and filter; every method opens a span through `utils/tracing` (no-op unless otel) |
| `internal/domain/fleet/domain_test.go` | create, get, list, update, delete, not found, against memstore |
| `internal/adapter/inbound/http/fleet/handler.go` | `New(uc port.FleetUseCases, log *slog.Logger) *Handler`; decodes `model.FleetInput`, calls the use cases, writes the envelope; `Routes()` for chi or `ServeMux`; the interface it depends on is declared on the inbound side, see 5.6 |
| `internal/adapter/inbound/http/fleet/handler_test.go` | httptest through Routes() with memstore |
| `internal/adapter/outbound/postgres/fleet/repository.go` | pgx implementation; `Update` locks the row inside the transaction from the context |
| `internal/adapter/outbound/postgres/fleet/repository_test.go` | testcontainers suite, `-short` skips |
| `migrations/000002_create_fleets.up.sql`, `.down.sql` | |
| `internal/testutil/memstore/fleet.go` | `FleetStore` implementing the repository |
| `app/storage.go`, `app/domain.go`, `app/handler.go`, `app/routes.go` | regenerated |

Then, each optional:

- `gox gen consumer fleet` [kafka]: `port/inbound/fleet.go` gains `FleetConsumer { Handle(ctx, msg []byte) error }`; `adapter/inbound/consumer/fleet/consumer.go` decodes `[]model.FleetInput` and calls the domain; `app/consumers.go` and `app/modes.go` gain `consume fleet`; a test with `brokertest`.
- `gox gen publisher fleet` [kafka]: `port/outbound/fleet.go` gains `FleetPublisher { PublishUpserted(ctx, *model.Fleet) error }`; `adapter/outbound/kafka/fleet/publisher.go`; `app/storage.go` provides it. The domain's `New` does not change by itself: add `pub port.FleetPublisher` to it and run `gox sync`.
- `gox gen cache fleet` [redis]: `port/outbound/fleet.go` gains `FleetCache { Get(ctx, id) (*model.Fleet, error); Set(ctx, *model.Fleet) error; Delete(ctx, id) error }`; `adapter/outbound/redis/fleet/cache.go` with TTL from config; memstore gains the fake; same `sync` step to use it in the domain.
- `gox gen command fleet import`: `port/inbound/fleet.go` gains `FleetCommand { Import(ctx, args []string) error }`; `adapter/inbound/command/fleet/import.go`; `app/commands.go` and the `command fleet import` mode.
- `gox gen client corebank`: `port/outbound/corebank.go` with an empty interface to fill; `adapter/outbound/externalapi/corebank/client.go` with base URL, timeout, auth header from config, an `http.Client` with tracing transport; `app/external_api.go`.

## 5. Cross-cutting design

### 5.1 Logging: one type, four backends

Everything in the project takes `*slog.Logger`. The log driver only changes the handler built in `app/logger.go`:

| Driver | Handler |
|---|---|
| `slog-json` | `slog.NewJSONHandler` |
| `slog-text` | `slog.NewTextHandler`, for local development |
| `zap` | `zapslog.NewHandler` over a zap core (the sample service's logger) |
| `zerolog` | `slogzerolog` bridge |

The masking handler from `utils/log` wraps whichever backend is chosen and redacts attributes whose key matches `utils/masking` (`password`, `token`, `secret`, `authorization`, `api_key`, ...), recursively through groups, maps and structs. `utils/log.FromContext(ctx)` adds the request id, action and principal from `utils/activity`. Level and format come from `config.yaml` (`server.log_level`, `server.log_driver`) and `APP_SERVER__LOG_LEVEL`.

Because `app/logger.go` is a tool file and nothing else names the backend, `gox set log zap` is a regeneration of one file plus `go get`.

### 5.2 Telemetry: no-op until added

`utils/tracing` and `utils/metrics` always exist and are always called from adapters and domain methods. Without otel they are twenty lines of no-ops with no third-party import. `gox add otel` regenerates them on the OpenTelemetry API, adds the SDK and OTLP exporters to `app/infra.go`, the HTTP middleware span and request metrics, consumer and repository spans, `OTEL_*` lines to `.env.example`, and `docker-compose.grafana.yml` with the collector, Loki, Tempo, Mimir, Grafana and dashboards. `remove otel` reverses it. Because the call sites never change, adding telemetry late does not touch a single once-file.

### 5.3 Identity in the context

`utils/activity` holds a request id, an action name and a principal. The HTTP middleware sets the request id (from `X-Request-ID` or new) and echoes it; consumers set it from the message header; commands from a fresh id. Logs, spans and error responses carry it.

### 5.4 Errors, validation, response

Unchanged from gox 0.4 and `docs/errors-and-response.md`: sentinels in `model/errors.go`, `ErrorMap` and `ErrorTypeMap`, `Lookup` through the wrap chain, `ValidationError` with field details, fixed 500 for anything unknown; `request.Decode` enforces 1 MiB, rejects unknown fields and runs `Validate`; `response.Success` and `response.Error` write the envelope `{status, message, data, meta, has_retry}` and `{status, error: {type, message, details}, has_retry}`. The entity's json tags are the wire form of `data`; hide a field with `json:"-"`.

### 5.5 Transactions

`port/outbound/transaction.go` declares `Transactor { InTx(ctx, fn func(ctx context.Context) error) error }`. The SQL adapters implement it by starting a transaction and storing it in the context; every repository method obtains its executor with `postgres.From(ctx)` which returns the transaction when present and the pool otherwise. A use case that touches two repositories wraps them in `tx.InTx`. Single-row updates keep the `Update(ctx, id, fn)` lock-read-modify-write. MongoDB implements `Transactor` with a session when the server is a replica set, and a pass-through otherwise.

### 5.6 Inbound dependency direction

Inbound adapters depend on the domain through an interface declared on the inbound side, in `port/inbound/<feature>.go`: `FleetUseCases { Create(ctx, in model.FleetInput) (*model.Fleet, error); Get; List; Update; Delete }`. The domain package asserts `var _ port.FleetUseCases = (*Domain)(nil)`. The HTTP handler, the consumer and the command all take `port.FleetUseCases`, so a handler test can pass a fake and the composition root resolves it by type as today. This is prabogo's `ClientDomain` interface, moved from the domain package to the port package where the hexagon says it belongs.

### 5.7 Run modes and lifecycle

`cmd/main.go` is `os.Exit(app.Run(os.Args[1:]))` and never changes. `app/modes.go` owns the arguments: `http` (default) serves; `consume <feature>` runs that consumer until a signal; `command <feature> <name> [args]` runs one command and exits; `migrate up|down|version` runs the embedded migrations.

Startup runs `InitConfig`, `InitLogger`, `InitInfra`, `InitExternalAPI`, `InitStorage`, `InitCache`, `InitDomain`, `InitHandler`, `InitConsumers`, `InitCommands`, then the mode. Shutdown is the reverse: stop accepting HTTP, drain in-flight requests within `shutdown_timeout`, stop consumers, flush telemetry, close Redis, close Kafka, close the database. `app.go` holds that order and is a once-file; the Init functions it calls are tool files. The Makefile gets `run`, `consume`, `command`, `migrate-*` targets, and `docker-compose.yml` gets one service per consumer so the same image runs as API and as worker.

### 5.8 Auth

`add auth --driver apikey` writes `middleware/auth_apikey.go` comparing a constant-time key from config and wires it on the `/api` group in `app/routes.go`. `--driver jwt` writes `utils/jwt` (JWKS fetch with cache, RS256 and ES256) and `middleware/auth_jwt.go` that puts the subject into `activity`. Routes that must stay public (`healthz`, `readyz`) are mounted outside the group. Switching drivers regenerates `routes.go` and writes the other middleware file; the old one is deleted when unchanged.

### 5.9 Dependency rule

Enforced by the generated depguard rules:

- `model` imports the standard library and ozzo only.
- `domain` imports `model`, `port`, `utils/{log,tracing,metrics,activity,id}`; never `adapter` or `app`.
- `port` imports `model` only.
- `adapter/inbound/*` imports `model`, `port`, `utils`, the router; never `adapter/outbound` or `domain` implementations.
- `adapter/outbound/*` imports `model`, `port`, `utils`, the driver; never `domain` or `adapter/inbound`.
- `utils/*` imports the standard library and its own driver; never `model`, `domain`, `port`, `adapter`.
- `app` is the only package that imports everything.

### 5.10 Capability graph

Each capability is one record in the generator, not prose, so that `add`, `remove`, `doctor`, the `[kafka]` style file gating and `gox.yaml` validation all read the same table:

| Capability | Config block | Infra provider | Files | Compose service | Requires | Blocks removal while |
|---|---|---|---|---|---|---|
| database (one of five) | `postgres:` ... | `i.Postgres` | `adapter/outbound/<db>/`, `testutil/dbtest`, `migrations/` | `postgres` | | any feature exists |
| `redis` | `redis:` | `i.Redis` | `adapter/outbound/redis/redis.go`, `app/cache.go`, memstore cache fakes | `redis` | | any `gen cache` exists |
| `kafka` / `rabbitmq` | `kafka:` | `i.Kafka` | `adapter/outbound/kafka/kafka.go`, `app/consumers.go`, `testutil/brokertest` | `kafka` | | any consumer or publisher exists |
| `otel` | `telemetry:` | exporters in `app/infra.go` | `utils/tracing`, `utils/metrics` regenerated, HTTP middleware span, `docker-compose.grafana.yml` | collector, Loki, Tempo, Mimir, Grafana | | never |
| `auth` (`apikey` or `jwt`) | `auth:` | | `middleware/auth_<driver>.go`, `utils/jwt` for jwt, `routes.go` group | | `http` not `none` | never |
| `s3` | `s3:` | `i.S3` | `adapter/outbound/s3/` | `minio` | | any use |
| `jobs` | `jobs:` | | `adapter/inbound/jobs/`, `app/jobs.go` | | | any `gen job` exists |
| `gen consumer`, `gen publisher` | | | per feature | | a broker | |
| `gen cache` | | | per feature | | `redis` | |

Every record also lists the Go modules to `go get` and the `.env.example` lines. `gox doctor` reports a capability whose files are missing or whose config block was removed by hand.

## 6. Command set

| Command | Effect |
|---|---|
| `gox new [name] [--module] [--db] [--http] [--log] [--with a,b] [--auth] [--git] [--defaults]` | project; asks for whatever is missing |
| `gox gen feature <Name> [field:type ...]` | model, domain, both ports, HTTP adapter, database adapter, memstore, tests, migration, errors, wiring |
| `gox gen consumer <Feature>` | consumer port and adapter, mode, test |
| `gox gen publisher <Feature>` | publisher port and adapter |
| `gox gen cache <Feature>` | cache port, adapter and fake |
| `gox gen command <Feature> <name>` | command port method, adapter, mode |
| `gox gen client <Provider>` | external API port and client |
| `gox gen job <name>` | ticker job [jobs] |
| `gox gen migration <name>` | empty pair |
| `gox gen openapi` | `docs/openapi.yaml` from inputs and routes |
| `gox add otel\|redis\|kafka\|rabbitmq\|auth\|s3\|jobs`, `gox remove <cap>` | config block, infra, compose service, env lines, tool files |
| `gox set log <driver>` | regenerates `app/logger.go` |
| `gox destroy feature <Name>` | everything the feature owns, including its port files and adapters of every kind |
| `gox sync`, `gox check`, `gox doctor`, `gox templates list\|eject\|diff` | as today |

Global flags as today. Everything mutating plans, prints, verifies and rolls back as today.

## 7. Scenarios

Each scenario: what you run, what gox writes, what you edit by hand.

1. **Plain CRUD API.** `gox new api --db postgres`, `gen feature`. Edit nothing; `make run`. The default log is JSON, telemetry off, no auth.
2. **Local development.** `--log slog-text` or `APP_SERVER__LOG_DRIVER=slog-text`; `--db sqlite` needs no container; `make run` applies migrations.
3. **API plus worker.** `add kafka`, `gen publisher order`, `gen consumer order`. Edit `domain/order/domain.go`: add `pub port.OrderPublisher` to `New`, call it after create; `gox sync`. Deploy the same image twice: `http` and `consume order`.
4. **Worker only.** `gox new ingest --http none --with kafka`, `gen feature event --no-http`, `gen consumer event`. No handler layer, no router dependency; `modes.go` defaults to `consume`.
5. **CLI tool with a database.** `--http none`, `gen command report monthly`. `go run ./cmd command report monthly 2026-11`.
6. **Read-heavy endpoint.** `add redis`, `gen cache fleet`. Edit the domain's `Get` to try the cache first (the generated cache adapter and fake are ready); `gox sync` wires the cache into `New`.
7. **Calls another service.** `gen client corebank`; fill the interface in `port/outbound/corebank.go` and the methods in the client; add `bank port.CoreBank` to a domain's `New`; `gox sync`. Errors from the client are catalogue errors prefixed `ErrCoreBank*` appended by `gen client`.
8. **Internal plus public endpoints.** `add auth --driver apikey` for the internal group; later `add auth --driver jwt` when an identity provider arrives. Public health routes stay open.
9. **Observability.** `add otel`; `docker compose -f docker-compose.grafana.yml up`. Nothing in the feature code changes; spans and metrics were already called.
10. **Switching the logger.** `gox set log zap`; one tool file regenerated.
11. **Two repositories in one transaction.** Inject `port.Transactor` into the domain; wrap the calls in `InTx`; repositories pick the transaction from the context.
12. **Adding a field later.** Edit `FleetInput` in `model/fleet.go`, the SQL constants in the repository, `gen migration add_fleet_colour`, the memstore and Mongo document if any. Nothing else refers to the field. (A `gen field` codemod for the model and SQL constants is a candidate for 2.x.)
13. **Partial update or immutable field.** Split `FleetInput` into a separate `FleetUpdate` by hand and change `Update` in the port, domain and handler; `gox check` keeps passing because none of these are tool files.
14. **A constructor changes.** Any `New` gains or loses a parameter: `gox sync`. A parameter with no provider or with two candidates is reported, nothing written.
15. **Editing a tool file.** `gox` refuses the next run; `--adopt` makes the file yours, `--force` regenerates it.
16. **Removing a feature.** `destroy feature fleet` removes model, domain, both port files, every adapter of every kind, tests, memstore, migration pair only if unapplied (it warns otherwise), the three errors, and regenerates `app`. Edited files are listed and left.
17. **Team conventions.** `templates eject feature/domain.go.tmpl` once, commit `.gox/templates`, every teammate's `gen feature` follows it; `templates diff` shows drift from upstream after a gox upgrade.
18. **CI.** `gox check`, `make lint`, `go test -race ./...` with the database as a service container; `-short` on runners without Docker.
19. **Deployment.** Distroless image; `config.yaml` baked in, secrets by `APP_` variables; one container per mode in compose or one Deployment per mode in Kubernetes.
20. **Monorepo with several services.** `gox new -C services/ billing`; each service has its own module, `gox.yaml` and manifest.
21. **Mongo instead of SQL.** Same commands; no migrations, optimistic updates on `updated_at`, `Transactor` is a session or pass-through.
22. **Upgrading gox.** `gox upgrade` regenerates tool files for the new version and reports once-files whose template changed, with `templates diff` to see what; three-way merge of once-files is out of scope.
23. **AI-assisted work.** The generated `AGENTS.md` carries the tree, the ownership rule, the dependency rule, the flows and the command table. Teams that use Spec Kit run `specify init` themselves; gox does not bundle it.
24. **A second database for one feature.** Not supported; one database per service. A feature that needs another store gets it as an external API client or a cache.
25. **gRPC or OpenAPI first.** `gen openapi` emits the contract from what exists; a gRPC inbound adapter kind is 2.x and follows the consumer pattern.

## 8. Implementation plan

| Step | Work | Effort |
|---|---|---|
| 1 | Layout: `model`, `domain`, per-feature ports with `FleetUseCases`, `adapter/inbound/http`, `adapter/outbound/<db>`, `utils/{log,masking,activity,id,tracing,metrics}` as no-ops, `app/modes.go` with `http` and `migrate`, `--http none`, `--log` with the four backends, `Transactor`; end-to-end suite on all five databases | 3 to 4 days |
| 2 | `add otel`, `add auth` (apikey, jwt), `set log`, generated CI workflow, `SECURITY.md` | 2 days |
| 3 | `add kafka` (franz-go) with `gen publisher`, `gen consumer`, `brokertest`, `consume` mode | 2 days |
| 4 | `add redis` with `gen cache` and the fake; `gen command`; `gen client` | 2 days |
| 5 | `gen openapi`; `add s3`; `add jobs` with `gen job`; `rabbitmq` as the second broker | 3 days |
| 6 | `gox upgrade`, docs site, release | as planned |

No generated project exists on the 0.4 tree, so there is no migration path to write; 0.4 is replaced.

## 9. Decisions to confirm

1. Replace the 0.4 tree with this one. Recommended.
2. `domain/` as the use-case layer, `FleetUseCases` as its inbound interface in `port/inbound`.
3. Response `data` is the entity; no separate response struct.
4. Logging defaults to `slog-json` with `zap`, `zerolog` and `slog-text` selectable; all code typed on `*slog.Logger`.
5. Telemetry calls always present as no-ops; `add otel` makes them real.
6. `internal/utils/<topic>` with the import rule of 5.9; connections never live there.
7. Kafka first, RabbitMQ second, both behind the same publisher and consumer ports.
8. Step 1 first, alone, before any capability.
