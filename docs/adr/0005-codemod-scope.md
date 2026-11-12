# ADR 0005: Seven codemod files and what gox may do to them

- Status: accepted
- Date: 2026-11-11
- Plan: docs/plan.md, section 4.1.1

## Context

The supplied layout keeps interfaces in single per-layer files, every error in one `errors.go` and every config field in one `config.go`. A feature or capability therefore has to add to files the developer also edits. The current injector does this with marker comments and import-spec surgery and has a known comment-stranding defect.

## Decision

Exactly seven files are codemod files: `internal/storage/repository.go`, `internal/storage/cache.go`, `internal/service/service.go`, `internal/handler/handler.go`, `externalapi/externalapi.go`, `internal/domain/errors.go`, `internal/config/config.go`.

gox performs these operations on them, through `go/ast` and `go/format`, followed by `imports.Process`:

- Interface files: append one `type XxxInterface interface { ... }` declaration; remove that declaration by name.
- `errors.go`: append one `var ( ErrFleet... )` block and one key-value entry per error at the end of the `ErrorMap` and `ErrorTypeMap` composite literals, each group under a feature comment; remove exactly those identifiers and entries.
- `config.go`: append one `type XxxConfig struct { ... }` declaration with `koanf` tags and one field on `Config`; remove them by name.

Only identifiers recorded in the manifest are ever removed. Anything the developer added, whatever its name, stays. Removal is refused, and the file listed for manual cleanup, when a declaration's method set, a map entry's value or a field's type no longer matches what gox wrote.

## Consequences

- The manifest records the identifiers gox added to each codemod file.
- The worst case of a refused removal is a leftover declaration that `gox doctor` reports.
- A generated `TestMapsAgree` guards the two error maps regardless of who edited them.
- `imports.Process` handles imports, so gox never edits an import spec itself.

## Alternatives considered

- **One interface file per feature** (`internal/storage/fleet.go`). Needs no codemod at all, but changes the supplied tree. Kept as the documented alternative to decision 3.
- **Marker comments.** Rejected, ADR 0002.
- **Regenerating the whole interface file.** Loses the methods developers add to interfaces.
