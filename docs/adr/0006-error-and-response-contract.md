# ADR 0006: The response envelope, `ErrorMap` and `ErrorTypeMap`, no `ErrorCodeMap`

- Status: accepted
- Date: 2026-11-11
- Plan: docs/plan.md, section 3.7; docs/errors-and-response.md

## Context

The sample service writes every response through one success function and one error function and resolves errors through three maps keyed by the error value: status, type, and code with name. Reading it showed defects: a wrapped error misses all three maps, so no layer can wrap and causes are dropped; an unmapped error returns `err.Error()` to the client, and the shared library's sanitiser is a substring list in which `:` matches almost everything; `ErrInternalServerError` maps to 400; the two maps have drifted; nearly every code collapses to `GEN_019`; the encode-failure path writes headers twice; success is always 200. On 2026-11-11 the owner decided to keep `ErrorMap` and `ErrorTypeMap`, remove `ErrorCodeMap`, keep every error in one file, and keep ozzo-validation.

## Decision

Wire format, unchanged from the sample except for the removed fields: success `{status, message, data, meta?, has_retry}`; error `{status, error: {type, message, details?}, has_retry}`; 200 for reads and updates, 201 for creates.

`internal/domain/errors.go` holds `ErrorType` constants, `errors.New` sentinels with user-facing messages, `ErrorMap map[error]int` and `ErrorTypeMap map[error]ErrorType`, as the sample writes them. Feature errors are appended by `gen feature` with a feature prefix (ADR 0005). Around the maps: `Lookup` walks the wrap chain and returns the first catalogue entry with its status and type, guarding the map lookup against uncomparable error types; `Wrap(entry, cause)` keeps the entry's message for the wire and the cause for the log; `Retryable` derives `has_retry` for timeout, unavailable, rate limit and concurrent update; `ValidationError` carries field details and unwraps to `ErrValidationFailed`.

`internal/handler/http/response/` has the only two writers, `Success` and `Error`. `Error` resolves through `Lookup`, falls back to `ErrInternalServerError` with its fixed message for anything unknown, logs the full error with the request id through `slog.Default()`, encodes to a buffer and writes headers once. `internal/handler/http/request/` has `Decode`, with a 1 MiB limit and unknown-field rejection, which runs the DTO's ozzo `Validate` method and converts `validation.Errors` into one `ValidationError`, fields ordered by name. `internal/storage/postgres/errors.go` has `Translate(err, notFound)`, which maps pgx errors onto catalogue entries and keeps the pgx error as cause.

The whole folder was compiled, vetted and tested in a standalone module before being specified.

## Consequences

- `code` and `name` leave the error object; clients key on `type`.
- Services may wrap errors with `fmt.Errorf("...: %w", err)`; the maps still resolve.
- The eighteen database sentinels of the sample are replaced by one translation function.
- A generated `TestMapsAgree` fails when the two maps disagree.
- The USSD branch of the sample's writer is not generated; `Option` is the extension point.

## Alternatives considered

- **One typed error carrying kind, code, name, message, retry and cause.** Designed and verified on 2026-11-11, then rejected by the owner in favour of the two maps the team already writes.
- **Three maps as in the sample.** `ErrorCodeMap` carried no information once every entry shared a few codes.
- **RFC 9457 problem details.** A different wire format from what the team's clients expect.
- **200 for every success.** The sample's behaviour; 201 for create was accepted.
