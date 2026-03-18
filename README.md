# CommandSpec

Generate production-ready CLI tools from OpenAPI specs. Feed it a schema, get a `gh`-style CLI with proper help text, structured output, and auth baked in.

## Why

MCP servers are just fancy API wrappers. Claude Code and other AI agents already know how to use CLIs — they read `--help`, compose flags, and parse output. So instead of writing another JSON-RPC adapter, just generate a real CLI.

CommandSpec takes your OpenAPI/Swagger spec and generates a Go CLI project using Cobra. Path params become positional args, query/body params become flags, and every command gets `--json` output for machine consumption.

## What You Get

```bash
# From this OpenAPI spec...
cmdspec init --schema https://api.example.com/docs/json --name myapi

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
go install github.com/theaiteam-dev/commandspec@latest

# Generate a CLI from an OpenAPI spec
cmdspec init --schema ./openapi.yaml --name myapi

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
- **Pagination Support** — Automatic pagination flag generation (`--page`, `--per-page`, `--cursor`, `--all`) with FetchAll helper supporting offset, cursor, and keyset strategies
- **Enum Support** — Automatic extraction and validation of enum fields with shell tab completion
- **Table and JSON Output** — Generated CLIs default to table-formatted output with `--json` flag for JSON mode
- **Shell Completions** — Both CommandSpec and generated CLIs provide bash/zsh/fish completion support
- **Multi-Auth Support** — Supports Bearer tokens, API keys, and Basic auth simultaneously with config file + env var integration
- **File Upload Handling** — Automatic detection of multipart endpoints with file flag generation and upload support
- **Code Generation** — Complete, buildable Go projects with Cobra CLI framework
- **Validation** — Dry-run spec validation before code generation with auth detection
- **Preview Command** — Dry-run code generation showing files that would be created without writing
- **Update Command** — Regenerate CLIs from updated specs while preserving custom code blocks

## Usage

### Validate a spec (dry run)

```bash
cmdspec validate --schema ./openapi.yaml
# Output: Spec: My API (1.0.0)
#         5 resources
#         23 commands
```

### Preview generated code (dry run)

```bash
cmdspec preview --schema ./openapi.yaml --name myapi
# Shows all files that would be generated without writing to disk
```

### Generate a CLI from a spec

```bash
# From local file
cmdspec init --schema ./openapi.yaml --name myapi

# From URL
cmdspec init --schema https://api.example.com/docs/json --name myapi

# Custom output directory
cmdspec init --schema ./openapi.yaml --name myapi --output-dir ./generated
```

### Update an existing CLI with a new spec

```bash
# Regenerate from updated spec, preserving custom code
cd myapi
cmdspec update --schema ../openapi-v2.yaml

# Preview changes without writing
cmdspec update --schema ../openapi-v2.yaml --dry-run

# Suppress diff output
cmdspec update --schema ../openapi-v2.yaml --no-diff
```

Custom code blocks marked with `commandspec:custom:start` / `commandspec:custom:end` comments are automatically preserved during update.

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
go build -o cmdspec .

# Test against a real spec
./cmdspec validate --schema https://petstore.swagger.io/v2/swagger.json
```

## Status

**Milestone 3 complete**. Production-ready feature set now available:
- **Milestone 1 (MVP)**: OpenAPI 3.0 parser, CLI model builder, code generation, authentication, JSON output
- **Milestone 2 (Rich Features)**: YAML/URL spec loading, table output formatting, enum validation, nested dot-notation flags, body parameters, shell completions, integration tests
- **Milestone 3 (Pagination & Update)**: Pagination codegen, table-as-default output, multi-auth support, file upload detection, custom code preservation, preview and update commands

Implemented features:
- YAML and URL-based spec loading
- Enum field extraction with validation and tab completion
- Body parameter support (`--body`, `--body-file`)
- Nested object flags with dot-notation (3 levels deep)
- Table output as default with `--json` flag for JSON mode
- Pagination support (`--page`, `--per-page`, `--cursor`, `--all`) with FetchAll helper
- Multi-auth support (Bearer, API key, Basic auth)
- File upload detection and `multipart/form-data` handling
- Shell completion scripts (bash/zsh/fish)
- Enhanced validation command with auth detection
- Preview command for dry-run code generation
- Update command for regenerating CLIs with custom code preservation
- Comprehensive integration test suite

See [docs/SPEC.md](docs/SPEC.md) for full design spec and roadmap.

## License

MIT
