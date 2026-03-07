# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**Swagger Jack** is a code generator that reads an OpenAPI 3.x spec and produces a complete, buildable Go CLI project using Cobra. The generated CLIs map API resources to subcommands, path params to positional args, and query/body params to flags.

This project is in early development — see `docs/SPEC.md` for the full design spec and implementation milestones.

## Commands

```bash
go test -race ./...                  # run all tests
go test -race -run TestName ./...    # run a single test
golangci-lint run ./...              # lint
```

Note: `go version` may report a stale version in some shells. Use `/opt/homebrew/bin/go` if the toolchain appears broken.

## Architecture

Three-phase pipeline:

1. **Parse** — Read OpenAPI spec (JSON or YAML, local or URL) into a normalized internal model. Resolve `$ref` references inline.
2. **Model** — Map API resources/endpoints to a CLI command tree. HTTP verbs → CLI verbs (GET collection→`list`, GET single→`get`, POST→`create`, PUT/PATCH→`update`, DELETE→`delete`). Path params → positional args; query/body params → flags.
3. **Generate** — Template out a buildable Go project (Cobra + HTTP client).

### Internal Model (core types)

```
APISpec → []Resource → []Command
Command: Name, Method, Path, Args (positional), Flags, RequestBody, Response
Flag: Name, Type (string/int/bool/[]string), Required, Default, Source (query/body/header)
```

### Generated Project Layout

```
<name>/
├── cmd/
│   ├── root.go              # root command, global flags, config loading
│   ├── <resource>.go        # resource group command
│   └── <resource>_<verb>.go # individual commands (e.g., users_list.go)
├── internal/
│   ├── client.go            # HTTP client, auth injection
│   ├── config.go            # config file (~/.config/<name>/), env vars, --token flag
│   ├── output.go            # table/JSON/quiet output modes
│   └── errors.go            # error formatting, HTTP status handling
├── main.go
└── go.mod
```

### Swagger Jack CLI Commands

- `swaggerjack init --schema <path-or-url> --name <cli-name>` — Generate a new CLI project
- `swaggerjack preview --schema <path-or-url>` — Dry run, show what would be generated
- `swaggerjack update --schema <path-or-url>` — Regenerate after spec changes
- `swaggerjack validate --schema <path-or-url>` — Validate spec is parseable

## Key Design Decisions

- **Path params** become positional args; strip common suffix patterns (`{userId}` → `<user-id>`)
- **Body params**: flat objects → individual flags; nested → dot notation (`--address.city`) + always support `--body` (raw JSON) and `--body-file`
- **Naming collisions**: append HTTP method (`users-update-put` vs `users-update-patch`)
- **Auth**: driven by OpenAPI `securitySchemes` — Bearer token, API key (custom header), or Basic auth; sourced from env vars, config file, or `--token` flag
- **Every command** gets `--json` (raw JSON output), `--verbose`, `--config`, `--base-url`, `--no-color` global flags
- **Enum params** → flag validation + shell completion via `ValidArgsFunction`
- **Custom code preservation** (for `update`): marker comments `// swagger-jack:custom`
- **Validation target for MVP**: Generate a working CLI from the Dittofeed OpenAPI spec

## Implementation Milestones

See `docs/SPEC.md` for the full milestone breakdown. MVP (Milestone 1) targets: OpenAPI 3.0 JSON parser, internal model builder, Go/Cobra code generator, Bearer token auth, JSON output, `swaggerjack init`.

## A(i)-Team Integration

This project uses the A(i)-Team plugin for PRD-driven development.

### When to Use A(i)-Team

Use the A(i)-Team workflow when:
- Implementing features from a PRD document
- Working on multi-file changes that benefit from TDD
- Building features that need structured test → implement → review flow

### Commands

- `/ai-team:plan <prd-file>` - Decompose a PRD into tracked work items
- `/ai-team:run` - Execute the mission with parallel agents
- `/ai-team:status` - Check current progress
- `/ai-team:resume` - Resume an interrupted mission

### Workflow

1. Place your PRD in the `prd/` directory
2. Run `/ai-team:plan prd/your-feature.md`
3. Run `/ai-team:run` to execute

The A(i)-Team will:
- Break down the PRD into testable units
- Write tests first (TDD)
- Implement to pass tests
- Review each feature
- Probe for bugs
- Update documentation and commit

**Do NOT** work on PRD features directly without using `/ai-team:plan` first.
