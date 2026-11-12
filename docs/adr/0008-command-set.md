# ADR 0008: Command vocabulary and the field DSL

- Status: accepted
- Date: 2026-11-11
- Plan: docs/plan.md, section 5

## Context

The current gox has `new`, `g resource`, `g module`, `add`, `remove` and `doctor`, with two layouts. The unit of generation must match how the team thinks about work, and every command must be scriptable.

## Decision

The generation unit is the **feature**. Commands:

- `gox new <name>` with `--db`, `--http`, `--with`, `--module`, `--git`, `--defaults`; prompts only for missing choices.
- `gox gen feature <Name> [field:type ...]` writes every layer and regenerates the wiring; aliases `g f`, `module`, `resource`. Atomic generators `gen domain|service|repository|handler|cache|publisher|consumer|migration|middleware`.
- `gox gen client <Name> [--kind http|grpc] [--auth basic|bearer|apikey]` for external API clients.
- `gox add <capability>` and `gox remove <capability>` for redis, kafka, s3, jobs, otel, grpc, openapi.
- `gox destroy feature <Name>`, manifest-driven with per-file confirmation.
- `gox sync`, `gox check`, `gox doctor`, `gox templates list|eject|diff`, `gox upgrade`.
- Global flags: `--dry-run`, `--yes`, `--force`, `--allow-dirty`, `--no-interactive`, `--verbose`.

Field DSL: `name:type` with `string`, `text`, `int`, `int64`, `float64`, `bool`, `time`, `uuid`; a trailing `?` marks a field nullable (addendum 2026-11-11: `:null` is accepted as the shell-safe equivalent, because zsh globs an unquoted `?`); the generated `Validate` method applies `validation.Required` to non-nullable fields.

## Consequences

- `--arch` is absent in v1.0 (ADR 0003) and returns with the clean preset.
- Every mutating command plans first and can be run with `--dry-run` (ADR 0002).

## Alternatives considered

- **`resource` or `module` as the unit.** Kept as aliases so existing habits still work.
- **Annotations in code to drive generation.** Rejected: no markers in developer-owned code.
