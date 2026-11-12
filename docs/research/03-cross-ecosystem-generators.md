# Generator-CLI research for gox (cross-ecosystem track)

## Lessons for a Go generator

1. Vocabulary: `new` / `gen <kind> <Name> [field:type…]` / `add` / `remove` / `destroy` / `doctor` / `eject`, with one-letter aliases (`g`, `d`) and a Rails/Phoenix-style transcript (`create`, `inject`, `identical`, `skip`, `conflict`, `manual`). Every surveyed tool converged on this; the transcript is the most-loved part of Rails' UX.
2. Positional data shape on the command line, prompts only for what's missing, and a flag for every prompt. Phoenix/Rails take `title:string body:text`; Nest asks exactly two questions for `g resource` (transport, CRUD) but exposes `--type` and `--crud`; Laravel's `make:model` multiselect maps 1:1 to `-mfsc`. Non-interactive mode (`--defaults`/`--no-interactive`) must always work.
3. Ask the boundary question, decide the layout. Phoenix never asks where files go, but when generating into an existing context it prints the function count and "prefer creating a new context over adding to the existing one. Would you like to proceed?". gox should ask feature/bounded-context and transport, never folders.
4. Project-level generator defaults with precedence CLI > project > global. `nest-cli.json generateOptions`, `angular.json schematics`, Rails `config.generators`. gox's project file should record arch, DB, transports and infra so `gen` never re-asks.
5. Edit existing Go files only with `go/ast`/`dave/dst` + `gofmt`, never regex; make every injection idempotent and fallible. Nest's "AST" editor only locates nodes then splices text by offset; dedupes by exact text; silently no-ops if `providers:` isn't an array literal; inserts imports with a regex. Rails' `route` uses a regex sentinel and Thor's loose `replacement_present?` (a commented-out route counts), which produced syntax errors on destroy. phx.gen.auth's injector is the model: `{:ok, code} | :already_injected | {:error, :unable_to_inject}` and, on error, prints the exact snippet and path to paste.
6. Minimise the number of hand-edited files that need touching. Laravel and .NET need no injection at all (auto-discovery/assembly scanning); AdonisJS confines edits to a few generator-owned files. gox should generate a per-feature `register.go` and one generator-owned aggregate file (regenerated from a manifest), so wiring never depends on parsing user code.
7. Plan in a virtual tree, then commit; dry-run becomes free. Angular's `Tree` (base + staging; Create/Rename/Overwrite/Delete actions; dry-run sink) is the reference. Phoenix batches conflicts up front instead of per-file surprises.
8. Destroy must be manifest-driven, not revoke-by-regex. Rails `destroy` re-runs the generator in `:revoke` mode and Thor removes inserted text with a greedy regex; Buffalo's `destroy resource` asks per file (`-y` to skip). Record what each `gen` created/injected (with hashes) and refuse to delete modified files without `--force`.
9. Require a clean git tree (or `--allow-dirty`) for anything that edits existing files, as `ng update` does; optionally `--create-commits`.
10. Per-file template override with fallback, plus a drift diff. Laravel `stub:publish [--only]`; Adonis `eject`; Phoenix `priv/templates/phx.gen.*`; Rails `lib/templates/<gen>/<file>.tt`. A first-class `templates diff` is valuable.
11. Compose big generators from small ones that work alone. Rails' model → resource → scaffold ladder; Laravel `make:model --all`; jasontaylordev's `ca-usecase` single-use-case item template. `gox gen feature` should orchestrate `gen entity|usecase|repository|handler`.
12. Generated code is the user's code; print tailored next steps. phx.gen.auth: "the user has complete freedom to modify". Spring's generated `HELP.md` is tailored to chosen dependencies; Phoenix prints "Remember to run migrations".
13. `add <infra>` = install + configure. AdonisJS `node ace add` is `npm install` + `configure` (codemods for rc file, middleware, env validation, `.env`/`.env.example`). Spring CLI `boot add` admits "the heuristic is not 100% complete". Keep merges confined to generator-owned files.
14. Minimal by default; heavy patterns opt-in. Django keeps `startapp` tiny; Rails core discussed removing scaffold; jasontaylordev's template drew "overkill" criticism and a licensing scramble when MediatR went commercial (domain coupled to `INotification`). gox's domain layer should be stdlib-only.
15. Validate input loudly; expose resolved names. Nest silently dropped trailing args (issue #1520). Adonis exposes naming conventions to templates; print resolved names in dry-run.

## Per-ecosystem findings

NestJS: `nest new|g <schematic>|add|build|info`; `--dry-run`, `--flat`, `--no-spec`, `--skip-import`, `--type rest|graphql...|microservice|ws`, `--crud`. Defaults in `nest-cli.json generateOptions`. ModuleFinder walks up to nearest `*.module.ts`; MetadataManager splices text at AST offsets, dedupes by text; import inserted by regex. `--dry-run` only; no destroy. Praised: fastest CRUD bootstrap. Criticized: service bodies are placeholders; args silently ignored.

Phoenix: `mix phx.gen.context|schema|html|json|live|auth` with `Context Schema table field:type`; context = boundary. Injects into context/test/fixtures by trimming trailing `end` and appending (skipped if already contained); router NOT edited, prints the line to add. phx.gen.auth injects via regex anchors with `:already_injected`/`:unable_to_inject` and printed fallbacks. Conflicts batched. No destroy/dry-run. Override via `priv/templates/phx.gen.*`. "The generators are first and foremost learning tools."

Rails: `rails g model|controller|migration|resource|scaffold|generator`, `rails d`; `-p/--pretend`, `-f/--force`, `-s/--skip`. Hooks and `config.generators`. Thor `inject_into_file after:/before:` sentinels; `route` regex-matches `.routes.draw do`. `destroy` via `:revoke` regex-based, known breakages. Override `lib/templates/<gen>/<file>.tt`.

Laravel: `make:model Flight -mfsc|--all`, `make:controller --resource --api --model --requests`. Prompts for missing input. No existing-file edits (auto-discovery). No destroy; `--force`. `stub:publish [--only] [--force]` → `stubs/`.

Angular: `ng new|g|add|update`; `--dry-run`, `--force`, `--defaults`; `schema.json x-prompt`; `angular.json schematics` defaults. Tree + sinks; `ng add` = install + `ng-add` schematic; `ng update` runs versioned migrations, `--allow-dirty` false, `--create-commits`. Criticized: authoring schematics is painful.

.NET: jasontaylordev `dotnet new ca-sln --database postgresql|sqlite|sqlserver`; `dotnet new ca-usecase -n X -fn Feature -ut command|query` (no wiring needed thanks to assembly scanning). Layers Domain/Application/Infrastructure/Web. ardalis `clean-arch` (Core/UseCases/Infrastructure/Web) and `min-clean` (single project by feature). Criticism: overkill for small apps; MediatR went commercial.

Django: `startapp name --template path|URL`; INSTALLED_APPS edited by hand. Core prefers minimal defaults. No dry-run/undo.

Spring Initializr: one-shot, metadata-driven catalog, tailored `HELP.md`. Spring CLI `boot add` merges pom/config/code, "expect a few bumps".

AdonisJS: `make:*`, `add` = install + `configure`; `eject`; codemods via ts-morph (`updateRcFile`, `registerMiddleware`, `defineEnvValidations`...). Stubs with `exports()` destination; project `stubs/` wins.

Buffalo (Go): `buffalo generate resource widget title description:nulls.Text` writes model/migration/actions/templates/tests and `app.Resource(...)` in `actions/app.go`; `buffalo destroy resource` prompts per file.

## Recommended command set and UX for gox

- `gox new <name> [--arch hex|clean] [--db postgres|mysql|sqlite|mongo] [--transport http,grpc] [--with redis,kafka,otel] [--module path] [--defaults]`; writes `gox.yaml`; prints tailored NEXT_STEPS.
- `gox gen feature <Name> [field:type …] [--crud] [--transport http,grpc] [--no-repo] [--no-handler]` (alias `g f`): entity, ports, use cases, repo adapter, handlers, migration, tests, per-feature `register.go`, aggregate wiring.
- `gox gen entity|usecase|repository|handler|middleware|migration|event <Feature>/<Name>`: atomic generators that `feature` composes.
- `gox add <infra> [--dev]` / `gox remove <infra>`: `go get` + config + `.env`/`.env.example` + compose service + DI wiring, confined to generator-owned files.
- `gox destroy feature <Name>` (alias `d`): manifest-driven; per-file confirm, `-y`; refuses modified files without `--force`.
- Global: `--dry-run/-n` (virtual tree + diff), `--force`, `--skip`, `--yes`, `--no-interactive`, `--allow-dirty`.
- `gox templates eject [path] [--only …]` / `gox templates diff` / `gox templates list`: per-file override into `.gox/templates/`.
- `gox doctor`: resolved config, overrides in effect, every feature registered, orphaned injections, build + vet.
- `gox list` / `gox info`.

## Sources
NestJS docs/cli/usages, recipes/crud-generator, nestjs/schematics src/utils; nest-cli issue #1520. Phoenix hexdocs contexts, phx.gen.json/auth, phoenix lib/mix/tasks, issue #3832, fly.io customizing-phoenix-generators. Rails guides generators.html, railties actions.rb, thor inject_into_file.rb. Laravel docs artisan, ModelMakeCommand.php, PR #49691. Angular schematics-authoring, cli/generate, cli/update. jasontaylordev/CleanArchitecture (+ ca-use-case template.json, discussions #1346/#1413); ardalis CleanArchitecture design-decisions. Django startapp docs, ticket #29540. Spring Initializr reference, Spring CLI boot-add guide. AdonisJS scaffolding docs. Buffalo resources docs, buffalo-cli destroy.
