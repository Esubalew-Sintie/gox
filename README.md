# gox

gox creates Go HTTP services with explicit composition-root wiring inspired by
the member-users project structure.

## Create A Project

Run the command without a project name or module path to choose the HTTP router,
persistence driver, optional infrastructure, and Git setup in the terminal:

\`\`\`sh
gox new
\`\`\`

It also supports unattended use:

\`\`\`sh
gox new members --module example.com/members --layout layered-rest --http chi --db postgres --with redis,kafka
gox new members --module example.com/members --layout feature-first --http gin --db mongo
\`\`\`

Supported layouts:

- \`layered-rest\` (default): central persistence, module, and HTTP-handler
  contracts plus per-resource implementations.
- \`feature-first\`: a self-contained feature package.

Supported HTTP routers: \`net/http\` (default), \`chi\`, and \`gin\`.

## Database Persistence

Choose one primary persistence driver with \`--db\`:

- \`memory\` (default): an in-process repository for development and tests.
- \`postgres\`: repositories use \`pgxpool\` and perform SQL CRUD.
- \`mongo\`: repositories use MongoDB collections and perform document CRUD.

The choice is stored as \`storage.driver\` in \`.gox.yaml\`. New resources keep
the same database-independent persistence interface; only their repository
implementation changes. Generated \`initiator/persistence.go\` selects and wires
the corresponding constructor.

Database lifecycle is owned by \`initiator/dependencies.go\`. It opens, pings,
exposes, and closes the selected database connection. Generated \`.env.example\`
contains the required settings:

- PostgreSQL: \`DATABASE_URL\`
- MongoDB: \`MONGO_URI\`, optionally \`MONGO_DATABASE\`

Only the selected database driver's Go dependency is included in \`go.mod\`.
Switching a database after resources are generated is intentionally not
automatic, because it changes real storage semantics and needs a migration.

## Generate A Resource

\`\`\`sh
gox g resource account
gox g module survey --dry-run
gox remove resource account
\`\`\`

Layered REST resource generation appends contracts to:

\`\`\`text
internal/storage/persistence/persistence.go
internal/module/module.go
internal/handler/rest/handler.go
\`\`\`

The initiator fields use those contracts, while generated concrete types remain
inside each feature package. This makes replacement and mocking straightforward.

## Generated Constants

\`\`\`text
internal/constant/
  constants.go
  errors/errors.go
  entities/<entity>/<entity>.go
  dto/<entity>.go
  response/model.go
  response/response.go
\`\`\`

Handlers return the shared \`response.Model\` envelope for successful and error
responses. The error package centralizes HTTP status and error-type metadata.

## Runtime Behavior

Generated services read \`PORT\` (default \`8080\`), expose \`/healthz\` and
\`/readyz\`, and shut down gracefully on \`SIGINT\` or \`SIGTERM\`. The
\`net/http\` and Chi variants include request logging; Gin uses its standard
logger and recovery middleware.

## Extending Layouts

Add templates under \`initiator/templates/<layout>/\`, then define generated
files and composition-root wiring in \`internal/generator/layout.go\`. Every
layout must keep its registry markers so resource generation remains
deterministic.
