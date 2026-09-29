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

- \`layered-rest\` (default): persistence, service, HTTP handler, and route packages.
- \`feature-first\`: a self-contained feature package.

Supported HTTP routers:

- \`net/http\` (default)
- \`chi\`
- \`gin\`

The selected router is persisted in \`.gox.yaml\`; generated resources use its
routing conventions automatically.

## Infrastructure

Select PostgreSQL, MongoDB, Redis, or Kafka during \`gox new\`, or add one to
an existing project:

\`\`\`sh
cd members
gox add postgres
gox add mongo
gox add redis
gox add kafka
\`\`\`

Each selection generates a concrete client adapter in
\`internal/infrastructure/\`, records the capability in \`.gox.yaml\`, and
adds the required Go module. Adapters expose explicit constructors and health
checks; the application does not connect to infrastructure until your
composition root calls them.

## Generate A Resource

\`\`\`sh
gox g resource account
gox g module survey --dry-run
gox remove resource account
\`\`\`

A generated resource includes CRUD routes, request validation, an in-memory
repository, handler tests, contracts, and automatic registration in the
application composition root.

## Layered REST Output

\`\`\`text
initiator/
  config.go
  persistence.go
  module.go
  handler.go
  routes.go
internal/
  storage/persistence/<resource>/
  module/<resource>/
  handler/rest/http/<resource>/
  glue/routing/<resource>/
  constant/model/
  infrastructure/
\`\`\`

The dependency direction is:

\`\`\`text
Persistence -> Module -> Handler -> Route
\`\`\`

## Runtime Behavior

Generated services read \`PORT\` (default \`8080\`), expose \`/healthz\` and
\`/readyz\`, and shut down gracefully on \`SIGINT\` or \`SIGTERM\`. The
\`net/http\` and Chi variants include request logging; Gin uses its standard
logger and recovery middleware.

## Extending Layouts

Add templates under \`initiator/templates/<layout>/\`, then define generated
files and composition-root wiring in \`internal/generator/layout.go\`. Every
layout must keep the four registry markers in \`initiator\` so resource
generation can remain deterministic.


what is the use of infratructure folder creation they are db or redis or storage instance creation why dont add them in the initiator like these handle 
/home/esubalew-sintie/comp/cbe_super_app2/cbe-super-app-member-users/initiator

and on the constant folder make like these and use the success and error response 
internal/
└── constant/
    ├── constants.go
    │
    ├── errors/
    │   └── errors.go
    │
    ├── entities/
    │   ├── <entity>/
    │   │   └── <entity>.go
    │   │
    │   └── <another-entity>/
    │       └── <another-entity>.go
    │
    ├── dto/
    │   ├── <entity>.go
    │   ├── <entity>_<operation>.go
    │   └── ...
    │
    └── response/
        ├── model.go
        └── response.go