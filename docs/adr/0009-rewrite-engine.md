# ADR 0009: Rebuild the engine, remove the marker injector

- Status: accepted
- Date: 2026-11-11
- Plan: docs/plan.md, section 8

## Context

The current engine is `internal/generator` (about 1150 lines coupled to the `layered-rest` and `feature-first` layouts), `internal/injector` (marker comments and import-spec edits, with a reproduced comment-stranding defect), `internal/renderer`, `internal/filesystem` and `internal/config`. None of it knows about a manifest, ownership classes or verification. The layouts it serves are retired (ADR 0003).

## Decision

The engine is rebuilt as the packages of plan section 8: `manifest`, `model`, `templates`, `render`, `codemod`, `plan`, `apply`, `resolve`, `capability`, `db`, `transcript`, `cli`. `internal/injector` is removed. `internal/generator`, `renderer`, `filesystem` and `config` are replaced or absorbed. `internal/naming` survives. The version moves to 0.2.0 and reaches 1.0 at the end of phase 6.

Until phase 2 replaces them, the legacy packages stay buildable and tested so the repository is never broken; the lint configuration may exclude them, and the exclusion is removed with the packages.

## Consequences

- No attempt is made to keep the current `.gox.yaml` or marker-based projects upgradable; 0.1 projects are not migrated.
- Phase 1 builds the engine with no generated layout at all; the first end-to-end test arrives in phase 2.

## Alternatives considered

- **Extend the current generator.** The marker mechanism and the layout coupling are the two things the plan replaces; extending would carry both forward.
