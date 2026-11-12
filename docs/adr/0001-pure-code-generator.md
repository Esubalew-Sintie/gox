# ADR 0001: gox is a pure code generator, not a runtime

- Status: accepted
- Date: 2026-11-11
- Plan: docs/plan.md, section 1

## Context

The PRD lists "not a runtime framework" as a non-goal. On 2026-11-04 a two-layer design was proposed anyway: a runtime kit module imported by generated services plus a CLI. On 2026-11-10 the project owner rejected it. The goal is the experience of a NestJS-style CLI for Go, where the developer writes business logic and the tool writes everything else, without the generated service depending on the tool.

## Decision

gox is a command-line generator and nothing else. Everything it emits is ordinary Go that compiles, tests and belongs to the developer. No generated file imports a gox package. Dependencies are constructor arguments in a generated composition root. There is no reflection container, no `init()` registration and no blank import for side effects.

Priorities, in order: explicit over magical; ownership of every file is explicit; every command is idempotent and reversible; generation is verified or rolled back; minimal by default, with every library behind a capability the developer chose; architecture is asked, implementation is decided by gox.

## Consequences

- Every behaviour of a generated project must be expressible as a template or as a codemod of a known file (ADR 0005). There is no library to put shared code in, so shared code is generated into each project.
- Upgrading a project means regenerating its tool-owned files, not bumping a dependency.
- The template surface is larger than a kit would need. This is the accepted cost.

## Alternatives considered

- **Runtime kit plus CLI.** Smaller templates, but a framework dependency in every service and wiring hidden behind the kit. Rejected by the owner on 2026-11-10.
- **google/wire or uber/fx for wiring.** wire needs hand-maintained provider sets and is in maintenance mode; fx uses reflection at start-up. Both put the wiring somewhere other than plain Go the developer can read.
