# ADR 0007: Library defaults of generated projects

- Status: accepted
- Date: 2026-11-11
- Plan: docs/plan.md, section 6; docs/research/04-tech-defaults.md

## Context

Every library in a generated project is a long-term commitment for the team that owns the project. The research in docs/research weighed maintenance, Go version policy, API stability and what the sample service already uses. The owner decided the open points on 2026-11-11.

## Decision

| Concern | Default | Selectable alternative |
|---|---|---|
| Router | chi v5 | net/http `ServeMux` |
| Validation | ozzo-validation v4, a `Validate` method per request DTO | go-playground/validator v10 |
| Postgres | pgx v5, SQL in the repository | sqlc |
| MySQL, SQLite | database/sql with go-sql-driver/mysql, modernc.org/sqlite | |
| MongoDB | mongo-go-driver v2 | |
| Migrations | golang-migrate v4, `.up.sql` and `.down.sql` pairs embedded in the binary | goose v3 |
| Config | koanf v2, `config.yaml` then environment | caarlos0/env |
| Logging | log/slog, JSON handler | zap |
| Kafka | franz-go | IBM/sarama |
| Object storage | aws-sdk-go-v2 S3 client, custom endpoint for MinIO | |
| Jobs | standard library ticker scheduler | |
| Tests | standard `testing`, testcontainers-go behind `-short` | testify |
| Fakes | generated `testutil/memstore` | mockery |
| gRPC | buf, protoc-gen-go, connect-go v2 | |
| OpenAPI | off; huma v2 on request | |
| Lint | golangci-lint v2 with depguard | |
| Go | `go 1.26.0` | |

Excluded on purpose: gin, echo, fiber, GORM in core, Viper, sqlx, google/wire, uber/fx, `init()` registration.

## Consequences

- Validation rules live in a method next to the DTO fields, as in the sample service, and ozzo's map result is converted into one ordered `ValidationError` (ADR 0006).
- Every default is behind a capability so it can be swapped when a dependency's health changes.
- Generated projects carry a test that guards the Go 1.27 json/v2 behaviour change.

## Alternatives considered

Listed in the table. zap, sarama and ozzo are what the sample service uses; the owner kept ozzo and accepted slog and franz-go.
