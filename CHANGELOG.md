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

## [Unreleased]

No changes yet.
