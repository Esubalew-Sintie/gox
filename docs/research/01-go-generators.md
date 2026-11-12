# Go scaffolding/generator survey — lessons for gox (as of 2026-11-10)

## Key lessons for gox
1. The field splits into "create-only skeletons" and "framework-bound resource generators"; nobody mainstream does framework-neutral `add module + wire it`. go-blueprint (9.0k stars), gowebly, gopamin, gonew only create. Buffalo, goctl, gf, Nunu, Goca, scaffold-go, HexaGo add to existing projects but each is tied to one framework/ORM. That gap is gox's positioning.
2. The most robust "modify existing project" model is ownership partition, not patching: tool-owned files fully regenerated every run, user-owned files created once and never touched (goctl internal/logic; Goa gen/ vs goa example; sqlc, ent, oapi-codegen, ogen; gf gen ctrl module file "generated only once"). Make gox's composition root a fully generated registry.
3. Make ownership explicit in the file header. gf decides overwrite-vs-skip by the header `// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.`; delete it and the file becomes yours.
4. google/wire is archived (2025-08-25). Kratos and Nunu are built on a dead generator. gox must own its constructor-graph codegen.
5. init() self-registration (cobra-cli, gf gen service) avoids editing central files but costs side-effect imports, runtime panic "forgot register?", and loses compiler import-cycle detection. Prefer generated explicit wiring.
6. Patching hand-written files is where tools break. Buffalo locates block via go/ast then inserts by line with no duplicate check; destroy removes route with strings.Replace. Goca integrate only works if main.go contains mux.NewRouter() and *gorm.DB. Never make wiring depend on recognising user-edited code.
7. Persist the choices. go-blueprint forgets framework/driver after create, so it cannot add. gf reads hack/config.yaml (CLI flags override), goctl has --style. gox needs a manifest every later command reads.
8. Safety UX worth copying: --dry-run (Buffalo, Goca), conflict detection + --backup + --force (Goca), confirmation on destroy (Buffalo), --clear to delete orphaned generated files (gf), O_EXCL never-overwrite (Nunu).
9. Template override tiers: goctl template init writes built-ins to ~/.goctl and --remote <git> --branch overrides; HexaGo loads project-local > user-global > embedded; Nunu -t; oapi-codegen user-templates ("unstable"). Ship embedded templates, `gox template init`, project-local override dir; version-stamp templates.
10. Keep generated code per-module/package. ent #3814/#3865: ~300 entities in one package exhaust compiler memory. Commit generated code (sqlc/ent/goctl norm).
11. Infra by convention, not runtime: Encore auto-provisions a Docker DB and runs migrations. gox gets 80% by generating docker-compose, migration dirs and a Makefile per selected DB with zero runtime dependency.
12. AI-era extras now expected: kratos-layout ships AGENTS.md + CLAUDE.md; gopamin emits AGENTS.md; HexaGo and Encore expose MCP servers; gopamin ships .go-arch-lint.yml + make arch-check. Generate AGENTS.md and an architecture-lint config per layout.

## Per-tool findings
- go-blueprint (Melkeydev): 8,983 stars, pushed 2026-11-08, v0.10.11 (2025-07). One skeleton: cmd/api, internal/server/{server,routes}.go, internal/database, Makefile, Air, Docker. Choose framework (chi/gin/fiber/httprouter/gorilla/echo), driver (mysql/postgres/sqlite/mongo/redis/scylla), --feature htmx|tailwind|react|websocket|docker|githubaction. NewServer() calls database.New() inside constructor (no injection). Create-only. Top issues: feature requests (logging, migrations+cobra).
- Buffalo: core 8,424 stars, v1.1.4 2026-03-20; CLI repo archived 2024-03-27. Rails-style generate resource|action|task|mailer, destroy, Pop migrations. Resource generator inserts app.Resource(...) via gogen.AddInsideBlock(f, "appOnce.Do(func() {", stmt); no dedupe. --dry-run. destroy strips route via strings.Replace.
- Kratos CLI: 26,021 stars, v3.0.0 2026-06-26. kratos new clones git layout; proto add|client|server; layout api/ cmd/server internal/{biz,data,service,server,conf}, wire ProviderSet per layer; ships AGENTS.md/CLAUDE.md. Adding a service: CLI writes only the service file; user edits http.go/grpc.go + ProviderSet then reruns wire. Built on archived wire.
- goctl (go-zero): 33,423 stars, v1.10.3 2026-08-01. api new|go (from .api DSL), rpc protoc, model mysql|mongo|pg, docker, kube, template init|clean|update|revert, --style, --home, --remote. Layout etc/ internal/{config,handler,logic,svc,types}; wiring = hand-edited ServiceContext. "never overwrites files in internal/logic"; re-running safe. Issues: more codegen features, handler-level middleware, "not working after upgrade".
- Encore: 12,702 stars, v1.58.8. Not a scaffolder: app create only. Services are packages with //encore:api; parser builds app model, generates main and gitignored encore.gen.go, provisions DBs on encore run. Complaints: "encore apps that happen to be in go", prevents following Go conventions; self-hosting friction.
- Nunu: 2,614 stars, v1.1.6 2026-08-12. new -r templates, create handler|service|repository|model|all, run, wire. Gin+GORM+wire. create renders templates to internal/<type>/, O_EXCL, does not touch wire.go/router, prints no hint.
- gonew (x/tools): copies template module, rewrites module path, dir must be empty; "highly experimental". Template-distribution idea only.
- GoFrame gf: 13,303 stars, v2.10.4. init, gen dao|do|entity, gen ctrl (one controller file per op; --merge, --clear, --watchFile), gen service (scans internal/logic, emits interfaces + registration via init and blank imports; panics "forgot register?"), up, docker, build. Config hack/config.yaml. Overwrite governed by header comment.
- Beego bee: 1,469 stars, v2.3.0 2024-11; effectively stale, GOPATH-era issues.
- cobra-cli: 940 stars, v1.3.0 2022-03. add -p parent; new file self-registers via init(); root.go never edited.
- Goa: 6,123 stars, v3.35.0 2026-10-22 (Go 1.27.3+). DSL -> goa gen replaces gen/ tree; goa example creates missing app files, leaves existing alone. Steep DSL curve; praised for REST+gRPC+OpenAPI from one design.
- ent: 17,227 stars, v0.14.6. schema user-owned, rest regenerated; --template, --feature, entc.go hooks. Compile memory blowups with ~300 entities; "code bloat" critiques.
- sqlc: 18,446 stars, v1.31.1. sqlc.yaml v2; outputs db.go models.go querier.go *.sql.go; vet, verify; generated code has no sqlc runtime. Complaints: dynamic queries (#3414 open). Praise: "sqlc is goated".
- oapi-codegen: 8,655 stars, v2.8.2 (needs Go 1.26). Models, ServerInterface, strict server, client; routers chi/echo/fiber/gin/gorilla/iris/net-http; user-templates "unstable"; "no defined release cadence".
- ogen: 2,158 stars, v1.24.0. Own radix router, no reflection, Opt[T], sum types, validation, OTel.
- google/wire: 14,377 stars, archived 2025-08-25; forks exist. uber fx out of scope.
- Recent entrants: Goca (sazardev, 306 stars): init, feature (domain/usecase/repository/handler + auto-integrate), di, integrate (anchor-string patching of main.go, GORM+mux only), --database postgres|mysql|mongodb, --dry-run --backup --force. HexaGo (31 stars): init, add service|domain|adapter|worker|migration|tool, validate, 3-tier templates, MCP server. scaffold-go (bancolombia): fx+Viper, Fiber, patches cmd/main.go between `// scaffold-go:imports:begin/end` markers (closest analogue to gox today). gopamin: create-only, 6 frameworks, 8 DBs, Kafka/RabbitMQ/Redis, OTel/JWT/Swagger, .go-arch-lint.yml, AGENTS.md. gowebly (1,235 stars): create-only htmx/templ.

## Features gox should copy, and from whom
- Ownership split + "create missing only" + DO-NOT-EDIT header as overwrite switch: goctl, Goa, gf.
- Generated registry/composition root regenerated whole (wire's output shape, without wire): Kratos/Nunu wire_gen.go model minus the archived tool.
- --dry-run, conflict detection, --backup/--force, destroy with confirmation, --clear for orphans: Buffalo, Goca, gf.
- Flags mirroring every prompt + persisted config with CLI precedence + web command builder: go-blueprint, gf hack/config.yaml.
- Template tiers and template init/--remote: goctl, HexaGo.
- One file per handler operation; --style naming: gf, goctl.
- Migrations dir convention + auto docker-compose per DB: Encore (convention only).
- AGENTS.md / arch-lint config emitted with project: kratos-layout, gopamin, HexaGo.
- Plugins/hooks for extra artifacts: ent entc.go, Goa plugins, sqlc WASM plugins.

## Anti-patterns to avoid
- Creating files without wiring and saying nothing: Nunu create, Kratos proto server.
- Wiring that depends on recognising hand-written code: Goca integrate, Buffalo block search, destroy via strings.Replace.
- Runtime registries via init() with panic-on-missing: gf gen service, cobra-cli.
- Building on an unmaintained dependency: Kratos/Nunu on wire.
- Forgetting the user's choices after create: go-blueprint, gowebly, gopamin.
- Framework lock-in / non-Go conventions: Encore, Buffalo.
- One giant generated package: ent.
- Hard-coding DB creation inside constructors: go-blueprint NewServer().
- Stale CLI with GOPATH-era issues: bee, cobra-cli.

## Sources
go-blueprint repo/docs; gobuffalo/buffalo, cli, genny gogen; go-kratos/kratos-layout, docs, v3.0.0 release; go-zero.dev goctl FAQ/cli-guide/template; encore.dev docs (service-structs, databases, cli-reference, migrate-away), dev.to encore thread; go-nunu/nunu + create.go; x/tools gonew, go.dev/blog/gonew; goframe.org cli docs (init, gen-ctrl, gen-service) + template source; beego/bee; spf13/cobra-cli; goadesign/goa; entgo.io code-gen, ent #3814; sqlc config reference, #3414; oapi-codegen; ogen; google/wire; sazardev/goca (+ cmd/integrate.go); padiazg/hexago; bancolombia/scaffold-go; bezmoradi/gopamin; gowebly; awesome-go project-layout. Issue lists fetched 2026-11-10.
