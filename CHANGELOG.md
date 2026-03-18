# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-07

### Added

- **OpenAPI 3.x Parser** — Load JSON/YAML specs from local files or URLs with full support for `$ref` reference resolution. Returns normalized internal model via `parser.Load()`.
- **Internal Model Builder** — Convert OpenAPI specs into CLI command trees. HTTP verbs map to CLI verbs (GET collection→`list`, GET single→`get`, POST→`create`, PUT/PATCH→`update`, DELETE→`delete`). Path parameters become positional arguments, query/body parameters become CLI flags. Supports operationId overrides and automatic naming collision resolution.
- **Security Scheme Extraction** — Extract Bearer token, API key, and Basic auth configurations from OpenAPI spec. Route credentials through environment variables, config files, and `--token` flags.
- **Go/Cobra Code Generator** — Generate complete, buildable Go projects using Cobra framework. Outputs main.go, go.mod, cmd/root.go, cmd/<resource>.go, cmd/<resource>_<verb>.go, and internal packages (client.go, config.go, output.go, errors.go). Validates CLI names (prevents empty, whitespace, backticks, and Go reserved keywords).
- **cmdspec init command** — Generate a new CLI project with `cmdspec init --schema <path-or-url> --name <cli-name> [--output-dir <dir>]`. Validates schema, builds command tree, generates project, and runs `go mod tidy`.
- **cmdspec validate command** — Dry-run validation: `cmdspec validate --schema <path-or-url>` reports spec title, version, resource count, and total command count without generating code.
- **Comprehensive test coverage** — Unit tests for parser, model builder, and code generator with real OpenAPI fixtures.

### Implementation Details

- Parser resolves `$ref` inline and normalizes specs to internal `parser.Result` structure
- Model builder implements `model.SpecProvider` and `model.RawJSONProvider` interfaces
- Code generator uses Go's `text/template` with safe string interpolation via `goString` FuncMap
- All generated CLIs include `--json` flag for structured machine-readable output
- Flag parsing supports both simple values and nested dot-notation for complex objects

## [0.2.0] - 2026-03-10

### Added

- **YAML spec loading** — Parser now accepts `.yaml` and `.yml` files in addition to JSON (#WI-501)
- **URL-based spec loading** — `parser.Load()` accepts `http://` and `https://` URLs with automatic content negotiation (#WI-512)
- **Enum field extraction** — Model builder extracts enum values from OpenAPI schemas and stores in `Flag.Enum []string` (#WI-502)
- **Enum validation and tab completion** — Generated commands validate flag values against enum constraints and provide shell tab completion for enum fields (#WI-508)
- **Body parameter flags** — Generated commands support `--body` (raw JSON) and `--body-file` (JSON file) for write operations (#WI-505)
- **Nested dot-notation flags** — Complex object parameters support nested access up to 3 levels deep (e.g., `--address.city`, `--metadata.tags.primary`) (#WI-506)
- **Table output formatting** — Generated CLIs support pretty-printed table output via `internal/output.go` with `tablewriter` (#WI-504)
- **Wire table output into commands** — Generated command `RunE` functions automatically call `PrintTable()` when `--json` flag is not set (#WI-511)
- **Shell completion command** — `cmdspec completion` generates bash/zsh/fish completion scripts for the CommandSpec CLI (#WI-503)
- **Generated CLI shell completions** — Generated CLI projects include a `completion` subcommand for bash/zsh/fish support (#WI-507)
- **Enhanced validate command** — `cmdspec validate` now detects auth schemes, reports exit codes, and supports YAML specs (#WI-509)
- **Integration tests** — Comprehensive integration test suite covering body flags, table output, nested fields, YAML loading, and URL-based specs (#WI-510)

### Changed

- Generated CLI projects now include table output support alongside JSON mode
- Enhanced spec validation with better error reporting and auth detection

### Implementation Details

- `Flag.Enum` contains possible values for enum parameters, enabling validation and completion
- Table output uses `tablewriter` library for consistent formatting across generated CLIs
- Nested flags use dot-notation parser to map flat CLI flags to nested object structures
- URL loader supports both http/https with standard Go HTTP client
- YAML parsing uses Go's standard yaml library with JSON fallback

## [0.3.0] - 2026-03-11

### Added

- **Pagination code generation** — Generated CLIs automatically include `--page`, `--per-page`, `--cursor`, and `--all` flags for paginated endpoints. Offset, cursor, and keyset pagination strategies supported via `FetchAll` helper in generated `internal/client/pagination.go` (#WI-511, #WI-517)
- **cmdspec preview command** — Dry-run code generation showing all files that would be created without writing to disk. Useful for previewing changes before running `init` or `update` (#WI-512)
- **Table output as default** — Generated CLIs now default to table-formatted output with `--json` flag available for raw JSON (#WI-513)
- **Multi-auth client support** — Generated CLIs support Bearer token, API key (custom header), and Basic authentication schemes simultaneously. Auth method automatically selected based on OpenAPI `securitySchemes` configuration (#WI-514, #WI-516, #WI-520)
- **File upload detection** — Model builder detects `multipart/form-data` endpoints and creates `FlagTypeFile` flags. Generated commands use `DoMultipart()` for upload requests (#WI-515, #WI-519)
- **Custom code preservation** — `internal/preserve` package enables updating generated code while preserving hand-written code blocks marked with `commandspec:custom:start` / `commandspec:custom:end` comments. Handles CRLF normalization and orphan comment-out fallback (#WI-518)
- **cmdspec update command** — Regenerate existing CLI project from updated OpenAPI spec, preserving custom code blocks. Supports `--dry-run` and `--no-diff` flags. Unified diff output shows file changes, orphan file warnings (#WI-521)
- **Pagination helper strategies** — `FetchAll()` supports offset, cursor, and keyset pagination with error-response termination. Respects user-provided `--page` N and `--limit` N flag values (#WI-517)

### Fixed

- SA1019 deprecation in `cmd/validate.go` — Replaced `parser.SetHTTPTimeout` with `parser.LoadWithTimeout` for proper timeout handling (#WI-521)
- Pagination error response handling — FetchAll now terminates pagination loop on error responses instead of continuing to next page (#WI-517)
- Generated `root.go` extension points — Added `init-hook` marker for custom initialization logic in updated projects (#WI-521)

### Changed

- Generated CLIs now default to table output with optional `--json` flag (instead of defaulting to JSON)
- Generated `root.go` now includes `init-hook` marker for custom code insertion points during `cmdspec update`
- Model builder now extracts pagination parameters from OpenAPI schemas automatically

### Implementation Details

- Pagination pagination strategies in `internal/generator/pagination.go` template
- Auth scheme wiring emits correct environment variable lookups per scheme type via `GenerateVerbCmdWithAuth`
- File upload support uses `DoMultipart` client method with per-flag scoped upload blocks
- Custom code preservation uses comment-marker scanning with CRLF-aware merge
- `cmdspec update` command integrates preserve package for seamless regeneration

## [Unreleased]

No changes yet.
