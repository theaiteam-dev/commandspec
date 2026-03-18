# CommandSpec — Design Spec

## Overview

A code generator that reads an OpenAPI 3.x spec and produces a complete, buildable Go CLI project using Cobra. The generated CLI maps API resources to subcommands, parameters to flags, and includes auth, output formatting, and help text out of the box.

## Architecture

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────────┐
│  OpenAPI Spec    │────▶│  Parse &     │────▶│  Go CLI Project │
│  (JSON or YAML)  │     │  Normalize   │     │  (Cobra + HTTP)  │
└─────────────────┘     └──────────────┘     └─────────────────┘
                              │
                        ┌─────┴─────┐
                        │ Internal  │
                        │ CLI Model │
                        └───────────┘
```

Three-phase pipeline:

1. **Parse** — Read OpenAPI spec into a normalized internal model
2. **Model** — Map API resources/endpoints to a CLI command tree
3. **Generate** — Template out a buildable Go project

## Phase 1: OpenAPI Parser

### Input

- OpenAPI 3.0 or 3.1 spec (JSON or YAML)
- Local file path or URL
- Resolve `$ref` references inline

### Internal Model

Each endpoint becomes a `Command` in the internal model:

```go
type APISpec struct {
    Name        string
    Description string
    BaseURL     string
    Auth        AuthConfig
    Resources   []Resource
}

type Resource struct {
    Name        string       // e.g., "users", "templates"
    Commands    []Command
    SubResources []Resource  // nested resources
}

type Command struct {
    Name        string       // e.g., "list", "create", "get", "delete"
    Description string
    Method      string       // HTTP method
    Path        string       // full path template
    Args        []Arg        // positional args (from path params)
    Flags       []Flag       // flags (from query + body params)
    RequestBody *BodySpec    // for POST/PUT/PATCH
    Response    *ResponseSpec
}

type Flag struct {
    Name        string
    Type        string       // string, int, bool, []string
    Required    bool
    Default     any
    Description string
    Source      string       // "query", "body", "header"
}
```

### Resource Grouping Strategy

Map URL paths to command groups:

| API Path | CLI Command |
|----------|-------------|
| `GET /users` | `myapi users list` |
| `POST /users` | `myapi users create` |
| `GET /users/{id}` | `myapi users get <id>` |
| `DELETE /users/{id}` | `myapi users delete <id>` |
| `GET /users/{id}/posts` | `myapi users posts list <user-id>` |
| `POST /users/{id}/posts` | `myapi users posts create <user-id>` |

Verb mapping from HTTP methods:

| HTTP Method | Default CLI Verb |
|-------------|-----------------|
| GET (collection) | `list` |
| GET (single) | `get` |
| POST | `create` |
| PUT | `update` |
| PATCH | `update` |
| DELETE | `delete` |

Custom `operationId` overrides the default verb when present.

### Path Parameter Handling

Path parameters (`{id}`, `{userId}`) become positional arguments:

```bash
# GET /users/{userId}/posts/{postId}
myapi users posts get <user-id> <post-id>
```

Rules:
- Strip common suffixes: `{userId}` → `<user-id>`, not `<user-id>`
- Order matches path order
- Required by definition (it's in the URL)

### Body Parameter Handling

For `POST`/`PUT`/`PATCH` with request bodies:

- **Flat objects**: Each top-level property becomes a flag
  ```bash
  myapi users create --name "Josh" --email "josh@example.com"
  ```
- **Nested objects**: Support dot notation or JSON string
  ```bash
  myapi users create --name "Josh" --address.city "Columbus"
  # or
  myapi users create --body '{"name": "Josh", "address": {"city": "Columbus"}}'
  ```
- **Always support `--body` flag**: Raw JSON input as escape hatch
- **Support `--body-file`**: Read JSON from file

## Phase 2: Code Generator

### Generated Project Structure

```
myapi/
├── cmd/
│   ├── root.go              # root command, global flags, config loading
│   ├── users.go             # `users` resource group
│   ├── users_list.go        # `users list` command
│   ├── users_create.go      # `users create` command
│   ├── users_get.go         # `users get` command
│   ├── users_delete.go      # `users delete` command
│   ├── templates.go         # `templates` resource group
│   ├── templates_list.go    # etc.
│   └── ...
├── internal/
│   ├── client.go            # HTTP client, base URL, auth injection
│   ├── config.go            # config file loading (~/.config/myapi/)
│   ├── output.go            # table, json, raw output formatters
│   └── errors.go            # error formatting, status code handling
├── main.go
├── go.mod
└── go.sum
```

### Generated Command Template

Each command file follows this pattern:

```go
// users_list.go
package cmd

import (
    "github.com/spf13/cobra"
)

var usersListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all users",
    Long:  `Retrieves a paginated list of users. Supports filtering by status and role.`,
    Example: `  myapi users list
  myapi users list --limit 50 --status active
  myapi users list --json | jq '.[] | .email'`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. Read flags
        // 2. Build HTTP request
        // 3. Execute via client
        // 4. Format output (table or JSON)
    },
}

func init() {
    usersCmd.AddCommand(usersListCmd)
    usersListCmd.Flags().IntP("limit", "l", 25, "Maximum number of results")
    usersListCmd.Flags().StringP("status", "s", "", "Filter by status")
}
```

### Global Flags (root.go)

Every generated CLI gets:

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON (for piping/jq) |
| `--verbose` | Show HTTP request/response details |
| `--config` | Path to config file |
| `--base-url` | Override API base URL |
| `--no-color` | Disable colored output |

### Auth Support

Generated `internal/config.go` supports:

1. **Environment variables**: `MYAPI_TOKEN`, `MYAPI_API_KEY`
2. **Config file**: `~/.config/myapi/config.yaml`
3. **Flag**: `--token` (for one-off use)

Auth types to support:
- Bearer token (`Authorization: Bearer <token>`)
- API key (custom header name from spec)
- Basic auth (`Authorization: Basic <base64>`)

The OpenAPI `securitySchemes` section drives which auth method is generated.

### Output Formatting

`internal/output.go` handles three modes:

- **Table** (default for humans): Columnar output, auto-width
- **JSON** (`--json`): Raw JSON response, pipe to `jq`
- **Quiet** (`-q`): IDs only, one per line (useful for scripting)

**Known table output limitations**: `PrintTable` does not recursively format nested objects within array rows or arrays of non-objects. These cases fall back to pretty-printed JSON output. Full nested-object rendering is planned for a future milestone.

## Phase 3: CLI for CommandSpec Itself

### Commands

```bash
# Generate a new CLI project from a spec
cmdspec init --schema <path-or-url> --name <cli-name> [--output-dir <dir>]

# Preview what would be generated (dry run)
cmdspec preview --schema <path-or-url>

# Regenerate after spec changes (preserves custom code in hooks)
cmdspec update --schema <path-or-url>

# Validate a spec is parseable
cmdspec validate --schema <path-or-url>
```

### Init Flow

```
cmdspec init --schema https://app.dittofeed.com/documentation/json --name dittofeed
```

1. Fetch and parse the OpenAPI spec
2. Build the internal CLI model
3. Show a summary: "Found 14 endpoints across 4 resources. Generate?"
4. Scaffold the Go project
5. Run `go mod tidy`
6. Print next steps: `cd dittofeed && go build -o dittofeed .`

### Update Flow

For when the API spec changes and you need to regenerate:

- Regenerate command files from the new spec
- Preserve any custom code the user added (via `// commandspec:custom` markers or a hooks system)
- Show a diff of what changed

## Phase 4: Edge Cases & Polish

### Naming Collisions

- If two endpoints map to the same command name, append the HTTP method: `users-update-put`, `users-update-patch`
- Reserved words (Go keywords, Cobra internals) get suffixed: `type` → `type-value`

### Pagination

Detect common pagination patterns and generate helper flags:

- `--all` flag to auto-paginate and collect all results
- `--page` / `--per-page` flags when pagination params exist

### Enum Parameters

OpenAPI `enum` values become flag validation:

```go
cmd.Flags().StringP("status", "s", "", "Filter by status (active|inactive|pending)")
// Generated with ValidArgsFunction for shell completion
```

### Array Parameters

Query params that accept arrays:

```bash
myapi users list --status active --status pending
# or
myapi users list --status active,pending
```

### File Uploads

Endpoints with `multipart/form-data`:

```bash
myapi documents upload --file ./report.pdf --name "Q4 Report"
```

## Implementation Plan

### Milestone 1: Core Pipeline (MVP) — ✓ COMPLETE

- [x] OpenAPI 3.0 JSON parser
- [x] Internal model builder (resources, commands, flags)
- [x] Go code generator with Cobra templates
- [x] Basic auth (Bearer token via env var)
- [x] JSON output mode
- [x] `cmdspec init` command

**Validation target**: Generate a working CLI from the Dittofeed OpenAPI spec. ✓ Completed with full HTTP client wiring, RunE handlers, conditional imports, and integration tests.

### Milestone 2: Rich Features

- [ ] YAML spec support
- [ ] Table output formatting
- [ ] `--body` and `--body-file` flags
- [ ] Nested object flag handling (dot notation)
- [ ] Shell completions generation
- [ ] Enum validation and completion
- [ ] `cmdspec validate` command

### Milestone 3: Lifecycle

- [ ] `cmdspec update` with diff and custom code preservation
- [ ] `cmdspec preview` dry run
- [ ] Pagination detection and `--all` flag
- [ ] File upload support
- [ ] Multiple auth scheme support

### Milestone 4: Distribution

- [ ] `go install` support
- [ ] Homebrew formula
- [ ] GitHub Actions for CI
- [ ] Docs site
- [ ] Example gallery (Dittofeed, Stripe, GitHub for comparison)

## Open Questions

1. **Generator language**: Write CommandSpec itself in Go (dogfood the ecosystem) or TypeScript (faster to prototype with OpenAPI tooling)?
2. **Custom code preservation**: Marker comments vs. separate hook files vs. partial regeneration?
3. **Config format**: YAML config files or stick with env vars only for simplicity?
4. **Shorthand flags**: Auto-assign `-n` for `--name`, `-l` for `--limit`, etc.? Could conflict.
5. **Workspace support**: Should generated CLIs support multiple profiles/environments (staging vs prod)?
