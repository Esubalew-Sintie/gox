# gox — Go Project Generator

**Product Requirements Document**

**Version:** 1.2
**Status:** Proposed
**Repository:** `github.com/esubalew-Sintie/gox`

---

## 1. Product Overview

`gox` is a CLI-based **Go project generator** designed to eliminate repetitive application scaffolding while preserving idiomatic Go architecture and explicit application code.

The primary goal is to make creating and extending a Go backend as convenient as using a framework CLI such as NestJS CLI, without turning Go into a runtime framework or hiding important application behavior behind reflection or magic.

`gox` generates ordinary Go source code.

The generated application should remain understandable, editable, testable, and maintainable without requiring `gox` at runtime.

### Core principle

> **Ask the developer about architectural choices; automatically handle implementation choices.**

The developer decides things that materially affect the architecture.

`gox` decides repetitive implementation details.

For example:

The developer chooses:

* HTTP framework
* database
* optional infrastructure
* Git initialization

`gox` automatically handles:

* repository creation
* service creation
* handler creation
* route creation
* dependency wiring
* imports
* route registration
* formatting
* validation
* build verification

The developer should not need to answer:

> "Do you want a repository?"

> "Do you want a service?"

> "Do you want a handler?"

Those are implementation details of a standard generated module.

---

# 2. Problem Statement

Building a Go backend repeatedly requires creating and wiring similar structures:

```text
handler
service
repository
model
DTO
routes
dependency injection
route registration
configuration
database integration
```

For every new project, developers repeatedly perform the same work.

For every new feature, developers repeat the same scaffolding.

This creates several problems:

1. Repetitive development work
2. Inconsistent project structures
3. Manual dependency wiring
4. Manual route registration
5. Boilerplate imports
6. Naming inconsistencies
7. Higher probability of compilation errors
8. Slow project initialization
9. Difficulty enforcing organizational architecture
10. Increased onboarding time

Existing Go tooling solves pieces of this problem, but `gox` aims to provide a cohesive developer workflow.

---

# 3. Product Goals

## 3.1 Primary Goals

### Goal 1 — Fast project creation

A developer should be able to create a functional Go backend with:

```bash
gox new payment-api
```

and have a runnable project generated.

---

### Goal 2 — Complete feature generation

A developer should be able to create a complete business module with:

```bash
gox g module payment
```

The command should generate the entire application stack required by the project's architecture.

---

### Goal 3 — Automatic wiring

Generated modules must automatically be connected to:

* dependency injection
* application composition
* routing
* configured infrastructure

The developer should not manually modify multiple files after generating a module.

---

### Goal 4 — Explicit infrastructure

Infrastructure should be opt-in.

For example:

```bash
gox add postgres
gox add redis
gox add kafka
```

A developer should not receive Kafka, Redis, or other infrastructure merely because they created a Go project.

---

### Goal 5 — Preserve idiomatic Go

Generated code must be normal Go.

`gox` must not require:

* runtime reflection
* runtime dependency injection
* generated runtime engines
* proprietary application servers
* hidden execution mechanisms

After generation, the application should behave like a normal Go application.

---

### Goal 6 — Generated projects compile

A successful generation operation should produce code that passes:

```bash
go build ./...
```

where practical.

---

# 4. Non-Goals

`gox` is **not** intended to become:

* a runtime framework
* an ORM
* a dependency-injection runtime
* a replacement for `go`
* a business logic framework
* a Kubernetes platform
* a deployment platform
* an application server
* a cloud platform
* an AI coding agent

`gox` generates code.

It does not own the runtime behavior of the generated application.

---

# 5. Target Users

Primary users:

* Go backend developers
* Backend teams
* Startups
* Engineering teams with standardized Go architecture
* Developers building REST APIs
* Developers building microservices
* Teams that repeatedly create similar Go services

Secondary users:

* Companies enforcing internal Go project conventions
* Developers migrating from framework-oriented CLI workflows
* Teams building multiple services with the same architecture

---

# 6. Product Philosophy

`gox` follows five principles.

## 6.1 Explicit over magical

Generated Go should make dependencies visible.

Example:

```go
repo := payment.NewRepository(p.DB)
service := payment.NewService(repo)
handler := payment.NewHandler(service)
```

This is preferred over:

```go
container.Resolve("payment.service")
```

---

## 6.2 Architecture decisions belong to the developer

The developer decides:

```text
HTTP framework
database
optional infrastructure
project module path
```

---

## 6.3 Implementation decisions belong to gox

`gox` decides:

```text
repository files
service files
handler files
route files
imports
dependency wiring
route registration
formatting
validation
```

---

## 6.4 Infrastructure is opt-in

Do not generate:

```text
Kafka
Redis
RabbitMQ
Elasticsearch
```

unless the developer explicitly enables them.

---

## 6.5 Generated code belongs to the developer

Once generated:

```text
gox
 ↓
ordinary Go source code
```

The developer owns and modifies the generated code normally.

---

# 7. CLI Design

The public CLI should remain small.

```text
gox new <project>
gox add <capability>
gox generate module <name>
gox version
gox doctor
```

Short alias:

```bash
gox g module <name>
```

---

# 8. Command: `gox new`

## 8.1 Purpose

Create a complete Go application skeleton.

Example:

```bash
gox new payment-api
```

---

## 8.2 Interactive Architecture Selection

`gox new` should ask the developer about decisions that materially affect the generated application.

Example:

```text
Create Go project: payment-api

? Go module path:
  ▸ github.com/esubalew-Sintie/payment-api

? HTTP framework:
  ▸ chi
    net/http
    gin

? Database:
  ▸ PostgreSQL
    MySQL
    None

? Redis:
  ▸ No
    Yes

? Kafka:
  ▸ No
    Yes

? Initialize Git?
  ▸ Yes
    No

Creating project...

✓ Project created
✓ PostgreSQL configured
✓ Git initialized
✓ Dependencies installed
✓ Project formatted
✓ Build successful

Project created successfully.
```

The exact questions may evolve.

The important requirement is:

> Only ask questions that materially affect project architecture.

---

# 9. Non-Interactive Project Creation

Interactive prompts should not prevent automation.

Example:

```bash
gox new payment-api \
  --module github.com/acme/payment-api \
  --http chi \
  --db postgres \
  --with redis,kafka \
  --git
```

This enables:

* CI/CD
* scripts
* reproducible project creation
* advanced users

---

# 10. Generated Project

A generated project may resemble:

```text
payment-api/
├── cmd/
│   └── server/
│       └── main.go
│
├── initiator/
│   ├── config.go
│   ├── database.go
│   ├── modules.go
│   ├── routes.go
│   └── initiator.go
│
├── internal/
│   ├── constant/
│   ├── handler/
│   ├── module/
│   ├── service/
│   ├── storage/
│   └── glue/
│
├── config/
├── migrations/
├── templates/
├── .gox.yaml
├── go.mod
└── go.sum
```

The exact layout is template-driven and may evolve.

---

# 11. `.gox.yaml`

`gox` maintains a project metadata file:

```yaml
version: 1

module: github.com/acme/payment-api

layout: clean-rest

storage:
  postgres: true

features:
  redis: false
  kafka: false
```

## 11.1 Purpose

`.gox.yaml` tells `gox` how the project was configured.

It is used by commands such as:

```bash
gox add redis
gox add kafka
gox g module payment
```

---

## 11.2 Developer Experience

The developer normally should **not manually maintain `.gox.yaml`**.

Instead:

```text
developer answers questions
        ↓
gox updates metadata
        ↓
gox generates code
```

For example:

```bash
gox add redis
```

automatically changes:

```yaml
features:
  redis: true
```

and updates the application code.

---

# 12. Command: `gox add`

## 12.1 Purpose

Add optional infrastructure or capabilities to an existing project.

Examples:

```bash
gox add postgres
gox add redis
gox add kafka
```

---

## 12.2 `gox add postgres`

The command should:

1. Locate project root
2. Load `.gox.yaml`
3. Validate the project
4. Check whether PostgreSQL already exists
5. Add required dependencies
6. Generate PostgreSQL integration
7. Update application configuration
8. Update dependency wiring
9. Update `.gox.yaml`
10. Format code
11. Validate/build

---

## 12.3 `gox add redis`

Example:

```bash
gox add redis
```

should automatically generate the required Redis infrastructure.

The developer should not need to manually create:

```text
redis client
redis configuration
redis initialization
dependency wiring
```

---

## 12.4 `gox add kafka`

Example:

```bash
gox add kafka
```

adds Kafka infrastructure.

It should not automatically attach Kafka to every module.

Kafka becomes an available application capability.

A module can use Kafka when explicitly configured to do so.

---

# 13. Command: `gox generate module`

## 13.1 Purpose

Generate a complete business/application module.

Example:

```bash
gox generate module payment
```

or:

```bash
gox g module payment
```

---

# 14. Module Generation Philosophy

A module is a complete feature.

The command should not require:

```bash
gox g repository payment
gox g service payment
gox g handler payment
gox g route payment
```

Instead:

```bash
gox g module payment
```

creates everything required.

---

# 15. Generated Module Architecture

Conceptually:

```text
HTTP Request
     │
     ▼
   Route
     │
     ▼
  Handler
     │
     ▼
  Service
     │
     ▼
 Repository
     │
     ▼
  Storage
```

---

# 16. Example Generated Structure

For:

```bash
gox g module payment
```

the project may contain:

```text
internal/
├── constant/
│   └── model/
│       ├── payment.go
│       └── dto/
│           └── payment.go
│
├── storage/
│   └── persistence/
│       └── payment/
│           └── payment.go
│
├── module/
│   └── payment/
│       └── payment.go
│
├── handler/
│   └── rest/
│       └── http/
│           └── payment/
│               └── payment.go
│
└── glue/
    └── routing/
        └── payment/
            └── payment.go
```

The exact structure should be controlled by templates.

---

# 17. Generated Repository

Example:

```go
type Repository struct {
    db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
    return &Repository{
        db: db,
    }
}
```

The repository should use whatever storage implementation the project configuration specifies.

---

# 18. Generated Service

Example:

```go
type Service struct {
    repo *Repository
}

func NewService(repo *Repository) *Service {
    return &Service{
        repo: repo,
    }
}
```

---

# 19. Generated Handler

Example:

```go
type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{
        service: service,
    }
}
```

---

# 20. Generated Module Wiring

Example:

```go
func New(p persistence.Persistence) Module {
    repo := payment.NewRepository(p.DB)
    service := payment.NewService(repo)
    handler := payment.NewHandler(service)

    return Module{
        Handler: handler,
    }
}
```

This keeps dependency injection explicit.

---

# 21. Generated Routes

Example:

```go
func Register(r chi.Router, handler *Handler) {
    r.Route("/payments", func(r chi.Router) {
        r.Post("/", handler.Create)
        r.Get("/", handler.List)
        r.Get("/{id}", handler.Get)
    })
}
```

---

# 22. Automatic Route Registration

`gox` should automatically register newly generated modules.

Example:

```go
func RegisterRoutes(r chi.Router, modules Modules) {
    payment.Register(r, modules.Payment.Handler)

    // gox:routes
}
```

When a new module is generated:

```bash
gox g module order
```

`gox` updates the composition code automatically.

The developer should not manually add:

```go
order.Register(...)
```

---

# 23. Automatic Dependency Registration

Example:

```go
type Modules struct {
    Payment payment.Module
    Order   order.Module

    // gox:fields
}
```

Initialization:

```go
func New(p persistence.Persistence) Modules {
    return Modules{
        Payment: payment.New(p),
        Order:   order.New(p),

        // gox:init
    }
}
```

Imports are also updated automatically.

---

# 24. Injection System

The injector is responsible for modifying existing Go source files.

Initial implementation may use deterministic markers.

Example:

```go
// gox:imports
```

```go
// gox:fields
```

```go
// gox:init
```

```go
// gox:routes
```

Example:

```go
import (
    "github.com/acme/payment-api/internal/module/payment"

    // gox:imports
)
```

---

# 25. Injector Requirements

The injector must be:

### Deterministic

The same input should produce the same output.

### Idempotent

Running:

```bash
gox g module payment
```

twice must not create duplicate imports or routes.

### Safe

Existing unrelated developer code must not be overwritten.

### Format-aware

Modified Go source must be passed through:

```go
go/format
```

### Atomic

File modifications should avoid leaving partially written files when an operation fails.

---

# 26. Future AST-Based Injection

The first implementation may use markers.

A future implementation should use:

```go
go/parser
go/ast
go/token
go/format
```

This allows structural Go source modification without relying heavily on markers.

The injector should remain an internal implementation detail.

---

# 27. Naming System

`gox` must have centralized naming logic.

Inputs:

```text
account
Account
ACCOUNT
payment-method
payment_method
user_profile
```

should produce predictable names.

Example:

```text
Input:  payment-method

Name:   payment-method
Pascal: PaymentMethod
Camel:  paymentMethod
Snake:  payment_method
Plural: payment_methods
```

All templates must consume the same naming model.

---

# 28. Generation Data

A centralized data structure should feed templates.

Example:

```go
type ModuleData struct {
    ProjectName string
    ModulePath  string

    Name   string
    Pascal string
    Camel  string
    Snake  string
    Plural string

    Storage StorageData
    Options ModuleOptions
}
```

Storage:

```go
type StorageData struct {
    Postgres bool
    Redis    bool
}
```

This prevents individual templates from implementing their own naming or configuration logic.

---

# 29. Template System

Templates should be embedded into the `gox` binary.

Example:

```go
//go:embed all:templates
var templates embed.FS
```

Templates should use:

```go
text/template
```

Recommended extension:

```text
.tmpl
```

Example:

```text
templates/
├── project/
├── module/
├── capability/
└── inject/
```

---

# 30. Internal Architecture

The `gox` source itself should follow:

```text
cmd
 ↓
initiator
 ↓
cli
 ↓
generator
 ├── renderer
 ├── injector
 ├── naming
 ├── config
 └── filesystem
```

---

# 31. `initiator`

`initiator` is the composition root for `gox` itself.

Example:

```go
func Start() {
    cmd.Execute()
}
```

This is separate from the generated application's own:

```text
initiator/
```

The distinction is:

```text
gox
 └── initiator
      └── initializes the generator CLI


generated-app
 └── initiator
      └── initializes the generated application
```

---

# 32. CLI Layer

The CLI layer should:

* parse arguments
* collect interactive answers
* validate user input
* invoke application services
* display progress/results

It should not contain template-generation logic.

Example:

```text
internal/cli/
├── new.go
├── generate.go
└── add.go
```

---

# 33. Generator Layer

The generator layer owns generation behavior.

Example:

```text
internal/generator/
├── project.go
├── module.go
└── capability.go
```

Responsibilities include:

* building generation plans
* rendering templates
* writing files
* injecting code
* validating generated projects

---

# 34. Filesystem Abstraction

Filesystem operations should be isolated.

Example:

```go
type FileSystem struct{}

func New() *FileSystem {
    return &FileSystem{}
}

func (fs *FileSystem) CreateDir(path string) error
func (fs *FileSystem) CreateFile(path string, content []byte) error
func (fs *FileSystem) Exists(path string) bool
```

This also makes filesystem behavior easier to test.

---

# 35. Configuration Package

The configuration package should:

* read `.gox.yaml`
* validate configuration
* write configuration
* provide project metadata to generators

Example:

```text
internal/config/
└── config.go
```

---

# 36. Project Detection

Commands such as:

```bash
gox add redis
gox g module payment
```

should automatically locate the project root.

Possible detection strategy:

```text
current directory
     ↓
search upward
     ↓
.gox.yaml
     ↓
project root
```

If no `.gox.yaml` exists, `gox` should provide a clear error.

---

# 37. Generation Pipeline

Every generation operation should follow:

```text
Command
  ↓
Validate
  ↓
Locate Project
  ↓
Load Config
  ↓
Build Generation Data
  ↓
Plan
  ↓
Preflight
  ↓
Dry Run?
  ├── Yes → Display Plan
  └── No
       ↓
     Apply
       ↓
     Inject
       ↓
     Format
       ↓
     Validate
       ↓
     Build
       ↓
     Summary
```

---

# 38. Dry Run

The generator should support:

```bash
gox g module payment --dry-run
```

Example output:

```text
Plan

CREATE
  internal/module/payment/payment.go
  internal/service/payment/payment.go
  internal/handler/payment/payment.go
  internal/storage/payment/payment.go
  internal/model/payment.go

MODIFY
  initiator/modules.go
  initiator/routes.go

No files were changed.
```

This allows developers to inspect changes before applying them.

---

# 39. Preflight Validation

Before modifying files, `gox` should verify:

* project exists
* `.gox.yaml` exists
* configuration is valid
* module name is valid
* target directories are writable
* target files do not unexpectedly exist
* required injection markers exist
* required infrastructure exists
* templates exist
* generated paths are safe

---

# 40. Rollback and Atomicity

Generation should minimize partial project states.

For operations modifying multiple files:

```text
Plan
 ↓
Validate everything
 ↓
Generate all changes
 ↓
Apply atomically where possible
 ↓
Format
 ↓
Build
```

If a critical operation fails, `gox` should avoid leaving half-generated modules.

---

# 41. Validation

After generation:

```bash
gofmt
go build ./...
```

should be executed as appropriate.

Future validation may include:

```bash
go vet ./...
```

and optionally project-configured checks.

---

# 42. Success Output

Successful commands should provide concise progress information.

Example:

```text
Generating module: payment

✓ Files generated
✓ Dependencies wired
✓ Routes registered
✓ Code formatted
✓ Build successful

Module "payment" created successfully.
```

Errors should clearly identify:

* what failed
* which file was involved
* what the developer can do next

---

# 43. Infrastructure Architecture

Capabilities are represented separately from business modules.

Example:

```text
Project
 ├── PostgreSQL
 ├── Redis
 ├── Kafka
 └── Business Modules
      ├── Payment
      ├── Order
      └── User
```

Infrastructure should not automatically become business logic.

---

# 44. Redis Design

`gox add redis` creates reusable Redis infrastructure.

For example:

```text
internal/
└── infrastructure/
    └── redis/
        ├── client.go
        └── config.go
```

Business modules can depend on the Redis abstraction when explicitly configured.

---

# 45. Kafka Design

`gox add kafka` creates reusable Kafka infrastructure.

For example:

```text
internal/
└── infrastructure/
    └── kafka/
        ├── client.go
        ├── producer.go
        └── consumer.go
```

However, producer/consumer commands should **not** be primary public CLI commands.

Avoid exposing:

```bash
gox g producer payment
gox g consumer payment
```

because these expose implementation details rather than developer intent.

Future module capabilities may allow:

```text
Event integration? Yes
```

and `gox` decides internally whether producer/consumer code is required.

---

# 46. Security Requirements

`gox` must validate user-provided names and paths.

It must prevent:

* path traversal
* arbitrary filesystem writes
* unsafe generated paths
* unintended overwrites
* arbitrary command execution through template input

`gox` must never execute generated application code.

---

# 47. Generated Code Security

Generated code should:

* use explicit dependencies
* avoid hidden global state where possible
* avoid unsafe defaults
* avoid embedding secrets
* use environment/configuration mechanisms for credentials

Example:

```text
DB_PASSWORD
REDIS_PASSWORD
KAFKA_PASSWORD
```

should not be written directly into generated source.

---

# 48. Testing Strategy

`gox` must be tested at multiple levels.

## 48.1 Naming Tests

Test:

```text
account
Account
ACCOUNT
payment-method
payment_method
user_profile
```

and verify generated naming.

---

## 48.2 Renderer Tests

Verify templates produce expected Go source.

---

## 48.3 Injector Tests

Test:

* imports
* fields
* initialization
* routes
* duplicate prevention
* malformed files
* missing markers
* atomic writes

---

## 48.4 Generator Tests

Test:

```text
new project
add capability
generate module
```

---

## 48.5 Golden Tests

Expected generated projects should be stored under:

```text
testdata/
└── golden/
```

Generated output is compared against known-good output.

---

# 49. End-to-End Test

A critical test should simulate:

```bash
gox new test-api
```

then:

```bash
cd test-api
```

then:

```bash
gox add postgres
```

then:

```bash
gox g module account
```

then:

```bash
gox g module payment
```

then:

```bash
go build ./...
```

and:

```bash
go vet ./...
```

The generated application must compile successfully.

---

# 50. CLI Error Handling

Examples:

```text
Error: project "payment-api" already exists.
```

```text
Error: no gox project found.
Run "gox new <project>" first.
```

```text
Error: module "payment" already exists.
```

```text
Error: PostgreSQL is not configured for this project.
Run "gox add postgres".
```

Errors should be actionable rather than exposing internal stack traces by default.

---

# 51. Command Separation

The distinction between commands must remain clear.

| Command               | Purpose                       |
| --------------------- | ----------------------------- |
| `gox new`             | Create application            |
| `gox add`             | Add infrastructure/capability |
| `gox generate module` | Create business feature       |
| `gox version`         | Show version                  |
| `gox doctor`          | Diagnose project/tooling      |

`gox g` is simply an alias for `gox generate`.

---

# 52. What Should Not Be Public CLI Concepts

The following are internal implementation concepts:

```text
repository
service
handler
route
DI
producer
consumer
middleware
```

They should not become the main user-facing mental model of `gox`.

Instead:

```text
Create application
Add capability
Create feature
```

The generator handles the technical implementation.

---

# 53. `gox doctor`

`gox doctor` should diagnose common project/tooling problems.

Example:

```bash
gox doctor
```

Possible output:

```text
gox doctor

✓ Go installed
✓ Go version supported
✓ Project detected
✓ .gox.yaml valid
✓ Dependencies available
✓ Generated structure valid
✓ Project builds

No problems found.
```

---

# 54. Extensibility

The architecture should allow future capabilities without redesigning the CLI.

Possible future capabilities:

```text
redis
kafka
rabbitmq
grpc
mongodb
mysql
postgres
object-storage
metrics
tracing
```

They should be implemented as capabilities rather than hardcoded into the entire generator.

---

# 55. Template Extensibility

Templates should be isolated from generator logic.

For example:

```text
templates/
├── project/
│   ├── go.mod.tmpl
│   ├── main.go.tmpl
│   └── initiator/
│
├── module/
│   ├── model.go.tmpl
│   ├── repository.go.tmpl
│   ├── service.go.tmpl
│   ├── handler.go.tmpl
│   └── route.go.tmpl
│
├── capability/
│   ├── postgres/
│   ├── redis/
│   └── kafka/
│
└── inject/
```

---

# 56. Versioning

`.gox.yaml` should contain a schema version:

```yaml
version: 1
```

This allows future versions of `gox` to migrate project metadata.

For example:

```text
.gox.yaml v1
      ↓
gox migration
      ↓
.gox.yaml v2
```

---

# 57. Backward Compatibility

Future versions should avoid breaking existing generated applications.

Because generated applications contain ordinary Go code, an application should continue working even if the developer stops using `gox`.

---

# 58. Distribution

Initial installation:

```bash
go install github.com/esubalew-Sintie/gox/cmd/gox@latest
```

Future distribution options:

* GitHub Releases
* GoReleaser
* Homebrew
* Scoop
* Linux packages

---

# 59. Performance Requirements

`gox` should:

* embed templates
* avoid unnecessary filesystem scanning
* modify only required files
* avoid unnecessary network calls
* avoid unnecessary builds
* generate files efficiently

For normal project/module generation, CLI responsiveness should feel immediate apart from dependency installation and build operations.

---

# 60. Initial Repository Structure

The `gox` repository should begin approximately as:

```text
gox/
├── cmd/
│   ├── main.go
│   └── root.go
│
├── initiator/
│   ├── config.go
│   ├── command.go
│   ├── generator.go
│   └── initiator.go
│
├── internal/
│   ├── cli/
│   │   ├── new.go
│   │   ├── generate.go
│   │   └── add.go
│   │
│   ├── config/
│   │   └── config.go
│   │
│   ├── naming/
│   │   └── naming.go
│   │
│   ├── renderer/
│   │   ├── renderer.go
│   │   └── template.go
│   │
│   ├── injector/
│   │   ├── injector.go
│   │   ├── marker.go
│   │   └── atomic.go
│   │
│   ├── filesystem/
│   │   └── filesystem.go
│   │
│   └── generator/
│       ├── project.go
│       ├── module.go
│       └── capability.go
│
├── templates/
│   ├── project/
│   ├── module/
│   ├── capability/
│   └── inject/
│
├── testdata/
│   └── golden/
│
├── .goreleaser.yaml
├── go.mod
└── go.sum
```

---

# 61. MVP Scope

The first production-oriented MVP should focus on:

### 1. Project creation

```bash
gox new <project>
```

with interactive architectural configuration.

### 2. Project metadata

Automatically create:

```text
.gox.yaml
```

### 3. PostgreSQL capability

```bash
gox add postgres
```

### 4. Complete module generation

```bash
gox g module <name>
```

### 5. Automatic wiring

Automatically modify:

* imports
* modules
* dependency initialization
* routes

### 6. Naming engine

Centralized naming transformations.

### 7. Template engine

Embedded Go templates.

### 8. Safe injector

Deterministic, idempotent, atomic.

### 9. Formatting and validation

Run:

```bash
gofmt
go build ./...
```

### 10. Testing

Unit, integration, golden, and end-to-end tests.

---

# 62. Post-MVP

After the MVP:

```text
Redis
Kafka
gRPC
MySQL
MongoDB
RabbitMQ
observability
authentication
scheduled jobs
event-driven module capabilities
```

can be added incrementally.

The architecture should allow these without changing the fundamental CLI model.

---

# 63. Future Module Interaction

A future version may allow module-level capability selection.

Example:

```bash
gox g module payment
```

could ask:

```text
Module: payment

Storage
  PostgreSQL ✓

Optional capabilities

? Redis integration:
  ▸ No
    Yes

? Event integration:
  ▸ No
    Yes

? Scheduled jobs:
  ▸ No
    Yes
```

This should be introduced only when the capability model is mature.

---

# 64. Future Configuration Model

The long-term model is:

```text
Developer
    │
    │ architectural decisions
    ▼
   gox
    │
    ├── .gox.yaml
    ├── templates
    ├── generator
    ├── injector
    └── project code
```

The developer should primarily interact with `gox` through commands rather than manually maintaining generator metadata.

---

# 65. Developer Experience

The desired experience is:

### Before gox

```text
Create folder
Create model
Create DTO
Create repository
Create service
Create handler
Create routes
Wire repository
Wire service
Wire handler
Register routes
Fix imports
Format
Build
Fix errors
```

### With gox

```bash
gox new payment-api
```

Answer architectural questions.

Then:

```bash
gox g module payment
```

Result:

```text
Complete payment feature
        +
Dependency wiring
        +
Route registration
        +
Formatted code
        +
Build validation
```

---

# 66. Design Principle

The most important UX distinction is:

```text
BAD

? Generate repository?
? Generate service?
? Generate handler?
? Generate routes?
? Generate DI?


GOOD

? What database do you want?
? Which HTTP framework?
? Do you need Redis?
? Do you need Kafka?
```

The first asks the developer to make implementation decisions that `gox` should already know.

The second asks the developer about architecture.

---

# 67. Success Criteria

`gox` is successful when a developer can:

```bash
gox new payment-api
```

answer a small number of architectural questions, then:

```bash
cd payment-api
gox g module payment
```

and receive a complete, compiling Go feature without manually wiring the application.

A generated project must remain:

* idiomatic Go
* explicit
* readable
* testable
* maintainable
* independent of `gox` at runtime

---

# 68. Final Product Model

The final mental model for `gox` is:

```text
                         gox
                          │
          ┌───────────────┼────────────────┐
          │               │                │
          ▼               ▼                ▼
       new app       add capability    generate feature
          │               │                │
          ▼               ▼                ▼
      architecture     infrastructure     module
       selection          │                │
          │               │                │
          └───────────────┼────────────────┘
                          ▼
                   Generated Go Code
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
           Handler      Service    Repository
              │           │           │
              └───────────┼───────────┘
                          ▼
                       Storage
```

The developer controls the architecture.

`gox` handles the repetition.

That is the core product.
