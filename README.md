# gox

gox creates Go HTTP services with explicit composition-root wiring inspired by
the member-users project structure.

## Create A Project

Run the command without a project name or module path to choose the HTTP router,
storage providers, optional infrastructure, and Git setup in the terminal:

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

Supported HTTP routers: \`net/http\` (default), \`chi\`, and \`gin\`. The
selected router is stored in \`.gox.yaml\`, and generated resources use its
routing conventions automatically.

## Infrastructure Lifecycle

PostgreSQL, MongoDB, Redis, and Kafka are selected during \`gox new\` or added
later:

\`\`\`sh
cd members
gox add postgres
gox add mongo
gox add redis
gox add kafka
\`\`\`

The generator creates \`initiator/dependencies.go\`. It owns constructing,
exposing, checking, and gracefully closing the selected clients, so connection
lifecycle stays in the composition root instead of being hidden in feature
packages. Required environment variables are:

- PostgreSQL: \`DATABASE_URL\`
- MongoDB: \`MONGO_URI\`
- Redis: \`REDIS_ADDR\`, optionally \`REDIS_PASSWORD\`
- Kafka: \`KAFKA_BROKERS\` and \`KAFKA_TOPIC\`

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
