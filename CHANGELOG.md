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
- **swaggerjack init command** — Generate a new CLI project with `swaggerjack init --schema <path-or-url> --name <cli-name> [--output-dir <dir>]`. Validates schema, builds command tree, generates project, and runs `go mod tidy`.
- **swaggerjack validate command** — Dry-run validation: `swaggerjack validate --schema <path-or-url>` reports spec title, version, resource count, and total command count without generating code.
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
- **Shell completion command** — `swaggerjack completion` generates bash/zsh/fish completion scripts for the swagger-jack CLI (#WI-503)
- **Generated CLI shell completions** — Generated CLI projects include a `completion` subcommand for bash/zsh/fish support (#WI-507)
- **Enhanced validate command** — `swaggerjack validate` now detects auth schemes, reports exit codes, and supports YAML specs (#WI-509)
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

## [Unreleased]

No changes yet.
