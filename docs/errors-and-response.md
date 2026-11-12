# Standard error and response handling

This is the error and response folder gox generates into every project. It follows the sample service's `ErrorMap` and `ErrorTypeMap`, keeps every error in one file as the sample does, and removes `ErrorCodeMap`. Every file below was compiled, vetted and tested as a standalone module (`example.com/fleet`, Go 1.26, ozzo-validation v4, pgx v5) before it was written here. It is a specification for the generator, not code in gox yet.

## 1. What changes from the sample service

The sample resolves an error in `internal/constant/errors/errors.go` through three maps keyed by the error value: `ErrorMap` for the HTTP status, `ErrorTypeMap` for the `type` string, and `ErrorCodeMap` for `code` and `name`. The generated project keeps the first two and drops the third.

| Sample | Generated | Why |
|---|---|---|
| one `errors.go` with every sentinel and the maps | one `internal/domain/errors.go`, same shape | the team's habit; one place to read the whole catalogue |
| `ErrorMap map[error]int` | kept | status per error, as the team writes it today |
| `ErrorTypeMap map[error]ErrorType` | kept | `type` per error |
| `ErrorCodeMap`, `GetErrorCode`, the `ErrorCode*` and `ErrorName*` constants | removed | every entry collapsed to a handful of shared codes, so the field carried no information; `code` and `name` leave the wire |
| map lookup by the bare error value | `domain.Lookup` walks the wrap chain | a service may `fmt.Errorf("...: %w", err)` and the maps still resolve; the sample misses the map on any wrapped error |
| unknown error returns `err.Error()` as `400 bad_request` | unknown error returns `ErrInternalServerError`, `500 internal`, fixed message | driver and network text never reach the client, and no substring sanitiser is needed |
| the writer type-asserts `validation.Errors` from ozzo | ozzo stays, as a `Validate` method on each request DTO; `request.Validate` converts its `validation.Errors` into `*domain.ValidationError`, which unwraps to `ErrValidationFailed` | the writer sees one shape, and it goes through the same two maps |
| eighteen `ErrDatabase*` sentinels, mapped by hand | `postgres.Translate(err, notFound)` maps pgx errors to the general entries and keeps the pgx error as cause | one function instead of fifty-four map lines |
| the two maps can drift (the sample has entries in one but not the other) | gox writes both lines together; `TestMapsAgree` fails if they ever disagree | |
| `WriteHeader` called twice on encode failure | encode to a buffer first, then write headers once | |

The wire format stays the sample's envelope, minus `code` and `name` in the error object.

## 2. The folder

```
internal/
├── domain/
│   ├── errors.go                 codemod  ErrorType, every error, ErrorMap, ErrorTypeMap, Lookup, Wrap, Retryable, ValidationError
│   └── page.go                   once     PageRequest, PageMeta, NewPageMeta
├── handler/http/
│   ├── response/
│   │   ├── envelope.go           once     Envelope, ErrorBody, FieldError
│   │   └── response.go           once     Success, Error, WithMeta, WithRetry
│   └── request/
│       ├── request.go            once     Decode, Validate, MaxBodyBytes
│       └── page.go               once     ParsePage
└── storage/postgres/
    └── errors.go                 once     Translate(err, notFound); postgres.go and tx.go sit beside it
```

Ownership. `gox init` writes all of these. `internal/domain/errors.go` is the one `codemod` file in the folder: `gox gen feature fleet` appends a `// Fleet errors.` var block with `ErrFleetNotFound`, `ErrFleetInvalidCapacity` and `ErrFleetDuplicateName`, and one line per error under a `// Fleet` comment in each map; `gox gen client corebank` does the same with `ErrCoreBank*`; `gox destroy` removes exactly the declarations and map lines it added, found by the identifiers recorded in the manifest. Everything else in the file is yours: add an error with one `errors.New` and one line in each map, as in the sample.

Dependencies. `domain` imports only the standard library. `domain/<feature>` imports `domain` for its errors. `response` and `request` import `domain`. `postgres` imports `domain` and pgx. Only `request` and the DTO files import ozzo.

## 3. The files

### 3.1 The error types, the catalogue and the two maps

`internal/domain/errors.go`, shown after `gox init` and `gox gen feature fleet`.

```go
// Package domain holds the entities, the error catalogue and the shared value
// types of the service. It imports nothing from the other layers.
package domain

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
)

// ErrorType is the machine-readable class written to error.type on the wire.
type ErrorType string

const (
	ErrorTypeValidation      ErrorType = "validation"
	ErrorTypeBadRequest      ErrorType = "bad_request"
	ErrorTypeUnauthorized    ErrorType = "unauthorized"
	ErrorTypeForbidden       ErrorType = "forbidden"
	ErrorTypeNotFound        ErrorType = "not_found"
	ErrorTypeTimeout         ErrorType = "timeout"
	ErrorTypeConflict        ErrorType = "conflict"
	ErrorTypePayloadTooLarge ErrorType = "payload_too_large"
	ErrorTypeRateLimit       ErrorType = "rate_limit"
	ErrorTypeUnavailable     ErrorType = "unavailable"
	ErrorTypeExternal        ErrorType = "external_service"
	ErrorTypeInternal        ErrorType = "internal"
)

// General errors. Every message is user-facing and is what the client sees.
// `gox gen feature <name>` appends a <Name> block below and one line per error
// to each map; `gox destroy feature <name>` removes them. Add your own errors
// the same way: one errors.New and one line in each map.
var (
	ErrInternalServerError = errors.New("Internal server error. Please try again later.")
	ErrBadRequest          = errors.New("Bad request.")
	ErrInvalidData         = errors.New("Invalid input data.")
	ErrEmptyBody           = errors.New("Request body is empty.")
	ErrMalformedJSON       = errors.New("Request body is not valid JSON.")
	ErrValidationFailed    = errors.New("Validation failed.")
	ErrUnauthorized        = errors.New("Unauthorized.")
	ErrForbiddenRequest    = errors.New("Forbidden request.")
	ErrNotFound            = errors.New("Data not found.")
	ErrRequestTimeout      = errors.New("Request timeout.")
	ErrRequestCancelled    = errors.New("The request was cancelled.")
	ErrConflict            = errors.New("This data already exists.")
	ErrConcurrentUpdate    = errors.New("Another operation is in progress. Please try again in a moment.")
	ErrPayloadTooLarge     = errors.New("Request body is too large.")
	ErrRateLimitExceeded   = errors.New("Too many requests. Please try again later.")
	ErrServiceUnavailable  = errors.New("Service is temporarily unavailable. Please try again shortly.")
	ErrExternalService     = errors.New("An external service failed. Please try again later.")
)

// Fleet errors.
var (
	ErrFleetNotFound        = errors.New("Fleet not found.")
	ErrFleetInvalidCapacity = errors.New("Capacity must be greater than zero.")
	ErrFleetDuplicateName   = errors.New("A fleet with this name already exists.")
)

// ErrorMap maps errors to HTTP status codes.
var ErrorMap = map[error]int{
	ErrInternalServerError: http.StatusInternalServerError,
	ErrBadRequest:          http.StatusBadRequest,
	ErrInvalidData:         http.StatusBadRequest,
	ErrEmptyBody:           http.StatusBadRequest,
	ErrMalformedJSON:       http.StatusBadRequest,
	ErrValidationFailed:    http.StatusBadRequest,
	ErrUnauthorized:        http.StatusUnauthorized,
	ErrForbiddenRequest:    http.StatusForbidden,
	ErrNotFound:            http.StatusNotFound,
	ErrRequestTimeout:      http.StatusRequestTimeout,
	ErrRequestCancelled:    http.StatusBadRequest,
	ErrConflict:            http.StatusConflict,
	ErrConcurrentUpdate:    http.StatusConflict,
	ErrPayloadTooLarge:     http.StatusRequestEntityTooLarge,
	ErrRateLimitExceeded:   http.StatusTooManyRequests,
	ErrServiceUnavailable:  http.StatusServiceUnavailable,
	ErrExternalService:     http.StatusBadGateway,

	// Fleet
	ErrFleetNotFound:        http.StatusNotFound,
	ErrFleetInvalidCapacity: http.StatusBadRequest,
	ErrFleetDuplicateName:   http.StatusConflict,
}

// ErrorTypeMap maps errors to error types.
var ErrorTypeMap = map[error]ErrorType{
	ErrInternalServerError: ErrorTypeInternal,
	ErrBadRequest:          ErrorTypeBadRequest,
	ErrInvalidData:         ErrorTypeBadRequest,
	ErrEmptyBody:           ErrorTypeBadRequest,
	ErrMalformedJSON:       ErrorTypeBadRequest,
	ErrValidationFailed:    ErrorTypeValidation,
	ErrUnauthorized:        ErrorTypeUnauthorized,
	ErrForbiddenRequest:    ErrorTypeForbidden,
	ErrNotFound:            ErrorTypeNotFound,
	ErrRequestTimeout:      ErrorTypeTimeout,
	ErrRequestCancelled:    ErrorTypeBadRequest,
	ErrConflict:            ErrorTypeConflict,
	ErrConcurrentUpdate:    ErrorTypeConflict,
	ErrPayloadTooLarge:     ErrorTypePayloadTooLarge,
	ErrRateLimitExceeded:   ErrorTypeRateLimit,
	ErrServiceUnavailable:  ErrorTypeUnavailable,
	ErrExternalService:     ErrorTypeExternal,

	// Fleet
	ErrFleetNotFound:        ErrorTypeNotFound,
	ErrFleetInvalidCapacity: ErrorTypeValidation,
	ErrFleetDuplicateName:   ErrorTypeConflict,
}

// Info is what the catalogue knows about an error.
type Info struct {
	Err    error // the catalogue entry that matched; Err.Error() is the user-facing message
	Status int
	Type   ErrorType
}

// Lookup finds the first catalogue entry in err's wrap chain, so an error
// wrapped with fmt.Errorf("...: %w", err) or Wrap still resolves. ok is false
// when nothing in the chain is in ErrorMap.
func Lookup(err error) (Info, bool) {
	for err != nil {
		if reflect.TypeOf(err).Comparable() {
			if status, ok := ErrorMap[err]; ok {
				typ, ok := ErrorTypeMap[err]
				if !ok {
					typ = ErrorTypeInternal
				}
				return Info{Err: err, Status: status, Type: typ}, true
			}
		}
		switch u := err.(type) { //nolint:errorlint // walks the chain by hand, which is the point
		case interface{ Unwrap() error }:
			err = u.Unwrap()
		case interface{ Unwrap() []error }:
			for _, inner := range u.Unwrap() {
				if info, ok := Lookup(inner); ok {
					return info, true
				}
			}
			return Info{}, false
		default:
			return Info{}, false
		}
	}
	return Info{}, false
}

// Wrap attaches cause to a catalogue entry. Lookup, errors.Is and errors.As
// see both: the wire shows the entry's message, the log shows the whole chain.
func Wrap(entry, cause error) error {
	if cause == nil {
		return entry
	}
	return fmt.Errorf("%w (%w)", entry, cause)
}

// Retryable reports whether the client may repeat the request that failed
// with err: timeouts, unavailability, rate limits and concurrent updates.
func Retryable(err error) bool {
	info, ok := Lookup(err)
	if !ok {
		return false
	}
	switch info.Type {
	case ErrorTypeTimeout, ErrorTypeUnavailable, ErrorTypeRateLimit:
		return true
	}
	return errors.Is(info.Err, ErrConcurrentUpdate)
}

// FieldError describes one invalid field of a request.
type FieldError struct {
	Field       string
	Description string
}

// ValidationError carries field details. It unwraps to ErrValidationFailed,
// so ErrorMap and ErrorTypeMap resolve it to 400 / validation. Its message is
// the first field's description, which is what the sample service returns.
type ValidationError struct {
	Fields []FieldError
}

// NewValidationError builds a ValidationError from one or more field errors.
func NewValidationError(fields ...FieldError) *ValidationError {
	return &ValidationError{Fields: fields}
}

func (e *ValidationError) Error() string {
	if len(e.Fields) > 0 {
		return e.Fields[0].Description
	}
	return ErrValidationFailed.Error()
}

// Unwrap makes errors.Is(err, ErrValidationFailed) true and lets Lookup find
// the status and type.
func (e *ValidationError) Unwrap() error { return ErrValidationFailed }
```

Notes:

- `Lookup` is the one place the maps are read. It checks the error itself, then each error it unwraps, in order, so the outermost catalogue entry wins. It guards the map lookup with `reflect.TypeOf(err).Comparable()` because hashing an error with an uncomparable dynamic type, such as ozzo's `validation.Errors`, which is a map, would panic.
- `Wrap(entry, cause)` is `fmt.Errorf("%w (%w)", entry, cause)`. Both are in the chain: the wire shows `entry.Error()`, the log shows the whole string, `errors.Is` works for either.
- `Retryable` is how `has_retry` is computed. Timeouts, unavailability, rate limits and concurrent updates are retryable. `response.WithRetry()` overrides it for one response.
- `ValidationError` is the only typed error. Its `Unwrap` returns `ErrValidationFailed`, so `Lookup` finds `400` and `validation` through the maps like any other error; the writer additionally reads its `Fields`.
- An error that is in `ErrorMap` but not in `ErrorTypeMap` resolves with type `internal`; `TestMapsAgree` fails before that can ship.
- Feature errors are prefixed with the feature name, `ErrFleetNotFound`, the way the sample writes `ErrSitotaSessionNotFound`, because they share one package.

### 3.2 Pagination

`internal/domain/page.go`

```go
package domain

// PageRequest is a page number and size, parsed by the transport layer.
type PageRequest struct {
	Page  int
	Limit int
}

const (
	DefaultPageLimit = 10
	MaxPageLimit     = 100
)

// Normalized returns a copy with defaults and bounds applied.
func (p PageRequest) Normalized() PageRequest {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	return p
}

// Offset is the number of rows to skip.
func (p PageRequest) Offset() int {
	n := p.Normalized()
	return (n.Page - 1) * n.Limit
}

// PageMeta is the pagination block returned in "meta" on list responses.
// The JSON names match the sample service's shared MetaData type.
type PageMeta struct {
	TotalDocs     int64 `json:"total_docs"`
	Limit         int64 `json:"limit"`
	TotalPages    int64 `json:"total_pages"`
	Page          int64 `json:"page"`
	PagingCounter int64 `json:"paging_counter"`
	HasPrevPage   bool  `json:"has_prev_page"`
	HasNextPage   bool  `json:"has_next_page"`
	PrevPage      int64 `json:"prev_page,omitempty"`
	NextPage      int64 `json:"next_page,omitempty"`
}

// NewPageMeta computes the block from a total count and the request.
func NewPageMeta(total int64, req PageRequest) PageMeta {
	n := req.Normalized()
	limit := int64(n.Limit)
	page := int64(n.Page)

	var totalPages int64
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	} else {
		page = 1
	}

	m := PageMeta{
		TotalDocs:   total,
		Limit:       limit,
		TotalPages:  totalPages,
		Page:        page,
		HasPrevPage: page > 1,
		HasNextPage: page < totalPages,
	}
	if total > 0 {
		m.PagingCounter = (page-1)*limit + 1
	}
	if m.HasPrevPage {
		m.PrevPage = page - 1
	}
	if m.HasNextPage {
		m.NextPage = page + 1
	}
	return m
}
```

### 3.3 The envelope

`internal/handler/http/response/envelope.go`

```go
package response

import "example.com/fleet/internal/domain"

// Envelope is the one JSON shape every response has. Success fills Message,
// Data and optionally Meta; Error fills Error. HasRetry is always present.
type Envelope struct {
	Status   int        `json:"status"`
	Message  string     `json:"message,omitempty"`
	Data     any        `json:"data,omitempty"`
	Meta     any        `json:"meta,omitempty"`
	Error    *ErrorBody `json:"error,omitempty"`
	HasRetry bool       `json:"has_retry"`
}

// ErrorBody is the error object: the type from domain.ErrorTypeMap, the
// message of the matched catalogue entry, and field details for validation.
type ErrorBody struct {
	Type    domain.ErrorType `json:"type"`
	Message string           `json:"message"`
	Details []FieldError     `json:"details,omitempty"`
}

// FieldError is one entry of ErrorBody.Details.
type FieldError struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}
```

### 3.4 The writers

`internal/handler/http/response/response.go`

```go
// Package response writes every HTTP response of the service. Handlers call
// Success or Error and never build JSON themselves.
package response

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"example.com/fleet/internal/domain"
)

// RequestIDHeader is set by the request-id middleware on the response.
const RequestIDHeader = "X-Request-ID"

// Option adjusts an envelope before it is written.
type Option func(*Envelope)

// WithMeta attaches a meta block, usually domain.PageMeta.
func WithMeta(meta any) Option { return func(e *Envelope) { e.Meta = meta } }

// WithRetry forces has_retry to true.
func WithRetry() Option { return func(e *Envelope) { e.HasRetry = true } }

// Success writes a success envelope. Use http.StatusCreated for creates.
func Success(w http.ResponseWriter, r *http.Request, status int, message string, data any, opts ...Option) {
	env := Envelope{Status: status, Message: message, Data: data}
	for _, o := range opts {
		o(&env)
	}
	write(w, r, env)
}

// Error writes an error envelope. The status and type come from
// domain.ErrorMap and domain.ErrorTypeMap through domain.Lookup, which walks
// the wrap chain, so services may wrap with fmt.Errorf("...: %w", err). The
// message is the matched entry's text; a *domain.ValidationError also fills
// details. Anything not in the maps is written as ErrInternalServerError.
// The full error goes to the log with the request id, never to the client.
func Error(w http.ResponseWriter, r *http.Request, err error, opts ...Option) {
	info, known := domain.Lookup(err)
	if !known {
		info, _ = domain.Lookup(domain.ErrInternalServerError)
	}
	body := &ErrorBody{Type: info.Type, Message: info.Err.Error()}
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		body.Message = ve.Error()
		for _, f := range ve.Fields {
			body.Details = append(body.Details, FieldError{Field: f.Field, Description: f.Description})
		}
	}
	env := Envelope{Status: info.Status, Error: body, HasRetry: domain.Retryable(err)}
	for _, o := range opts {
		o(&env)
	}
	logError(r.Context(), r, w.Header().Get(RequestIDHeader), env.Status, info.Type, err)
	write(w, r, env)
}

// write encodes to a buffer first so an encoding failure can still produce a
// well-formed 500 and the headers are written exactly once.
func write(w http.ResponseWriter, r *http.Request, env Envelope) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(env); err != nil {
		info, _ := domain.Lookup(domain.ErrInternalServerError)
		logError(r.Context(), r, w.Header().Get(RequestIDHeader), info.Status, info.Type, err)
		buf.Reset()
		_ = json.NewEncoder(&buf).Encode(Envelope{
			Status: info.Status,
			Error:  &ErrorBody{Type: info.Type, Message: info.Err.Error()},
		})
		env.Status = info.Status
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(env.Status)
	_, _ = w.Write(buf.Bytes())
}

func logError(ctx context.Context, r *http.Request, requestID string, status int, typ domain.ErrorType, err error) {
	attrs := []any{
		slog.String("request_id", requestID),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", status),
		slog.String("type", string(typ)),
		slog.String("error", err.Error()),
	}
	switch typ {
	case domain.ErrorTypeInternal:
		slog.ErrorContext(ctx, "request failed", attrs...)
	case domain.ErrorTypeUnavailable, domain.ErrorTypeTimeout, domain.ErrorTypeExternal:
		slog.WarnContext(ctx, "request failed", attrs...)
	default:
		slog.InfoContext(ctx, "request rejected", attrs...)
	}
}
```

`Error` does, in order: resolve the error through the maps with `domain.Lookup`, falling back to `ErrInternalServerError`; take the message from the matched entry; if a `*domain.ValidationError` is in the chain, use its first field as the message and copy its fields into `details`; compute `has_retry` with `domain.Retryable`; apply options; log once with the request id, method, path, status, type and the full error text, at error level for `internal`, warn for `unavailable`, `timeout` and `external_service`, info for everything else; encode to a buffer and write headers and body once.

### 3.5 Decoding and validation

`internal/handler/http/request/request.go`

```go
// Package request decodes and validates HTTP input. It is the only package
// that imports ozzo-validation, so the rest of the service never sees it.
package request

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"example.com/fleet/internal/domain"
)

// MaxBodyBytes caps request bodies, as the sample service does with MaxBytesReader.
const MaxBodyBytes = 1 << 20

// Validator is implemented by request DTOs. The ozzo rules live in the
// method, next to the fields, as in the sample service:
//
//	func (r *createFleetRequest) Validate() error {
//		return validation.ValidateStruct(r,
//			validation.Field(&r.Name, validation.Required, validation.Length(1, 100)),
//			validation.Field(&r.Capacity, validation.Required, validation.Min(1)),
//		)
//	}
type Validator interface {
	Validate() error
}

// Decode reads a JSON body into dst, rejects unknown fields and oversized
// bodies, and runs dst's Validate method when it has one. Every failure is a
// catalogue error or a *domain.ValidationError, so handlers pass it straight
// to response.Error.
func Decode(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &maxErr):
			return domain.Wrap(domain.ErrPayloadTooLarge, err)
		case errors.Is(err, io.EOF):
			return domain.ErrEmptyBody
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
			return domain.Wrap(domain.ErrMalformedJSON, err)
		case errors.As(err, &typeErr):
			return domain.NewValidationError(domain.FieldError{
				Field:       typeErr.Field,
				Description: typeErr.Field + " has the wrong type, expected " + typeErr.Type.String(),
			})
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
			return domain.NewValidationError(domain.FieldError{Field: field, Description: field + " is not a known field"})
		default:
			return domain.Wrap(domain.ErrBadRequest, err)
		}
	}
	if dec.More() {
		return domain.ErrMalformedJSON
	}
	return Validate(dst)
}

// Validate runs v's ozzo rules when it implements Validator and converts the
// result into one *domain.ValidationError. Fields are listed in name order,
// nested fields as "parent.child", so the first message is stable; ozzo
// returns its errors as a map.
func Validate(v any) error {
	val, ok := v.(Validator)
	if !ok {
		return nil
	}
	err := val.Validate()
	if err == nil {
		return nil
	}
	var verrs validation.Errors
	if !errors.As(err, &verrs) {
		return domain.Wrap(domain.ErrInternalServerError, err)
	}
	return domain.NewValidationError(fields("", verrs)...)
}

func fields(prefix string, verrs validation.Errors) []domain.FieldError {
	keys := make([]string, 0, len(verrs))
	for k := range verrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []domain.FieldError
	for _, k := range keys {
		err := verrs[k]
		if err == nil {
			continue
		}
		name := k
		if prefix != "" {
			name = prefix + "." + k
		}
		var nested validation.Errors
		if errors.As(err, &nested) {
			out = append(out, fields(name, nested)...)
			continue
		}
		out = append(out, domain.FieldError{Field: name, Description: name + " " + err.Error()})
	}
	return out
}
```

`internal/handler/http/request/page.go`

```go
package request

import (
	"net/http"
	"strconv"

	"example.com/fleet/internal/domain"
)

// ParsePage reads ?page= and ?limit=, applying the domain defaults and bounds.
func ParsePage(r *http.Request) domain.PageRequest {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	return domain.PageRequest{Page: page, Limit: limit}.Normalized()
}
```

### 3.6 Database error translation

`internal/storage/postgres/errors.go`

```go
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"example.com/fleet/internal/domain"
)

// Translate turns a pgx error into a catalogue error with the original
// attached as cause, so the client sees the catalogue message and the log
// sees the driver text. notFound is the feature's own not-found error, for
// example domain.ErrFleetNotFound, so callers write:
//
//	if err := row.Scan(...); err != nil {
//		return nil, postgres.Translate(err, domain.ErrFleetNotFound)
//	}
func Translate(err error, notFound error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Wrap(notFound, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.Wrap(domain.ErrRequestTimeout, err)
	}
	if errors.Is(err, context.Canceled) {
		return domain.Wrap(domain.ErrRequestCancelled, err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return domain.Wrap(domain.ErrConflict, err)
		case "23503", "23502", "23514", "22P02", "22001": // fk, not null, check, invalid text, value too long
			return domain.Wrap(domain.ErrInvalidData, err)
		case "40001", "40P01": // serialization_failure, deadlock_detected
			return domain.Wrap(domain.ErrConcurrentUpdate, err)
		case "57014": // query_canceled (statement_timeout)
			return domain.Wrap(domain.ErrRequestTimeout, err)
		case "08000", "08003", "08006", "53300", "57P01": // connection errors, too_many_connections, admin_shutdown
			return domain.Wrap(domain.ErrServiceUnavailable, err)
		}
	}
	return domain.Wrap(domain.ErrInternalServerError, err)
}
```

## 4. On the wire

Success, 200 for reads and updates, 201 for creates. `meta` is present only on lists.

```json
{
  "status": 200,
  "message": "Fleets fetched successfully",
  "data": [ { "id": "…", "name": "North", "capacity": 12, "status": "ACTIVE" } ],
  "meta": { "total_docs": 25, "limit": 10, "total_pages": 3, "page": 1, "paging_counter": 1,
            "has_prev_page": false, "has_next_page": true, "next_page": 2 },
  "has_retry": false
}
```

Business or infrastructure error. `type` comes from `ErrorTypeMap`, `status` from `ErrorMap`, `message` is the matched entry's text.

```json
{
  "status": 404,
  "error": { "type": "not_found", "message": "Fleet not found." },
  "has_retry": false
}
```

Validation error. `message` is the first field error, as the sample returns, and `details` lists all of them. ozzo returns a map, so fields are ordered by name to keep the first message stable.

```json
{
  "status": 400,
  "error": { "type": "validation", "message": "capacity cannot be blank",
             "details": [ { "field": "capacity", "description": "capacity cannot be blank" },
                          { "field": "name", "description": "name cannot be blank" } ] },
  "has_retry": false
}
```

Unknown error. Anything not in the maps.

```json
{
  "status": 500,
  "error": { "type": "internal", "message": "Internal server error. Please try again later." },
  "has_retry": false
}
```

Retryable error. `has_retry` is true for `timeout`, `unavailable`, `rate_limit` and `ErrConcurrentUpdate`.

```json
{
  "status": 503,
  "error": { "type": "unavailable", "message": "Service is temporarily unavailable. Please try again shortly." },
  "has_retry": true
}
```

## 5. How each layer uses it

**Domain.** The entity package imports `domain` and returns the feature's errors from its constructor and methods.

```go
// internal/domain/fleet/fleet.go
func New(name string, capacity int) (*Fleet, error) {
	if capacity <= 0 {
		return nil, domain.ErrFleetInvalidCapacity
	}
	...
}
```

**Storage.** Every pgx error goes through `Translate` with the feature's not-found entry.

```go
func (r *repository) Get(ctx context.Context, id string) (*fleet.Fleet, error) {
	row := r.db.QueryRow(ctx, getFleet, id)
	var f fleet.Fleet
	if err := row.Scan(&f.ID, &f.Name, &f.Capacity, &f.Status); err != nil {
		return nil, postgres.Translate(err, domain.ErrFleetNotFound)
	}
	return &f, nil
}
```

A repository that detects a business conflict itself returns the feature's own error, wrapping the cause:

```go
if pgErr.Code == "23505" && pgErr.ConstraintName == "fleets_name_key" {
	return domain.Wrap(domain.ErrFleetDuplicateName, err)
}
```

**External API.** `gox gen client corebank` appends the provider's errors to the same file, `ErrCoreBankAccountNotFound` and so on. The client maps upstream failures to them, or to the general entries, and keeps the upstream response as cause.

```go
switch {
case resp.StatusCode == http.StatusNotFound:
	return nil, domain.Wrap(domain.ErrCoreBankAccountNotFound, fmt.Errorf("corebank %d: %s", resp.StatusCode, body))
case resp.StatusCode >= 500:
	return nil, domain.Wrap(domain.ErrExternalService, fmt.Errorf("corebank %d: %s", resp.StatusCode, body))
}
```

**Service.** Returns errors unchanged or wraps them with context. Wrapping is safe because `Lookup` walks the chain.

```go
f, err := s.repo.Get(ctx, id)
if err != nil {
	return nil, fmt.Errorf("get fleet %s: %w", id, err)
}
```

**Handler.** Never inspects errors. Two calls end every handler.

```go
func (h *fleetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createFleetRequest
	if err := request.Decode(r, &req); err != nil {
		response.Error(w, r, err)
		return
	}
	out, err := h.service.Create(r.Context(), req.toInput())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, r, http.StatusCreated, "Fleet created successfully", toResponse(out))
}

func (h *fleetHandler) List(w http.ResponseWriter, r *http.Request) {
	page := request.ParsePage(r)
	items, total, err := h.service.List(r.Context(), page)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "Fleets fetched successfully", toResponses(items),
		response.WithMeta(domain.NewPageMeta(total, page)))
}
```

**Middleware** (`internal/handler/http/middleware/`)**.** Auth returns `domain.ErrUnauthorized` or `domain.ErrForbiddenRequest` through `response.Error`. Recovery turns a panic into `response.Error(w, r, fmt.Errorf("panic: %v", v))`, which is unknown to the maps and so becomes the fixed 500.

**Logging.** `response.Error` logs through `slog.Default()`, which `app/logger.go` sets, so the writer takes no logger parameter.

## 6. Converting an existing catalogue

A sample entry with its three map rows and a database sentinel:

```go
ErrSitotaSessionNotFound = errors.New("Sitota not found.")
ErrorMap:     ErrSitotaSessionNotFound: http.StatusNotFound,
ErrorTypeMap: ErrSitotaSessionNotFound: ErrorTypeNotFound,
ErrorCodeMap: ErrSitotaSessionNotFound: {Code: ErrorCodeGEN019, Name: ErrorNameVALIDATION_FAILED},

ErrDatabaseNoRows = errors.New("Data not found.")   // plus three map rows
```

becomes, in `internal/domain/errors.go`:

```go
ErrSitotaSessionNotFound = errors.New("Sitota not found.")
ErrorMap:     ErrSitotaSessionNotFound: http.StatusNotFound,
ErrorTypeMap: ErrSitotaSessionNotFound: ErrorTypeNotFound,
// no code row; ErrDatabaseNoRows is gone, repositories call postgres.Translate(err, domain.ErrSitotaSessionNotFound)
```

| Sample | Generated |
|---|---|
| `internal/constant/errors/errors.go` | `internal/domain/errors.go`, same shape, package `domain` |
| `errors.ErrorMap`, `errors.ErrorTypeMap` | `domain.ErrorMap`, `domain.ErrorTypeMap` |
| `errors.ErrorCodeMap`, `errors.GetErrorCode` | removed |
| `errors.ErrDatabase*` | removed; `postgres.Translate` |
| `response.SendSuccessResponse(w, r, 200, msg, data, false, nil, meta)` | `response.Success(w, r, http.StatusOK, msg, data, response.WithMeta(meta))` |
| `response.SendErrorResponseWithMaps(w, err, false)` | `response.Error(w, r, err)` |
| `validation.Errors` handled inside the writer | the same ozzo rules in a `Validate` method on the DTO; `request.Decode` runs it and converts the result to `*domain.ValidationError` |
| `ErrorTypeDatabase`, `ErrorTypeNetwork`, `ErrorTypeInvalidFormat`, `ErrorTypeUnknown` | not generated; `database` and `network` are infrastructure detail the client cannot act on, `invalid_format` is `bad_request`, `unknown` is `internal` |

## 7. Tests that accompany the folder

`gox init` writes these next to the files. They pass in the scratch module.

| Package | Test | Checks |
|---|---|---|
| domain | `TestMapsAgree` | every error is in both maps |
| domain | `TestLookupThroughWrapping` | two levels of wrapping still resolve; `errors.Is` sees entry and cause; message is the entry's |
| domain | `TestLookupUnknownAndUncomparable` | unknown, nil and slice-typed errors return not found without panicking |
| domain | `TestValidationErrorResolves` | 400 validation, first field as message |
| domain | `TestRetryable` | unavailable, timeout, rate limit, concurrent update true; conflict, not found, unknown false |
| response | `TestSuccessWithMeta` | envelope and pagination block |
| response | `TestErrorWrappedCatalogueEntry` | 404 not_found "Fleet not found.", no `code`, no `details`, `has_retry` false |
| response | `TestErrorValidationDetails` | 400 validation with two details |
| response | `TestErrorUnknownIsHidden` | 500 internal with the fixed message |
| response | `TestRetryFlag` | derived true for unavailable, false for conflict, forced by `WithRetry` |
| request | `TestDecodeValid`, `TestDecodeValidationFields`, `TestDecodeNoValidator`, `TestDecodeUnknownField`, `TestDecodeWrongType`, `TestDecodeEmptyAndSyntax`, `TestDecodeTooLarge`, `TestParsePage` | decoding paths, ozzo messages in field order such as "status must be a valid value", DTOs without a `Validate` method |
| postgres | `TestTranslate` | eight pgx cases resolve to the right entry and status, keep their cause; deadlock is retryable; nil stays nil |

`internal/domain/errors_test.go`

```go
package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"example.com/fleet/internal/domain"
)

func TestMapsAgree(t *testing.T) {
	for err := range domain.ErrorMap {
		if _, ok := domain.ErrorTypeMap[err]; !ok {
			t.Errorf("%v has a status but no type", err)
		}
	}
	for err := range domain.ErrorTypeMap {
		if _, ok := domain.ErrorMap[err]; !ok {
			t.Errorf("%v has a type but no status", err)
		}
	}
}

func TestLookupThroughWrapping(t *testing.T) {
	cause := errors.New("no rows in result set")
	err := fmt.Errorf("get fleet 7: %w", domain.Wrap(domain.ErrFleetNotFound, cause))

	info, ok := domain.Lookup(err)
	if !ok || info.Err != domain.ErrFleetNotFound || info.Status != 404 || info.Type != domain.ErrorTypeNotFound {
		t.Fatalf("Lookup through two wraps: %+v %v", info, ok)
	}
	if !errors.Is(err, domain.ErrFleetNotFound) || !errors.Is(err, cause) {
		t.Fatal("errors.Is must see both the entry and the cause")
	}
	if info.Err.Error() != "Fleet not found." {
		t.Fatalf("wire message must be the entry's text, got %q", info.Err.Error())
	}
}

type sliceError []string

func (s sliceError) Error() string { return "slice" }

func TestLookupUnknownAndUncomparable(t *testing.T) {
	if _, ok := domain.Lookup(errors.New("dial tcp: connection refused")); ok {
		t.Fatal("unknown error must not resolve")
	}
	if _, ok := domain.Lookup(sliceError{"a"}); ok {
		t.Fatal("uncomparable error must not resolve, and must not panic")
	}
	if _, ok := domain.Lookup(nil); ok {
		t.Fatal("nil must not resolve")
	}
}

func TestValidationErrorResolves(t *testing.T) {
	err := domain.NewValidationError(
		domain.FieldError{Field: "name", Description: "name is required"},
		domain.FieldError{Field: "capacity", Description: "capacity must be greater than 0"},
	)
	info, ok := domain.Lookup(err)
	if !ok || info.Status != 400 || info.Type != domain.ErrorTypeValidation {
		t.Fatalf("validation error must resolve to 400 validation: %+v", info)
	}
	if !errors.Is(err, domain.ErrValidationFailed) || err.Error() != "name is required" {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestRetryable(t *testing.T) {
	cases := map[error]bool{
		domain.ErrServiceUnavailable:                     true,
		domain.ErrRequestTimeout:                         true,
		domain.ErrRateLimitExceeded:                      true,
		domain.ErrConcurrentUpdate:                       true,
		fmt.Errorf("tx: %w", domain.ErrConcurrentUpdate): true,
		domain.ErrConflict:                               false,
		domain.ErrFleetNotFound:                          false,
		errors.New("unknown"):                            false,
	}
	for err, want := range cases {
		if got := domain.Retryable(err); got != want {
			t.Errorf("Retryable(%v) = %v, want %v", err, got, want)
		}
	}
}
```

`internal/handler/http/response/response_test.go`

```go
package response_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/fleet/internal/domain"
	"example.com/fleet/internal/handler/http/response"
)

func do(fn func(w http.ResponseWriter, r *http.Request)) (int, map[string]any) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/fleets/1", nil)
	fn(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		panic(err)
	}
	return rec.Code, body
}

func TestSuccessWithMeta(t *testing.T) {
	code, body := do(func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, r, http.StatusOK, "Fleets fetched successfully", []string{"a"},
			response.WithMeta(domain.NewPageMeta(25, domain.PageRequest{Page: 1, Limit: 10})))
	})
	if code != 200 || body["status"].(float64) != 200 || body["message"] != "Fleets fetched successfully" {
		t.Fatalf("unexpected: %d %v", code, body)
	}
	meta := body["meta"].(map[string]any)
	if meta["total_pages"].(float64) != 3 || meta["has_next_page"] != true || meta["next_page"].(float64) != 2 {
		t.Fatalf("bad meta: %v", meta)
	}
	if _, ok := body["error"]; ok {
		t.Fatal("error must be omitted on success")
	}
}

func TestErrorWrappedCatalogueEntry(t *testing.T) {
	cause := errors.New("no rows in result set")
	err := fmt.Errorf("get fleet: %w", domain.Wrap(domain.ErrFleetNotFound, cause))
	code, body := do(func(w http.ResponseWriter, r *http.Request) { response.Error(w, r, err) })
	e := body["error"].(map[string]any)
	if code != 404 || e["type"] != "not_found" || e["message"] != "Fleet not found." {
		t.Fatalf("unexpected: %d %v", code, body)
	}
	if _, ok := e["code"]; ok {
		t.Fatal("error.code must not be on the wire")
	}
	if _, ok := e["details"]; ok {
		t.Fatal("details must be omitted when there are none")
	}
	if body["has_retry"] != false {
		t.Fatal("has_retry must be present and false")
	}
}

func TestErrorValidationDetails(t *testing.T) {
	err := domain.NewValidationError(
		domain.FieldError{Field: "name", Description: "name is required"},
		domain.FieldError{Field: "capacity", Description: "capacity must be greater than 0"},
	)
	code, body := do(func(w http.ResponseWriter, r *http.Request) { response.Error(w, r, err) })
	e := body["error"].(map[string]any)
	if code != 400 || e["type"] != "validation" || e["message"] != "name is required" {
		t.Fatalf("unexpected: %d %v", code, body)
	}
	details := e["details"].([]any)
	if len(details) != 2 || details[1].(map[string]any)["field"] != "capacity" {
		t.Fatalf("expected 2 details: %v", e)
	}
}

func TestErrorUnknownIsHidden(t *testing.T) {
	err := errors.New("dial tcp 10.0.0.5:5432: connection refused")
	code, body := do(func(w http.ResponseWriter, r *http.Request) { response.Error(w, r, err) })
	e := body["error"].(map[string]any)
	if code != 500 || e["type"] != "internal" || e["message"] != domain.ErrInternalServerError.Error() {
		t.Fatalf("unexpected: %d %v", code, body)
	}
}

func TestRetryFlag(t *testing.T) {
	_, body := do(func(w http.ResponseWriter, r *http.Request) { response.Error(w, r, domain.ErrServiceUnavailable) })
	if body["has_retry"] != true {
		t.Fatalf("unavailable must set has_retry: %v", body)
	}
	_, body = do(func(w http.ResponseWriter, r *http.Request) { response.Error(w, r, domain.ErrFleetDuplicateName) })
	if body["has_retry"] != false {
		t.Fatalf("conflict must not set has_retry: %v", body)
	}
	_, body = do(func(w http.ResponseWriter, r *http.Request) {
		response.Error(w, r, domain.ErrFleetDuplicateName, response.WithRetry())
	})
	if body["has_retry"] != true {
		t.Fatalf("WithRetry must force has_retry: %v", body)
	}
}
```

`internal/handler/http/request/request_test.go`

```go
package request_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"example.com/fleet/internal/domain"
	"example.com/fleet/internal/handler/http/request"
)

type createFleet struct {
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
	Status   string `json:"status"`
}

func (c *createFleet) Validate() error {
	return validation.ValidateStruct(c,
		validation.Field(&c.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&c.Capacity, validation.Required, validation.Min(1)),
		validation.Field(&c.Status, validation.In("ACTIVE", "INACTIVE")),
	)
}

func decode(body string) (createFleet, error) {
	r := httptest.NewRequest(http.MethodPost, "/fleets", strings.NewReader(body))
	var dst createFleet
	err := request.Decode(r, &dst)
	return dst, err
}

func TestDecodeValid(t *testing.T) {
	dst, err := decode(`{"name":"North","capacity":12}`)
	if err != nil || dst.Name != "North" || dst.Capacity != 12 {
		t.Fatalf("unexpected: %v %v", dst, err)
	}
}

func TestDecodeValidationFields(t *testing.T) {
	_, err := decode(`{"name":"","capacity":0,"status":"MAYBE"}`)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || !errors.Is(err, domain.ErrValidationFailed) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if ve.Error() != "capacity cannot be blank" || len(ve.Fields) != 3 {
		t.Fatalf("unexpected: %q %v", ve.Error(), ve.Fields)
	}
	if ve.Fields[2].Field != "status" || ve.Fields[2].Description != "status must be a valid value" {
		t.Fatalf("unexpected field message: %v", ve.Fields[2])
	}
}

func TestDecodeNoValidator(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
	var dst struct {
		A int `json:"a"`
	}
	if err := request.Decode(r, &dst); err != nil || dst.A != 1 {
		t.Fatalf("a DTO without Validate must decode: %v", err)
	}
}

func TestDecodeUnknownField(t *testing.T) {
	_, err := decode(`{"name":"North","capacity":1,"colour":"red"}`)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Fields[0].Field != "colour" {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestDecodeWrongType(t *testing.T) {
	_, err := decode(`{"name":"North","capacity":"twelve"}`)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Fields[0].Field != "capacity" {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestDecodeEmptyAndSyntax(t *testing.T) {
	if _, err := decode(``); !errors.Is(err, domain.ErrEmptyBody) {
		t.Fatalf("empty body: %v", err)
	}
	if _, err := decode(`{"name":`); !errors.Is(err, domain.ErrMalformedJSON) {
		t.Fatalf("syntax: %v", err)
	}
}

func TestDecodeTooLarge(t *testing.T) {
	big := `{"name":"` + strings.Repeat("x", request.MaxBodyBytes) + `","capacity":1}`
	if _, err := decode(big); !errors.Is(err, domain.ErrPayloadTooLarge) {
		t.Fatalf("too large: %v", err)
	}
}

func TestParsePage(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/fleets?page=3&limit=500", nil)
	p := request.ParsePage(r)
	if p.Page != 3 || p.Limit != domain.MaxPageLimit || p.Offset() != 200 {
		t.Fatalf("unexpected: %+v", p)
	}
	r = httptest.NewRequest(http.MethodGet, "/fleets", nil)
	if p := request.ParsePage(r); p.Page != 1 || p.Limit != domain.DefaultPageLimit {
		t.Fatalf("defaults: %+v", p)
	}
}
```

`internal/storage/postgres/errors_test.go`

```go
package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"example.com/fleet/internal/domain"
	"example.com/fleet/internal/storage/postgres"
)

func TestTranslate(t *testing.T) {
	cases := []struct {
		in     error
		want   error
		status int
	}{
		{pgx.ErrNoRows, domain.ErrFleetNotFound, 404},
		{fmt.Errorf("scan: %w", pgx.ErrNoRows), domain.ErrFleetNotFound, 404},
		{&pgconn.PgError{Code: "23505"}, domain.ErrConflict, 409},
		{&pgconn.PgError{Code: "23503"}, domain.ErrInvalidData, 400},
		{&pgconn.PgError{Code: "40P01"}, domain.ErrConcurrentUpdate, 409},
		{&pgconn.PgError{Code: "08006"}, domain.ErrServiceUnavailable, 503},
		{context.DeadlineExceeded, domain.ErrRequestTimeout, 408},
		{errors.New("something odd"), domain.ErrInternalServerError, 500},
	}
	for _, c := range cases {
		got := postgres.Translate(c.in, domain.ErrFleetNotFound)
		if !errors.Is(got, c.want) {
			t.Errorf("Translate(%v) = %v, want %v", c.in, got, c.want)
		}
		if !errors.Is(got, c.in) {
			t.Errorf("Translate(%v) lost its cause", c.in)
		}
		if info, ok := domain.Lookup(got); !ok || info.Status != c.status {
			t.Errorf("Translate(%v) resolves to %+v, want status %d", c.in, info, c.status)
		}
	}
	if !domain.Retryable(postgres.Translate(&pgconn.PgError{Code: "40P01"}, domain.ErrFleetNotFound)) {
		t.Error("deadlock must be retryable")
	}
	if postgres.Translate(nil, domain.ErrFleetNotFound) != nil {
		t.Error("nil must stay nil")
	}
}
```
