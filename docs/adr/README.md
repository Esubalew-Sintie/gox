# Architecture decision records

One file per decision, numbered in the order taken. A record is never edited after acceptance except to change its status; a change of mind is a new record that supersedes the old one. The full design these records support is [../plan.md](../plan.md).

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-pure-code-generator.md) | gox is a pure code generator, not a runtime | accepted |
| [0002](0002-ownership-model.md) | Three file classes, a manifest with hashes, verified atomic apply | accepted |
| [0003](0003-layout.md) | The project layout, hexagonal only in v1.0 | accepted |
| [0004](0004-wiring.md) | Generated composition root with fixed seams, `New` constructors, resolution by type | accepted |
| [0005](0005-codemod-scope.md) | Seven codemod files and what gox may do to them | accepted |
| [0006](0006-error-and-response-contract.md) | The response envelope, `ErrorMap` and `ErrorTypeMap`, no `ErrorCodeMap` | accepted |
| [0007](0007-technology-defaults.md) | Library defaults of generated projects | accepted |
| [0008](0008-command-set.md) | Command vocabulary and the field DSL | accepted |
| [0009](0009-rewrite-engine.md) | Rebuild the engine, remove the marker injector | accepted |
| [0010](0010-v1-scope.md) | v1.0 is phases 0 to 4 with all five databases | accepted |

To add one, copy [0000-template.md](0000-template.md), take the next number, and add a row here.
