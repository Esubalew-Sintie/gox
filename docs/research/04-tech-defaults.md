# gox default-stack research (as of 2026-11-10)

Method: web search + official release notes, GitHub feeds / raw go.mod files pulled 2026-11-10. Heads-up: Go 1.27 shipped Aug 2026; local toolchain 1.26.3 is one release behind.

## Recommendation table

| Concern | Default | Alternatives offered | Rationale | Maintenance evidence |
|---|---|---|---|---|
| SQL access (Postgres) | pgx v5 (pgxpool; pgx/v5/stdlib when database/sql needed) + sqlc (output committed by generator) | plain pgx/database/sql hand-written SQL; GORM (generics API gorm.G[T], v1.30+); Ent; Bun; go-jet. Do not offer sqlx. | sqlc: compile-time-typed queries, zero runtime dep beyond pgx; build-time tool. GORM most popular (39.9k stars) but reflective; Ent type-safe but single-vendor (Ariga). sqlx dormant. | pgx v5.12.0 2026-10-31. sqlc v1.31.1 2026-04-22, commits to 2026-10-23, 594 issues. GORM v1.31.2 2026-06-25. Ent v0.14.6 2026-03-23. Bun v1.3.0 2026-10-03. go-jet v2.16.1 2026-10-06. sqlx last tag v1.4.0 2024-04-23. |
| Migrations | goose v3 (library + CLI; embed.FS via goose.NewProvider/SetBaseFS; Postgres/MySQL/SQLite; SQL or Go migrations) | golang-migrate (iofs, widest drivers, 317 issues backlog); tern (Postgres-only); dbmate (CLI-first); Atlas (declarative; migrate lint paywalled at v0.38). River is a job queue, not migrations. | goose embeds cleanly, freshest cadence. | goose v3.30.0 2026-11-10 (min Go 1.26). golang-migrate v4.20.1 2026-09-09. tern v2.4.3 2026-08-23. dbmate v2.38.1 2026-11-09. Atlas OSS v1.3.0 2026-08-02. River v0.51.2 2026-11-09. |
| MongoDB | mongo-go-driver v2 (go.mongodb.org/mongo-driver/v2) | none (v1 EOL 2026-01-15) | v2 API deltas: mongo.Connect(opts) no ctx; options.Lister[T]; ClientOptions.Timeout; mongo.SessionFromContext; bson package consolidation; nested docs decode to bson.D. | v2.10.0 2026-11-07; pin >= 2.9.3 (security fixes in 2.8.2/2.9.1). |
| HTTP routing | net/http ServeMux (Go 1.22 method + {id}/{path...}/{$} patterns) with small internal middleware chain | chi v5 (groups, Mount, scoped middleware, same http.Handler); Echo v5; Gin; Fiber v3 (opt-in only, fasthttp) | Stdlib sufficient for most services; chi zero-cost upgrade. JetBrains 2026 survey: net/http 46% (up from 32%), Gin 48%, chi 15%, Echo 13%, Fiber 13%. Echo v5 changed handler signature. | chi v5.5.0 2026-10-14. Echo v5.4.4 2026-10-30. Gin v1.12.0 2026-02-28. Fiber v3.7.0 2026-11-09. |
| OpenAPI | Code-first: huma v2 on humago (ServeMux) or humachi | Spec-first: oapi-codegen (std-http-server + strict-server); ogen; fuego (pre-1.0); swag (v2 still RC) | Generator owns handler layer and DTOs, so code-first keeps spec, validation, RFC 9457 errors, docs UI in sync. Offer oapi-codegen for contract-first teams. huma bus factor: one maintainer. | huma v2.40.0 2026-10-27. oapi-codegen v2.8.2 2026-10-31. ogen v1.24.0 2026-08-07. fuego v0.21.0 2026-10-09. swag v2.0.0-rc6. |
| gRPC | buf CLI + protoc-gen-go + connect-go v2 (connectrpc.com/connect/v2; gRPC/gRPC-Web/Connect over net/http) | grpc-go + protoc-gen-go-grpc; connect-go v1 | Connect handlers mount on the same ServeMux. buf.gen.yaml v2 with remote plugins avoids local protoc. | buf v1.75.0 2026-10-22. connect-go v2.0.2 2026-10-21 (v2 launched 2026-10-07). protobuf-go v1.37 min Go 1.26. grpc-go v1.85.0 2026-10-27. |
| Validation | go-playground/validator v10 (skip when huma selected) | ozzo-validation (revived Jul 2026); govalid (codegen) | De-facto standard, struct tags match DTOs. | validator v10.30.6 2026-11-02. ozzo v4.5.0 2026-10-27. govalid v1.9.0. |
| Config | caarlos0/env v11 (env to struct, 12-factor) | koanf v2 (file + env layering); sethvargo/go-envconfig v2. NOT Viper (stalled 13 months), kelseyhightower/envconfig (2019), cleanenv (2023). | One tiny dep; container-first. | env v11.4.2 2026-11-03. koanf v2.4.1 2026-10-18. go-envconfig v2.0.0 2026-09-23. Viper v1.21.0 2025-09-08. |
| Logging | log/slog (JSON handler; 1.26 NewMultiHandler) | zerolog or zap as slog.Handler backends | Zero deps; slog is the default frontend in all 2026 guides. | zerolog v1.35.1 2026-04-20. zap v1.28.0 2026-04-28. |
| DI | Manual constructor injection (composition root) | goforj/wire (fork of google/wire); uber fx | google/wire ARCHIVED 2025-08-25. Manual wiring is idiomatic, zero-dep, deterministic for a generator. | google/wire v0.7.0 2025-08-22 archived. goforj/wire v1.3.1 2026-11-02 (347 stars). fx v1.24.0 2025-05-13. |
| Mocking | mockery v3 (config-driven; moq-style template for zero-runtime-dep mocks) | uber-go/mock; moq; counterfeiter | Ports are interfaces; mockery v3 freshest. | mockery v3.9.1 2026-11-09. uber mock v0.6.0 2025-08-18. moq v0.8.1 2026-10-14. counterfeiter v6.14.0. |
| Testing | stdlib testing + testify v1; testcontainers-go for opt-in DB integration | matryer/is (dormant); gotest.tools | testify stays v1. | testify v1.12.1 2026-08-19. testcontainers-go v0.45.0 2026-11-05. |
| Lint | golangci-lint v2 (version: "2" config; binary install in CI) | none | | v2.15.0 2026-10-16. |
| Task runner | Makefile | Taskfile; mage (recently revived) | | Task v3.54.0 2026-10-01. mage v1.17.2 2026-04-23. |
| Hot reload | air (.air.toml) | wgo | | air v1.67.4 2026-08-01. wgo v0.7.2 2026-10-09. |

## Go 1.22–1.26 features to exploit (and 1.27 notes)

- 1.22: ServeMux method+wildcard patterns, r.PathValue, {$}; per-iteration loop vars; range N; math/rand/v2; database/sql.Null[T]; cmp.Or; slices.Concat.
- 1.23: range-over-func + iter.Seq/Seq2; slices.All/Values/Collect/Sorted, maps.Keys/Values/Collect; http.Request.Pattern; unique; go.mod godebug; go mod tidy -diff (CI check).
- 1.24: tool directive + go tool / go get -tool; generic type aliases; os.Root; testing.B.Loop, t.Context(), t.Chdir; omitzero JSON tag; strings.Lines/SplitSeq; vet tests/printf.
- 1.25: testing/synctest GA; sync.WaitGroup.Go; http.CrossOriginProtection; slog.GroupAttrs; container-aware GOMAXPROCS; go.mod ignore directive; go no longer auto-adds toolchain line; vet hostport/waitgroup.
- 1.26: errors.AsType[T]; slog.NewMultiHandler; signal.NotifyContext cancel cause names the signal; httptest.Server.Client resolves example.com; ServeMux trailing-slash redirect now 307; new(expr); go fix modernizers; go mod init writes go 1.(N-1).0; testing.T.ArtifactDir.
- Older but required: errors.Join (1.20), context.WithoutCancel/WithCancelCause/AfterFunc, slices/maps, min/max/clear (1.21).
- 1.27 (Aug 2026): generic methods; encoding/json backed by json/v2 (rejects duplicate keys/invalid UTF-8 by default; GOEXPERIMENT=nojsonv2 opt-out); stdlib uuid package; httptest.NewTestServer for synctest; go test runs stdversion vet; go mod tidy consolidates require blocks.

Minimum go directive strategy: emit `go 1.26.0`, no toolchain line. goose, huma, validator, gin, grpc-go, connect-go v2, oapi-codegen runtime, caarlos0/env, testcontainers-go and Ent already declare go 1.26.0. Expect to bump every ~6 months. Use `tool` directives only for small generators; keep heavy CLIs (sqlc, golangci-lint, mockery, buf) out of the main module via pinned binaries in Makefile or `go tool -modfile=tools.mod`.

## Risks / things changing fast
1. Go 1.27 json/v2 switch changes generated-API behavior on bump; add a test.
2. Rapid min-Go bumps.
3. DI: google/wire archived; forks small. Manual DI safe.
4. sqlc cadence: last release Apr 2026, 594 open issues; commits continue.
5. Ent/Atlas single vendor; Atlas paywalled migrate lint.
6. connect-go v2 is one month old.
7. Echo v5 breaking; Fiber v3 non-net/http.
8. Stalled: Viper, sqlx, kelseyhightower/envconfig, cleanenv, matryer/is; swag v2 RC; ozzo revived; mage revived.
9. huma bus factor; oapi-codegen volunteer sustainability.
10. mongo-go-driver two 2026 security fixes; pin >= 2.9.3.

Could not verify: JetBrains State of Go 2026 article body (403), zerolog first-party slog adapter, Go 1.27 exact release date, issue-count trends.

## Sources
go.dev/doc/go1.22..go1.27; GitHub releases pages for pgx, sqlc, gorm, ent, bun, go-jet, sqlx, goose, golang-migrate, dbmate, tern, atlas, river, mongo-go-driver (+ docs/migration-2.0.md), chi, echo (API_CHANGES_V5.md), gin, fiber, huma, oapi-codegen, ogen, fuego, swag, buf (blog connect-go-v2), connect-go, protobuf.dev news 2026-10-16, grpc-go, validator, ozzo, govalid, caarlos0/env, koanf, go-envconfig, viper, zerolog, zap, google/wire (archived), goforj/wire, fx, mockery, uber mock, moq, counterfeiter, testify, testcontainers-go, gotest.tools, is, golangci-lint docs, task, mage, air, wgo; blog.jetbrains.com/go/2026/11/10/state-of-golang-2026; encore.dev best-go-backend-frameworks; dev.to atlas-paywalled article; raw go.mod files fetched 2026-11-10.
