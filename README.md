# Swagger Jack

Generate production-ready CLI tools from OpenAPI specs. Feed it a schema, get a `gh`-style CLI with proper help text, structured output, and auth baked in.

## Why

MCP servers are just fancy API wrappers. Claude Code and other AI agents already know how to use CLIs — they read `--help`, compose flags, and parse output. So instead of writing another JSON-RPC adapter, just generate a real CLI.

Swagger Jack takes your OpenAPI/Swagger spec and generates a Go CLI project using Cobra. Path params become positional args, query/body params become flags, and every command gets `--json` output for machine consumption.

## What You Get

```bash
# From this OpenAPI spec...
swaggerjack init --schema https://api.example.com/docs/json --name myapi

# ...you get a full CLI project
cd myapi && go build -o myapi .

# That works like this
myapi users list --limit 10 --json
myapi users create --name "Josh" --email "josh@example.com"
myapi templates render --template-id abc123 --user-id xyz
myapi --help
```

Every generated CLI includes:

- **Nested subcommands** grouped by resource (`users`, `templates`, `segments`)
- **Built-in auth** via env vars or config file
- **`--json` flag** on every command for structured output
- **Shell completions** for bash, zsh, fish
- **Help text** pulled straight from your API descriptions

## Quick Start

```bash
# Install
go install github.com/queso/swagger-jack@latest

# Generate a CLI from an OpenAPI spec
swaggerjack init --schema ./openapi.yaml --name myapi

# Build and use it
cd myapi
go build -o myapi .
./myapi --help
```

## Features

- **OpenAPI 3.x Parser** — Loads JSON/YAML specs from local files or URLs, resolves `$ref` inline
- **Smart Command Mapping** — Automatically converts API resources and endpoints to CLI subcommands (list, get, create, update, delete)
- **Parameter Handling** — Path params become positional args, query/body params become typed CLI flags with enum validation
- **Nested Object Flags** — Complex objects support dot-notation access (e.g., `--address.city`, `--metadata.tags.primary`)
- **Body Parameter Flags** — Write operations support `--body` (raw JSON) and `--body-file` (load from file) for inline payloads
- **Enum Support** — Automatic extraction and validation of enum fields with shell tab completion
- **Table and JSON Output** — Generated CLIs support pretty-printed tables and structured JSON output
- **Shell Completions** — Both swagger-jack and generated CLIs provide bash/zsh/fish completion support
- **Security Schemes** — Supports Bearer tokens, API keys, and Basic auth with config file + env var integration
- **Code Generation** — Complete, buildable Go projects with Cobra CLI framework
- **Validation** — Dry-run spec validation before code generation with auth detection

## Usage

### Validate a spec (dry run)

```bash
swaggerjack validate --schema ./openapi.yaml
# Output: Spec: My API (1.0.0)
#         5 resources
#         23 commands
```

### Generate a CLI from a spec

```bash
# From local file
swaggerjack init --schema ./openapi.yaml --name myapi

# From URL
swaggerjack init --schema https://api.example.com/docs/json --name myapi

# Custom output directory
swaggerjack init --schema ./openapi.yaml --name myapi --output-dir ./generated
```

### Generated project structure

```
myapi/
├── cmd/
│   ├── root.go              # root command setup, config loading
│   ├── users.go             # resource group command
│   ├── users_list.go        # GET /users
│   ├── users_create.go      # POST /users
│   ├── users_update.go      # PUT/PATCH /users/{id}
│   └── users_delete.go      # DELETE /users/{id}
├── internal/
│   ├── client.go            # HTTP client with auth
│   ├── config.go            # config file + env var loading
│   ├── output.go            # JSON and table output formatting
│   └── errors.go            # error handling and HTTP status codes
├── main.go
└── go.mod
```

### Using a generated CLI

```bash
cd myapi
go build -o myapi .

# List resources with JSON output
./myapi users list --limit 10 --json

# Create a resource
./myapi users create --name "Alice" --email "alice@example.com"

# Update a resource
./myapi users update 123 --name "Alice Johnson"

# Delete a resource
./myapi users delete 123

# View help
./myapi --help
./myapi users --help
./myapi users create --help
```

## Implementation

See [docs/SPEC.md](docs/SPEC.md) for architecture, design decisions, and implementation milestones.

## Development

```bash
# Run tests
go test -race ./...

# Run linter
golangci-lint run ./...

# Build the generator
go build -o swaggerjack .

# Test against a real spec
./swaggerjack validate --schema https://petstore.swagger.io/v2/swagger.json
```

## Status

**Milestone 2 complete**. Rich feature set now available:
- **Milestone 1 (MVP)**: OpenAPI 3.0 parser, CLI model builder, code generation, authentication, JSON output
- **Milestone 2 (Rich Features)**: YAML/URL spec loading, table output formatting, enum validation, nested dot-notation flags, body parameters, shell completions, integration tests

Implemented features:
- YAML and URL-based spec loading
- Enum field extraction with validation and tab completion
- Body parameter support (`--body`, `--body-file`)
- Nested object flags with dot-notation (3 levels deep)
- Table output formatting with `--json` override
- Shell completion scripts (bash/zsh/fish)
- Enhanced validation command with auth detection
- Comprehensive integration test suite

See [docs/SPEC.md](docs/SPEC.md) for full design spec and roadmap.

## License

MIT
