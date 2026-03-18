// Package generator_test contains TDD tests for multi-auth-scheme support in
// generated verb command RunE bodies (WI-520).
//
// These tests define the acceptance criteria for wiring security schemes into
// GenerateVerbCmd. They MUST FAIL before implementation because:
//
//   - GenerateVerbCmd currently has no SecuritySchemes parameter, so the new
//     GenerateVerbCmdWithAuth function tested here does not yet exist.
//   - Even if a new overload existed, buildRunEBody always emits the _TOKEN
//     env var regardless of scheme type — it never reads API key env vars.
//
// B.A. must either add a new GenerateVerbCmdWithAuth(resource, cmd, cliName,
// schemes) function or extend GenerateVerbCmd with an optional schemes param.
package generator_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers shared by auth tests
// ---------------------------------------------------------------------------

// minimalListCmd returns a simple GET /items command with no args or flags —
// the smallest possible command for testing RunE body auth wiring.
func minimalListCmd() model.Command {
	return model.Command{
		Name:        "list",
		HTTPMethod:  "GET",
		Path:        "/items",
		Description: "List all items",
	}
}

// minimalItemsResource wraps a command in a simple resource.
func minimalItemsResource() model.Resource {
	return model.Resource{
		Name:        "items",
		Description: "Item management",
	}
}

// bearerOnlySchemes returns a SecuritySchemes map with a single Bearer scheme.
func bearerOnlySchemes(cliName string) map[string]model.SecurityScheme {
	envPrefix := strings.ToUpper(strings.ReplaceAll(cliName, "-", "_"))
	return map[string]model.SecurityScheme{
		"bearerAuth": {
			Type:   model.SecuritySchemeBearer,
			EnvVar: envPrefix + "_TOKEN",
		},
	}
}

// apiKeyOnlySchemes returns a SecuritySchemes map with a single API key scheme.
func apiKeyOnlySchemes(cliName string) map[string]model.SecurityScheme {
	envPrefix := strings.ToUpper(strings.ReplaceAll(cliName, "-", "_"))
	return map[string]model.SecurityScheme{
		"apiKeyAuth": {
			Type:       model.SecuritySchemeAPIKey,
			HeaderName: "X-API-Key",
			EnvVar:     envPrefix + "_API_KEY",
		},
	}
}

// basicOnlySchemes returns a SecuritySchemes map with a single Basic auth scheme.
func basicOnlySchemes(cliName string) map[string]model.SecurityScheme {
	envPrefix := strings.ToUpper(strings.ReplaceAll(cliName, "-", "_"))
	return map[string]model.SecurityScheme{
		"basicAuth": {
			Type:   model.SecuritySchemeBasic,
			EnvVar: envPrefix + "_TOKEN",
		},
	}
}

// multiSchemes returns a SecuritySchemes map with both Bearer and API key.
func multiSchemes(cliName string) map[string]model.SecurityScheme {
	envPrefix := strings.ToUpper(strings.ReplaceAll(cliName, "-", "_"))
	return map[string]model.SecurityScheme{
		"bearerAuth": {
			Type:   model.SecuritySchemeBearer,
			EnvVar: envPrefix + "_TOKEN",
		},
		"apiKeyAuth": {
			Type:       model.SecuritySchemeAPIKey,
			HeaderName: "X-API-Key",
			EnvVar:     envPrefix + "_API_KEY",
		},
	}
}

// ---------------------------------------------------------------------------
// Test 1 — Bearer-only spec (backward-compat)
// ---------------------------------------------------------------------------

// TestGenerateVerbCmdWithAuth_BearerOnly_TokenEnvVar verifies that a Bearer-only
// spec generates os.Getenv("MYAPI_TOKEN") in the RunE body — preserving the
// existing behavior so callers that relied on _TOKEN are unaffected.
//
// FAILS until GenerateVerbCmdWithAuth is implemented; the function does not
// exist yet in the generator package.
func TestGenerateVerbCmdWithAuth_BearerOnly_TokenEnvVar(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := bearerOnlySchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	assert.Contains(t, src, `os.Getenv("MYAPI_TOKEN")`,
		"Bearer-only spec should generate os.Getenv for the _TOKEN env var")
}

// TestGenerateVerbCmdWithAuth_BearerOnly_PassesTokenToNewClient verifies that
// the generated RunE passes the Bearer token to client.NewClient.
//
// FAILS until GenerateVerbCmdWithAuth is implemented.
func TestGenerateVerbCmdWithAuth_BearerOnly_PassesTokenToNewClient(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := bearerOnlySchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	assert.Contains(t, src, "client.NewClient",
		"generated RunE should call client.NewClient")
	// The token variable should be passed to NewClient.
	assert.Contains(t, src, "token",
		"generated RunE should pass a token variable to NewClient")
}

// ---------------------------------------------------------------------------
// Test 2 — API key spec
// ---------------------------------------------------------------------------

// TestGenerateVerbCmdWithAuth_APIKey_ApiKeyEnvVar verifies that an API-key-only
// spec generates os.Getenv("MYAPI_API_KEY") in the RunE body — NOT _TOKEN.
//
// FAILS until GenerateVerbCmdWithAuth uses scheme type to pick the env var,
// since the current buildRunEBody always emits _TOKEN unconditionally.
func TestGenerateVerbCmdWithAuth_APIKey_ApiKeyEnvVar(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := apiKeyOnlySchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	assert.Contains(t, src, `os.Getenv("MYAPI_API_KEY")`,
		"API-key-only spec should read MYAPI_API_KEY env var, not MYAPI_TOKEN")
}

// TestGenerateVerbCmdWithAuth_APIKey_DoesNotEmitTokenVar verifies that an
// API-key-only spec does NOT generate an unused MYAPI_TOKEN lookup, keeping the
// generated code clean.
//
// FAILS until GenerateVerbCmdWithAuth conditionally omits the _TOKEN line for
// non-Bearer specs.
func TestGenerateVerbCmdWithAuth_APIKey_DoesNotEmitTokenVar(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := apiKeyOnlySchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	assert.NotContains(t, src, `os.Getenv("MYAPI_TOKEN")`,
		"API-key-only spec should not emit an unused MYAPI_TOKEN lookup")
}

// ---------------------------------------------------------------------------
// Test 3 — Basic auth spec
// ---------------------------------------------------------------------------

// TestGenerateVerbCmdWithAuth_Basic_TokenEnvVar verifies that a Basic-auth
// spec reads credentials from the env var defined in the scheme — which by
// convention is _TOKEN for Basic auth.
//
// FAILS until GenerateVerbCmdWithAuth is implemented (function does not exist).
func TestGenerateVerbCmdWithAuth_Basic_TokenEnvVar(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := basicOnlySchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	assert.Contains(t, src, `os.Getenv("MYAPI_TOKEN")`,
		"Basic-auth spec should read the _TOKEN env var for credentials")
}

// ---------------------------------------------------------------------------
// Test 4 — Multi-scheme spec (Bearer + API key)
// ---------------------------------------------------------------------------

// TestGenerateVerbCmdWithAuth_MultiScheme_BothEnvVarsPresent verifies that a
// spec with both Bearer and API key schemes reads BOTH env vars in the RunE
// body so the generated command can supply all credentials to the client.
//
// FAILS until GenerateVerbCmdWithAuth iterates all schemes and emits an env var
// lookup for each distinct credential, rather than always emitting just _TOKEN.
func TestGenerateVerbCmdWithAuth_MultiScheme_BothEnvVarsPresent(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := multiSchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	assert.Contains(t, src, `os.Getenv("MYAPI_TOKEN")`,
		"multi-scheme spec should read MYAPI_TOKEN for the Bearer scheme")
	assert.Contains(t, src, `os.Getenv("MYAPI_API_KEY")`,
		"multi-scheme spec should read MYAPI_API_KEY for the API key scheme")
}

// ---------------------------------------------------------------------------
// Test 5 — Valid Go syntax for each auth case
// ---------------------------------------------------------------------------

// TestGenerateVerbCmdWithAuth_BearerOnly_ValidGoSyntax verifies that the
// generated source for a Bearer-only spec parses as valid Go.
//
// FAILS until GenerateVerbCmdWithAuth is implemented.
func TestGenerateVerbCmdWithAuth_BearerOnly_ValidGoSyntax(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := bearerOnlySchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "items_list.go", src, parser.AllErrors)
	assert.NoError(t, parseErr,
		"Bearer-only generated source must be valid Go:\n%s", src)
}

// TestGenerateVerbCmdWithAuth_APIKey_ValidGoSyntax verifies that the generated
// source for an API-key-only spec parses as valid Go.
//
// FAILS until GenerateVerbCmdWithAuth is implemented.
func TestGenerateVerbCmdWithAuth_APIKey_ValidGoSyntax(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := apiKeyOnlySchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "items_list.go", src, parser.AllErrors)
	assert.NoError(t, parseErr,
		"API-key generated source must be valid Go:\n%s", src)
}

// TestGenerateVerbCmdWithAuth_Basic_ValidGoSyntax verifies that the generated
// source for a Basic auth spec parses as valid Go.
//
// FAILS until GenerateVerbCmdWithAuth is implemented.
func TestGenerateVerbCmdWithAuth_Basic_ValidGoSyntax(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := basicOnlySchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "items_list.go", src, parser.AllErrors)
	assert.NoError(t, parseErr,
		"Basic auth generated source must be valid Go:\n%s", src)
}

// TestGenerateVerbCmdWithAuth_MultiScheme_ValidGoSyntax verifies that the
// generated source for a multi-scheme spec parses as valid Go.
//
// FAILS until GenerateVerbCmdWithAuth is implemented.
func TestGenerateVerbCmdWithAuth_MultiScheme_ValidGoSyntax(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()
	schemes := multiSchemes("myapi")

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", schemes)
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "items_list.go", src, parser.AllErrors)
	assert.NoError(t, parseErr,
		"multi-scheme generated source must be valid Go:\n%s", src)
}

// ---------------------------------------------------------------------------
// Test 6 — Backward compatibility: existing GenerateVerbCmd still works
// ---------------------------------------------------------------------------

// TestGenerateVerbCmd_BackwardCompat_BearerTokenEnvVar verifies that the
// existing GenerateVerbCmd (without schemes param) continues to emit
// os.Getenv("MYAPI_TOKEN") — no regression from adding the auth-aware variant.
//
// This test PASSES with the current code if the package builds. It exists to
// guard against regressions when B.A. refactors buildRunEBody.
func TestGenerateVerbCmd_BackwardCompat_BearerTokenEnvVar(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()

	src, err := generator.GenerateVerbCmd(res, cmd, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, `os.Getenv("MYAPI_TOKEN")`,
		"existing GenerateVerbCmd (no schemes) must still emit _TOKEN env var lookup")
}

// TestGenerateVerbCmd_BackwardCompat_HyphenatedCliName verifies that a
// hyphenated CLI name (e.g. "my-api") still produces the correct env prefix
// (MY_API_TOKEN) with underscores, not hyphens.
//
// This test PASSES with the current code if the package builds. It guards
// against regressions in cliNameToEnvPrefix during refactoring.
func TestGenerateVerbCmd_BackwardCompat_HyphenatedCliName(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()

	src, err := generator.GenerateVerbCmd(res, cmd, "my-api")
	require.NoError(t, err)

	assert.Contains(t, src, `os.Getenv("MY_API_TOKEN")`,
		"hyphenated CLI name must produce underscore-separated env prefix, not hyphens")
}

// TestGenerateVerbCmdWithAuth_NilSchemes_FallsBackToTokenEnvVar verifies that
// passing a nil schemes map falls back to the Bearer token behavior — providing
// a safe default for callers that don't have scheme info.
//
// FAILS until GenerateVerbCmdWithAuth is implemented.
func TestGenerateVerbCmdWithAuth_NilSchemes_FallsBackToTokenEnvVar(t *testing.T) {
	res := minimalItemsResource()
	cmd := minimalListCmd()

	src, err := generator.GenerateVerbCmdWithAuth(res, cmd, "myapi", nil)
	require.NoError(t, err)

	assert.Contains(t, src, `os.Getenv("MYAPI_TOKEN")`,
		"nil schemes should fall back to default Bearer token env var lookup")
}
