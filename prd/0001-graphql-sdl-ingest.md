# PRD-0001: GraphQL SDL Ingest Mode

**Author:** Josh  **Date:** 2026-03-14  **Status:** Draft

## 1. Context & Background

swagger-jack today generates complete, buildable Go CLIs from OpenAPI 3.x specs. The tool is
useful for developers who consume REST APIs and want a CLI without writing one by hand.

A significant share of modern APIs — GitHub, Shopify, Stripe, Linear, and many others — are
GraphQL-first. These developers have no equivalent tool: they either hand-roll CLI wrappers,
reach for generic HTTP clients, or write scripts. swagger-jack could serve them directly with
minimal changes to the generator pipeline, since the internal model (`APISpec → Resource →
Command → Flag`) is already decoupled from the input format.

Adding GraphQL SDL as a first-class input format expands the addressable API surface the tool
can cover without changing the generation side at all.

## 2. Problem Statement

Developers who consume GraphQL APIs cannot use swagger-jack today. When they encounter a
GraphQL service they want to script or automate, they must build CLI tooling from scratch or
use generic, non-ergonomic clients. The barrier is that swagger-jack's parser only understands
OpenAPI — not GraphQL Schema Definition Language (SDL), the standard format for describing a
GraphQL schema.

## 3. Target Users & Use Cases

**Primary users:**
- Developer/API consumer — someone who regularly works with one or more GraphQL APIs and
  wants a generated CLI for querying or automating them. Same persona as the existing OpenAPI
  user.

**Key use cases:**
- A developer needs to script queries against a GraphQL API so that they can automate
  repetitive tasks without writing HTTP boilerplate.
- A developer needs to explore a GraphQL API's operations quickly so that they understand
  what commands are available without reading schema documentation.
- A developer needs to regenerate their CLI after the GraphQL schema changes so that new
  operations are available as commands without manual updates.
- A developer needs to override the auto-inferred resource grouping so that their CLI's
  command tree matches their mental model of the API.

## 4. Goals & Success Metrics

| Goal | Metric | Target |
|------|--------|--------|
| Correct command generation | All Query and Mutation fields produce a command in the generated CLI | 100% of fields (excluding Subscriptions) |
| Ergonomic flag surface | Simple scalar/enum arguments map to typed flags | All scalar and enum args become flags |
| Complex input escape hatch | List-of-objects and deep-nested inputs get a `--<arg>-json` flag | 100% of complex args have a JSON escape hatch |
| Real-world viability | A working CLI can be generated from a well-known public GraphQL schema | At least one of: GitHub, Shopify, Linear |
| Format transparency | No user action required to use SDL vs OpenAPI | Auto-detection works for `.graphql`, `.gql`, and SDL content |

## 5. Scope

### In Scope

- Parse GraphQL SDL files (`.graphql`, `.gql`) into the existing internal `APISpec` model
- Map `Query` root type fields → `get` or `list` verb (by whether return type is a list)
- Map `Mutation` root type fields → `create`, `update`, or `delete` verb via name heuristics;
  unrecognised mutations default to a `run` verb
- Skip `Subscription` root type fields (no command generated, no error)
- Infer resource groups from field names (`createUser`, `listUsers` → `users` group) using
  naming convention heuristics
- Optional manifest file (`swagger-jack.graphql.yaml` alongside the SDL) to override
  resource grouping, verb assignment, or field inclusion/exclusion
- Scalar and enum arguments → typed CLI flags (string, int, bool, float, enum with validation)
- Non-null arguments (`!`) → required flags
- Shallow input object arguments (depth 1, scalar/enum fields only) → dot-notation flags
  (`--address.city`)
- List-of-objects arguments and input objects deeper than depth 1 → `--<arg>-json` flag
  accepting raw JSON
- Auto-detect Relay cursor pagination (`first`, `after`, `last`, `before` args +
  `pageInfo` in response) and wire up `FetchAll` pagination support
- Auto-detect offset pagination (`limit`/`offset` args) and wire up pagination support
- Auto-detect input format from file extension (`.graphql`, `.gql`) and SDL content
  (heuristic: presence of `type Query` or `type Mutation`) so `swaggerjack init --schema`
  works without a `--mode` flag
- `swaggerjack update` works with SDL schemas (same preserve + diff flow as OpenAPI)
- `swaggerjack preview` works with SDL schemas

### Out of Scope

- GraphQL Subscriptions (separate effort; requires streaming/websocket support)
- Live introspection against a running endpoint (SDL file only; introspection endpoint
  support is a separate PRD)
- Generating mutations that send variables as a GraphQL variables file (the `--body-file`
  equivalent); raw JSON escape hatch covers this use case for now
- Custom scalar type plugins (custom scalars treated as `string`)
- GraphQL directives (ignored; no effect on generated commands)
- Federation / schema stitching support
- Fragment or operation document input (SDL only, not `.graphql` operation documents)

## 6. Requirements

### Functional Requirements

**Parsing**

1. The tool shall parse a GraphQL SDL file and extract all fields from the `Query` and
   `Mutation` root types.
2. The parser shall resolve named types (input objects, enums, scalars) referenced by
   arguments inline.
3. The parser shall ignore `Subscription` root type fields without emitting a warning or
   error.
4. If the SDL file is syntactically invalid, the tool shall emit a human-readable error
   message identifying the file and location of the parse failure.

**Resource Grouping**

5. The tool shall infer a resource group name from field names using the following heuristics,
   applied in order: strip a leading verb prefix (`get`, `list`, `create`, `update`, `delete`,
   `fetch`, `find`, `search`, `add`, `remove`, `set`, `upsert`), then singularise the
   remainder and use it as the resource name (e.g., `listUsers` → `users`, `createOrder` →
   `orders`).
6. If the heuristic produces an empty or ambiguous resource name, the field name itself
   (kebab-cased) shall be used as both the resource and command name.
7. A manifest file (`swagger-jack.graphql.yaml`) in the same directory as the SDL shall
   override inferred resource groups and verb assignments for specific fields.
8. Fields listed in the manifest as `exclude: true` shall be omitted from the generated CLI.

**Verb Mapping**

9. Fields in the `Query` root type shall map to `list` if the return type is a list (or
   connection type), and `get` otherwise.
10. Fields in the `Mutation` root type shall map to verbs via name prefix: `create*` →
    `create`, `update*`/`edit*`/`modify*`/`patch*` → `update`, `delete*`/`remove*`/
    `destroy*` → `delete`. Unmatched mutations shall map to verb `run`.
11. If two fields in the same resource group resolve to the same verb, the full field name
    (kebab-cased) shall be used as the command name instead to avoid collisions.

**Argument → Flag Mapping**

12. Scalar arguments (`String`, `Int`, `Float`, `Boolean`, `ID`) shall map to typed flags
    (`string`, `int`, `float64`, `bool`). `ID` maps to `string`.
13. Enum arguments shall map to string flags with `ValidArgs` validation and shell completion.
14. Non-null arguments (`!`) shall produce required flags.
15. If a mutation field has exactly one argument and that argument is an input object type,
    the input object's fields shall be unwrapped and exposed as top-level flags (e.g.,
    `createUser(input: CreateUserInput!)` → `--name`, `--email`, not `--input.name`).
    If the unwrapped input object contains nested objects or lists, those nested fields shall
    follow rule 16 using the field name (not `input`) as the prefix.
16. Input object arguments that are not subject to rule 15 and have all scalar/enum fields
    (depth 1) shall be flattened to dot-notation flags (`--<arg>.<field>`).
17. Input object arguments that contain nested objects or lists, and any list-of-objects
    arguments, shall produce a single `--<arg>-json` flag accepting a raw JSON string.
18. List-of-scalar arguments (`[String]`) shall map to a repeatable flag (`--<arg> val1
    --<arg> val2`).

**Pagination**

19. If a `Query` field's arguments include `first` and `after` (Relay cursor style), and its
    return type contains a `pageInfo` field with `hasNextPage` and `endCursor`, the tool shall
    wire up the existing `FetchAll` pagination helper for that command.
20. If a `Query` field's arguments include `limit` and `offset`, the tool shall wire up offset
    pagination for that command.
21. Pagination wiring shall be identical in behaviour to the existing OpenAPI pagination
    support.

**Format Auto-Detection**

22. `swaggerjack init --schema <path>` shall detect GraphQL SDL automatically if the file
    extension is `.graphql` or `.gql`, or if the file content contains a `type Query` or
    `type Mutation` declaration.
23. OpenAPI detection shall take precedence when the file contains an `openapi:` key.
24. If format cannot be determined, the tool shall emit an actionable error message.

**Validate Command**

25. `swaggerjack validate --schema <path>` shall work with SDL files, reporting: parse
    errors, fields that could not be mapped to a resource/verb, arguments that will fall
    back to `--<arg>-json`, and any manifest override warnings.
26. Validate shall exit non-zero if the SDL contains parse errors; it shall exit zero with
    warnings for mapping issues that still produce a valid (if imperfect) CLI.

**Manifest File**

27. The manifest shall be a YAML file with the following structure, all fields optional:

    ```yaml
    overrides:
      <fieldName>:
        resource: <resource-name>   # override inferred resource group
        verb: <verb>                # override inferred verb
        exclude: true               # omit this field from the generated CLI
    ```

28. The manifest shall be loaded automatically if it exists alongside the SDL file; no
    flag is required to activate it.

### Non-Functional Requirements

1. Parsing a 10,000-line SDL file shall complete in under 2 seconds on standard developer
   hardware.
2. The SDL parser shall not make network requests.
3. The existing OpenAPI code path shall be unaffected; all new logic shall live in a new
   `internal/graphql/` package.
4. Generated code for GraphQL-sourced CLIs shall pass `go build` and `go vet` without errors.
5. Generated code shall be covered by the same `go test -race ./...` test suite as OpenAPI
   generated output.

### Edge Cases & Error States

- **Schema with no Query or Mutation types:** emit a warning and produce an empty CLI project
  (no resource commands); do not error out.
- **Field with no arguments:** generates a command with no flags (valid; command sends a
  zero-argument operation).
- **All mutations unrecognised by verb heuristic:** all map to `run` verb; resource group
  disambiguates. No error.
- **Name collision after heuristic (two fields → same resource + verb):** full field name
  used as command name for both; log a notice.
- **Manifest references a field that doesn't exist in the SDL:** emit a warning; skip the
  override; do not error out.
- **SDL file contains only `Subscription` type:** emit an informational message ("No Query
  or Mutation fields found; Subscriptions are not supported"); produce empty CLI.
- **Custom scalar type:** treat as `string`; no warning.
- **Circular input type reference:** detect and break the cycle; treat the recursive field
  as `--<field>-json`.
- **`swaggerjack update` with a field that was removed from the schema:** treat as an orphan
  command (existing orphan detection logic applies).

## 7. Design Principles

- **Convention over configuration:** naming heuristics should produce a usable CLI from any
  well-named GraphQL schema with zero configuration. The manifest exists for when conventions
  fall short, not as a required step.
- **Ergonomics degrade gracefully:** simple args → clean flags; complex args → JSON escape
  hatch. Users should never be blocked from calling an operation just because its input is
  complex.
- **No surprises in the generated output:** the same SDL in should always produce the same
  CLI out. Heuristics are deterministic; no randomness or environment-dependent behaviour.
- **Pipeline consistency:** GraphQL SDL is just another path into the same internal model.
  A developer looking at generated code should not be able to tell whether it came from an
  OpenAPI spec or a GraphQL SDL.

## 8. Solution Approach

The existing pipeline has a clean separation between parsing (input format) and generation
(output). Adding GraphQL support means adding a new parser that reads SDL and produces the
same internal `APISpec` structure the OpenAPI parser already produces. The generator sees no
difference.

The SDL parser reads the schema file, walks the `Query` and `Mutation` root types, and for
each field applies naming heuristics to determine resource group and verb. Arguments are
inspected and classified as scalar flags, dot-notation flags, or JSON-escape-hatch flags
depending on their type and depth. A manifest file, if present, is applied as a final
override pass before the `APISpec` is handed to the generator.

Format auto-detection happens at the `swaggerjack init` entry point: the tool inspects the
file extension and, as a fallback, a small set of content signatures. No user-facing flag
is needed.

The manifest format is intentionally minimal — a YAML file that maps field names to
resource/verb overrides or exclusions. It lives alongside the SDL file for easy versioning.

## 9. Technical Considerations

**Constraints:**
- Must integrate with the existing `internal/model` types (`APISpec`, `Resource`, `Command`,
  `Flag`, `FlagType`) without modifying those types.
- Must not break the existing OpenAPI pipeline.
- Go module must remain buildable; the GraphQL parsing library is a new dependency.

**Dependencies:**
- `github.com/vektah/gqlparser/v2` — well-maintained Go GraphQL parser; supports full SDL
  including schema extensions, directives, and built-in scalars.
- No other new external dependencies required.

**Integration points:**
- `cmd/init.go` — format auto-detection logic
- `cmd/update.go` — must pass SDL files through the preserve + diff flow
- `cmd/preview.go` — no changes needed if it calls the same pipeline entry point
- `internal/parser/` — new `graphql/` sub-package or sibling package

## 10. Risks & Open Questions

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Naming heuristics produce poor resource grouping for non-standard schemas | Medium | Confusing CLI command tree | Manifest override available; document common patterns |
| `gqlparser/v2` lacks a feature needed (e.g., schema extensions) | Low | Blocked on a real-world schema | Evaluate against GitHub/Shopify SDL before committing |
| Relay pagination detection false-positives | Low | Incorrect FetchAll wiring | Require both args (`first`/`after`) AND response `pageInfo` to trigger |
| Circular input type references cause infinite recursion | Low | Parser crash | Detect and break cycles; tested with self-referential schema |
| Generated CLI for a large schema (500+ operations) has long build times | Low | Poor UX for large schemas | No action now; note as known limitation |

### Resolved

- **Manifest filename:** Fixed at `swagger-jack.graphql.yaml`. Not configurable for now;
  add a `--manifest` flag only if users request it.
- **Single `input:` argument unwrapping:** Auto-unwrap. If a mutation has exactly one input
  object argument, flatten its fields to top-level flags. More ergonomic; the special case
  is straightforward.
- **`swaggerjack validate` for SDL:** Yes, include. Minimal incremental work on top of the
  parser/mapper; reports parse errors, mapping issues, and JSON-fallback notices.
- **Union/Interface return types:** Print raw JSON output for those commands. No special
  handling needed.
