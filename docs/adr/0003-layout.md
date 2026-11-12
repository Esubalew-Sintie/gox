# ADR 0003: The project layout, hexagonal only in v1.0

- Status: accepted
- Date: 2026-11-11
- Plan: docs/plan.md, sections 3.1 to 3.6

## Context

Two layouts were proposed and rejected: an `internal/core` and `internal/adapter` nesting, then golang-standards/project-layout with `internal/app` and `pkg/`. On 2026-11-11 the project owner supplied the tree gox must generate, modelled on the `initiator` package of a production service, and revised it the same day. The team wants gox to reproduce what it already knows, not a textbook.

## Decision

The tree in plan section 3.1, exactly as supplied. Its defining points:

- `app/` is the composition root, one file per concern with an `Init*` function each (ADR 0004).
- `externalapi/` holds external API clients, one folder per provider, with interfaces in `externalapi/externalapi.go`. It is the sample's `platform/` folder, renamed; the folder and the package share the name.
- `internal/domain/` holds entities per feature, `page.go`, and one `errors.go` with every error and both maps (ADR 0006). Entity packages import only `domain`.
- `internal/service/`, `internal/handler/`, `internal/storage/` each have single interface files: `service.go`, `handler.go`, `storage/repository.go`, `storage/cache.go`, `storage/publisher.go`.
- HTTP code is under `internal/handler/http/`: `middleware/`, `request/`, `response/` and one folder per feature.
- `internal/storage/postgres/` holds the pool, the transaction helper and the pgx error translation in package `postgres`, with one folder per feature below it. Other adapters sit beside it: `redis/`, `kafka/`, `object_storage/`, `jobs/`, `observability/`.
- `internal/config/config.go` is the single config file. `config.yaml`, `.env.example`, `AGENTS.md`, `.golangci.yml` live at the root.
- Tests use `internal/testutil/memstore/`, one file per feature.

A dependency rule (plan section 3.1 table) is enforced by a generated depguard configuration: `domain` imports only the standard library; implementations depend on interfaces, never on other implementations; `app` is the only package that knows concrete types.

v1.0 ships this tree only. A Clean Architecture preset that renames `domain`, `service` and `handler` to `entity`, `usecase` and `controller`, and nothing else, follows after v1.0 as `--arch clean`.

## Consequences

- `internal/domain/errors.go` and `internal/config/config.go` become codemod files (ADR 0005), because features and capabilities add to them.
- Several packages are named `fleet`; the generated `app` files use import aliases.
- Feature errors are prefixed, `ErrFleetNotFound`, because they share one package.
- The `layered-rest` and `feature-first` layouts of the current gox are retired; `feature-first` returns later as `--group-by module`.

## Alternatives considered

- **golang-standards/project-layout** with `pkg/`, `internal/app`. Rejected by the owner.
- **Textbook hexagonal names** (`core`, `adapter`, `port`). Rejected by the owner.
- **One errors file per feature** and **`configs/` directory with `config/app.go`**. Present in the first revision, removed at the owner's request.
- **Clean preset in v1.0.** Deferred on 2026-11-11 to shrink phase 2.
