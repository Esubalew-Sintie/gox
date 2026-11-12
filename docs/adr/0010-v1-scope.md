# ADR 0010: v1.0 is phases 0 to 4 with all five databases

- Status: accepted
- Date: 2026-11-11
- Plan: docs/plan.md, section 9

## Context

The plan has seven phases. The owner had to choose how much is v1.0, whether all databases are in it, and whether the clean preset is.

## Decision

v1.0 is phases 0 to 4:

0. Decision records and repository hygiene (this record closes it).
1. Engine: manifest, model, templates with override, virtual-tree planner, transcript, atomic apply with build-and-vet verification and rollback, hash-based clobber detection.
2. The layout of ADR 0003 with memory storage and chi; `new`, `gen feature`, `destroy`; the codemod of ADR 0005; Makefile, Dockerfile, compose, lint config, `AGENTS.md`.
3. Databases: postgres, mysql, sqlite, mongo under `internal/storage/<db>`, all five; golang-migrate pairs; `gen migration`; testcontainers suites; transactional `Update`.
4. `sync` with resolution by type, `check`, `doctor`, `templates eject|diff`, the stdlib `ServeMux` option.

Capabilities (redis, kafka, s3, jobs, otel), `gen client`, gRPC, OpenAPI, `upgrade` and the clean preset follow in 1.x.

## Consequences

- Phase 3 is the largest phase. If it slips, mysql and sqlite move to 1.1 without touching anything else, because the SQL repository template is dialect-parameterised.
- `externalapi/` and `app/external_api.go` are generated empty by `gox new` in v1.0 so that `gen client` can fill them in 1.x without changing the tree.

## Alternatives considered

- **Drop mysql and sqlite from v1.** Rejected by the owner on 2026-11-11.
- **Capabilities in v1.0.** Deferred; each needs its own end-to-end test and compose service.
