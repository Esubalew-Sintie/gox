# ADR 0004: Generated composition root with fixed seams, `New` constructors, resolution by type

- Status: accepted
- Date: 2026-11-11
- Plan: docs/plan.md, sections 4.1 to 4.4

## Context

Wiring is what makes a generator worth using and what breaks first. The current gox injects into marker comments. The sample service wires everything by hand in `initiator`, one `Init*` function per file, called in order from one entry function. gox must produce that shape and keep it correct as features and capabilities are added, without the developer touching the wiring.

## Decision

The `app` package has two kinds of files. Developer-owned: `app.go` with `Initialize`, which is the only file that knows the order; `config.go`; `logger.go`; one file per infrastructure client (`postgres.go`, `redis.go`, `kafka.go`) so pool sizes and options can be tuned by hand. Tool-owned, regenerated from `gox.yaml`: `infra.go`, `external_api.go`, `storage.go`, `cache.go`, `service.go`, `handler.go`, `routes.go`, exposing seven functions whose signatures never change after `gox new`: `InitInfra`, `InitExternalAPI`, `InitStorage`, `InitCache`, `InitService`, `InitHandler`, `InitRoutes`. Each hands the next layer only what it needs; nothing below `app` sees the aggregate structs.

Feature packages expose a `New(...)` constructor returning the concrete type, with `var _ storage.FleetRepository = (*Repository)(nil)` beside it, and HTTP handlers expose `Routes()`. The `app` package keeps the sample's `Init*` names for its own wiring functions.

When a developer changes a constructor, `gox sync` (run automatically by `gen` and `add`) type-checks the feature packages with `go/packages`, reads every `New` signature, checks with `types.Implements` that the returned type satisfies the interface field it is assigned to, and resolves each parameter by type against the known providers: `Infra`, `ExternalAPI`, `Storage`, `Cache`, `Service`, `Config`, the logger. A parameter with no provider is an error naming the missing port. If a package does not type-check, gox regenerates from the signatures recorded in the manifest and prints the compiler error. Health and readiness are registered in `InitRoutes` from the clients present in `Infra`.

## Consequences

- Adding a feature regenerates four `app` files and edits none of the developer's.
- Resolution by type is the most complex engine piece; it ships in phase 4 behind the manifest fallback so phases 2 and 3 do not wait for it.
- No marker comments exist anywhere in developer-owned code.

## Alternatives considered

- **`Init<Feature><Layer>` returning the interface**, the sample's style, was the plan's default until 2026-11-11, when the owner chose idiomatic `New`.
- **google/wire, uber/fx**: see ADR 0001.
- **Marker injection** (current): see ADR 0002.
