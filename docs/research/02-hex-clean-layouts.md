# Hexagonal vs Clean Architecture in Go (2024–2026): what a generator should emit

Star counts/dates checked 2026-11-10.

## 1. Recommended trees and dependency rules

### 1a. Hexagonal (ports & adapters)
Synthesised from Cockburn (driving/driven terminology), bagashiz/go-pos (internal/core/{domain,port,service}, internal/adapter/{handler/http,repository/postgres,cache/redis}), Three Dots Labs Wild Workouts (6.5k stars, pushed 2026-08-27), Ben Johnson's WTF Dial.

```
myapp/
├── cmd/api/main.go                 # composition root: build adapters, inject into core, run
├── config/                         # env/file config structs
├── internal/
│   ├── core/                       # the hexagon; imports nothing outside core (+ stdlib)
│   │   ├── domain/                 # entities, value objects, domain errors, invariants
│   │   │   └── order/              # one package per aggregate
│   │   ├── port/                   # driven ports: OrderRepository, EventPublisher, Clock…
│   │   └── service/                # application services = driving-port implementations
│   │       └── order/
│   ├── adapter/
│   │   ├── driving/                # primary: http, grpc, consumer/kafka
│   │   └── driven/                 # secondary: postgres, mongo, redis, kafka publisher, memory fakes
│   └── platform/                   # shared kernel: logger, otel, httpserver, errs, page
├── api/                            # openapi.yaml, *.proto
├── migrations/
└── pkg/                            # optional; only for code meant to be imported elsewhere
```
Dependency rule: adapter/* -> core/{service,port,domain}; core/service -> core/{port,domain}; core/domain -> nothing. cmd imports everything.

Naming variants: core|domain|root package; port|ports|contracts|consumer-side interfaces; service|application|app|usecase; adapter(s)|infrastructure|driver+driven|handler+repository. Three Dots use `ports/` for DRIVING adapters and `adapters/` for driven, opposite of Cockburn; a generator must pick one meaning and document it.

### 1b. Clean Architecture
Synthesised from Martin, bxcodec/go-clean-arch v4 (10.2k stars, last push 2024-04-24), evrone/go-clean-template (7.7k, pushed 2026-10-26), amitshekhariitbhu/go-backend-clean-architecture (6.2k), sklinkert/go-ddd, Wild Workouts app/{command,query}.

```
myapp/
├── cmd/app/main.go
├── config/
├── internal/
│   ├── app/                        # wiring + graceful shutdown
│   ├── entity/                     # enterprise rules: entities, VOs, domain errors
│   ├── usecase/                    # application rules; one package per area; contracts.go holds needed interfaces
│   ├── controller/                 # inbound interface adapters: http/v1/{handler,request,response}, grpc/v1, kafka
│   ├── repo/                       # outbound: postgres/<x>, mongo/<x>, webapi/
│   └── pkg/ or /pkg                # frameworks & drivers wrappers: postgres pool, httpserver, logger, tracing
├── migrations/
├── api/ and docs/
└── integration-test/
```
Dependency rule: controller,repo -> usecase -> entity; app/main -> all. Only plain structs/DTOs cross boundaries. Interface placement varies: evrone usecase/contracts.go; bxcodec v4 consumer-side; amitshekhari/sklinkert in domain/.

## 2. Where things go
| Concern | Hexagonal | Clean |
|---|---|---|
| Entities/VOs/domain errors | core/domain/<aggregate> | entity/ or domain/<aggregate> |
| Transport DTOs | adapter/driving/http | controller/http/v1/{request,response} |
| Use-case input/output | core/service/<x> | usecase/<x> or app/command, app/query |
| Ports | core/port or beside consumer | beside consumer or domain/repository.go |
| Use cases/services | core/service | usecase |
| http/grpc adapters | adapter/driving/{http,grpc} | controller/{http,grpc} |
| postgres/mongo/redis/kafka | adapter/driven/<tech> | repo/<tech>; pkg/<tech> for client wrapper |
| Composition root | cmd/<bin>/main.go | cmd/app/main.go -> internal/app/app.go |
| Config | config/ env-based | same |
| Migrations | migrations/ at root or inside postgres adapter | migrations/ + app/migrate.go behind build tag |
| Shared kernel | internal/common or internal/platform | pkg/ |

go.dev guidance: everything under internal/, binaries under cmd/; pkg/ not mentioned. Russ Cox issue #117 on golang-standards/project-layout: "not a standard", pkg/ not idiomatic. Kat Zien 2023: drop cmd/ and pkg/ for small apps. So: internal/ + cmd/ yes; pkg/ only when genuinely exportable.

## 3. Cross-cutting concerns
Transactions: (1) UpdateFn closure on aggregate repo (Three Dots default): repo opens tx, SELECT FOR UPDATE, maps rows->domain, runs closure, persists; use case never sees sql.Tx. (2) Unit of work / transaction provider RunInTx(ctx, func(Stores) error) rebuilding repos from one tx (rednafi 2026; Three Dots TransactionProvider; ardanlabs BeginCommitRollback middleware + Business.NewWithTx). (3) Explicit WithTx(tx) or *sql.Tx params: anti-pattern. (4) Tx hidden in context: rejected. Generator advice: UpdateFn per aggregate repo + optional UnitOfWork/Transactor port with Stores struct; sqlc DBTX interface (pgx.Conn, pgxpool.Pool, pgx.Tx) makes rebinding trivial.

Domain errors -> HTTP: (a) sentinel errors + errors.Is in handler (bxcodec); (b) typed error with code (Ben Johnson wtf.Error{Code,Message}, ardanlabs errs.ErrCode -> httpStatus map); (c) slug errors matched with errors.As (Three Dots). Mapping lives in transport adapter; internal errors never expose messages; app layer wraps business errors.

Validation: syntactic DTO validation in inbound adapter (validator); business invariants in domain constructors with private fields. Three Dots warn against cross-field validator tags.

Pagination: no canonical placement. Recommendation: small transport-neutral Page/PageResult[T] in use-case layer; parsing/cursor encoding in HTTP adapter; repos accept plain struct.

Context propagation: ctx first param of every port method; request-scoped values via typed helpers in a mid/common package, never in domain.

Logging/metrics/tracing: decorators around handlers applied in composition root (Wild Workouts generic commandLoggingDecorator[C]); ardanlabs ExtBusiness; evrone traced repo wrapper; HTTP middleware. Domain has no logger dependency.

## 4. Testing per layer
- Domain: black-box table-driven tests, no mocks.
- Use cases: fakes/mocks of ports. Hand-written mocks (Three Dots, Ben Johnson), mockery (bxcodec), uber mock (evrone), counterfeiter. 2026 consensus: testify+mockery safe default, fewer mocks overall, reserve for external boundaries.
- Driven adapters: integration tests against real infra; one shared suite run against every implementation (Wild Workouts TestRepository loops over memory/Firestore/MySQL). testcontainers-go, docker-compose, dockertest. Build tags or -short.
- Driving adapters: httptest against router with fake service; component tests booting the whole service with in-memory adapters.
- Dependency-rule enforcement: go-cleanarch (stale 2021), go-arch-lint (YAML component graph), depguard in golangci-lint.

## 5. Pitfalls and mitigations
- Single model with json+gorm+validate tags -> separate transport/domain/storage models with explicit mapping; accept duplication.
- Interface pollution -> consumer-side, small, only at I/O boundaries (bxcodec v4 changelog is this migration).
- Mapping boilerplate -> generators absorb it (oapi-codegen, sqlc, protoc); keep mapping in adapters.
- Premature layering / "Java in Go" -> fewer layers; domain layer optional per module; if explaining takes >30 min it's too much.
- Transactions leaking into use cases, mock-heavy tests, cmd/+pkg/ cargo cult, deep nesting.

## 6. Hex vs Clean in Go, and the generator decision
In practice both produce the same Go code: inbound adapter -> application service/use case -> domain, with outbound deps behind core-owned interfaces wired in main. Real differences: (1) names; (2) interface placement (central port pkg vs consumer-side/domain-owned); (3) internal structure of core (Clean prescribes entities vs use cases; Go projects add CQRS command/query); (4) grouping axis: by bounded context/module first (Wild Workouts, ardanlabs, MarioCarrion: internal/<module>/{domain,app,ports,adapters}) vs by layer first (evrone, bxcodec).

Recommendation: ONE internal generation model, TWO naming presets (`--layout hexagonal|clean` changes directory/package names and interface placement only), plus two orthogonal switches: `--cqrs` (split usecase into command/query with read models) and `--group-by module|layer`. Keep internal/ + cmd/ always, pkg/ opt-in, migrations at root, DTO/transport models always separate from domain, error-code type + transport map, UpdateFn repos + optional unit of work, hand-written fakes in a memory adapter plus optional mock generator.

Unverified: Kat Zien 2023 interview body; Cockburn book year; Arvucore 2026 article; star counts from HTML pages.

## 7. Sources
bxcodec/go-clean-arch; evrone/go-clean-template; ThreeDotsLabs/wild-workouts-go-ddd-example; threedots.tech posts (introducing-clean-architecture, repository-pattern-in-go, database-transactions-in-go, common-anti-patterns, basic-cqrs, ddd-cqrs-clean-architecture-combined, too-modern-go-application, is-clean-architecture-overengineering, microservices-test-architecture, list-of-recommended-libraries); gobeyond.dev/standard-package-layout; benbjohnson/wtf; katzien/go-structure-examples, app-structure-examples; JetBrains 2023 Kat Zien interview; golang-standards/project-layout + issue #117; go.dev/doc/modules/layout; alistair.cockburn.us/hexagonal-architecture; blog.cleancoder.com clean-architecture; GitHub topic rankings; ardanlabs/service; MarioCarrion/todo-api-microservice-example; sklinkert/go-ddd; amitshekhariitbhu/go-backend-clean-architecture; khannedy/golang-clean-architecture; bagashiz dev.to hexagonal; rednafi.com/go/repo-txn-uow; roblaszczak/go-cleanarch; fe3dback/go-arch-lint; dev.to 2026 mocking comparison; codeartify from-hexagonal-to-clean; dev.to go-doesnt-force-clean-architecture.
