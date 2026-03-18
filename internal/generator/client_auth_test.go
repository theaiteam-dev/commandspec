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

// specWithSchemes returns an APISpec with the given security schemes.
func specWithSchemes(schemes map[string]model.SecurityScheme) *model.APISpec {
	return &model.APISpec{
		Title:           "Auth Test API",
		Version:         "1.0.0",
		BaseURL:         "https://api.example.com",
		SecuritySchemes: schemes,
	}
}

// bearerScheme returns a SecurityScheme for Bearer token auth.
func bearerScheme(envVar string) model.SecurityScheme {
	return model.SecurityScheme{
		Type:   model.SecuritySchemeBearer,
		EnvVar: envVar,
	}
}

// apiKeyScheme returns a SecurityScheme for API key auth.
func apiKeyScheme(headerName, envVar string) model.SecurityScheme {
	return model.SecurityScheme{
		Type:       model.SecuritySchemeAPIKey,
		HeaderName: headerName,
		EnvVar:     envVar,
	}
}

// basicScheme returns a SecurityScheme for Basic auth.
func basicScheme(envVar string) model.SecurityScheme {
	return model.SecurityScheme{
		Type:   model.SecuritySchemeBasic,
		EnvVar: envVar,
	}
}

// --- Bearer token auth (existing behavior preserved) ---

// TestGenerateClient_BearerAuth_ExistingBehaviorPreserved verifies that a
// Bearer-only spec still produces a client that injects Authorization: Bearer.
func TestGenerateClient_BearerAuth_ExistingBehaviorPreserved(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"bearerAuth": bearerScheme("MYAPI_TOKEN"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	assert.Contains(t, src, "Authorization", "should set Authorization header")
	assert.Contains(t, src, "Bearer", "should use Bearer scheme for bearer auth")
}

// TestGenerateClient_BearerAuth_ValidGoSyntax verifies the generated source parses.
func TestGenerateClient_BearerAuth_ValidGoSyntax(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"bearerAuth": bearerScheme("MYAPI_TOKEN"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "client.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "bearer auth generated client should be valid Go:\n%s", src)
}

// --- API key auth ---

// TestGenerateClient_APIKeyAuth_SetsCustomHeader verifies that an API key spec
// produces a client that injects the custom header defined in HeaderName.
func TestGenerateClient_APIKeyAuth_SetsCustomHeader(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"apiKeyAuth": apiKeyScheme("X-API-Key", "MYAPI_API_KEY"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	assert.Contains(t, src, "X-API-Key",
		"generated client should set the custom header name from the API key scheme")
}

// TestGenerateClient_APIKeyAuth_UsesEnvVar verifies that the generated client
// sources the API key from the env var specified in the scheme.
func TestGenerateClient_APIKeyAuth_UsesEnvVar(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"apiKeyAuth": apiKeyScheme("X-API-Key", "MYAPI_API_KEY"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	assert.Contains(t, src, "MYAPI_API_KEY",
		"generated client should reference the API key env var")
}

// TestGenerateClient_APIKeyAuth_ValidGoSyntax verifies the generated source parses.
func TestGenerateClient_APIKeyAuth_ValidGoSyntax(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"apiKeyAuth": apiKeyScheme("X-API-Key", "MYAPI_API_KEY"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "client.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "apikey auth generated client should be valid Go:\n%s", src)
}

// TestGenerateClient_APIKeyAuth_DifferentHeaderName verifies that the actual
// HeaderName value (not a hardcoded string) is embedded.
func TestGenerateClient_APIKeyAuth_DifferentHeaderName(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"apiKeyAuth": apiKeyScheme("Authorization-Token", "MYAPI_API_KEY"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	assert.Contains(t, src, "Authorization-Token",
		"generated client should embed the exact header name from the spec, not a hardcoded default")
}

// --- Basic auth ---

// TestGenerateClient_BasicAuth_SetsAuthorizationHeader verifies that a Basic
// auth spec produces a client that sets the Authorization header.
func TestGenerateClient_BasicAuth_SetsAuthorizationHeader(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"basicAuth": basicScheme("MYAPI_TOKEN"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	assert.Contains(t, src, "Authorization",
		"generated client should set the Authorization header for Basic auth")
}

// TestGenerateClient_BasicAuth_UsesBase64Encoding verifies that the generated
// client uses base64 encoding for Basic auth credentials.
func TestGenerateClient_BasicAuth_UsesBase64Encoding(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"basicAuth": basicScheme("MYAPI_TOKEN"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	hasBase64 := strings.Contains(src, "base64") ||
		strings.Contains(src, "encoding/base64") ||
		strings.Contains(src, "StdEncoding") ||
		strings.Contains(src, "RawStdEncoding")
	assert.True(t, hasBase64,
		"generated client should use base64 encoding for Basic auth credentials")
}

// TestGenerateClient_BasicAuth_ContainsBasicKeyword verifies that the generated
// client explicitly references "Basic" in the Authorization header.
func TestGenerateClient_BasicAuth_ContainsBasicKeyword(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"basicAuth": basicScheme("MYAPI_TOKEN"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	assert.Contains(t, src, "Basic",
		"generated client should reference 'Basic' in the Authorization header for Basic auth")
}

// TestGenerateClient_BasicAuth_ValidGoSyntax verifies the generated source parses.
func TestGenerateClient_BasicAuth_ValidGoSyntax(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"basicAuth": basicScheme("MYAPI_TOKEN"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "client.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "basic auth generated client should be valid Go:\n%s", src)
}

// --- Multiple schemes ---

// TestGenerateClient_MultipleSchemes_AllPresent verifies that when a spec has
// both Bearer and API key schemes, the generated client handles both.
func TestGenerateClient_MultipleSchemes_AllPresent(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"bearerAuth": bearerScheme("MYAPI_TOKEN"),
		"apiKeyAuth": apiKeyScheme("X-API-Key", "MYAPI_API_KEY"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	// Both auth mechanisms should be represented.
	assert.Contains(t, src, "Bearer", "should support Bearer auth")
	assert.Contains(t, src, "X-API-Key", "should support API key auth")
}

// TestGenerateClient_MultipleSchemes_ValidGoSyntax verifies mixed-scheme client parses.
func TestGenerateClient_MultipleSchemes_ValidGoSyntax(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"bearerAuth": bearerScheme("MYAPI_TOKEN"),
		"apiKeyAuth": apiKeyScheme("X-API-Key", "MYAPI_API_KEY"),
		"basicAuth":  basicScheme("MYAPI_TOKEN"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "client.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "multi-scheme generated client should be valid Go:\n%s", src)
}

// --- No schemes (backward compat) ---

// TestGenerateClient_NoSchemes_StillValid verifies that a spec with no
// security schemes produces a valid, compilable client.
func TestGenerateClient_NoSchemes_StillValid(t *testing.T) {
	spec := specWithSchemes(nil)

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "client.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "no-schemes generated client should be valid Go:\n%s", src)
}

// TestGenerateClient_NoSchemes_NoRegressionOnExisting verifies that a spec
// with no schemes still contains the core Do method and NewClient.
func TestGenerateClient_NoSchemes_NoRegressionOnExisting(t *testing.T) {
	spec := specWithSchemes(nil)

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	assert.Contains(t, src, "func (c *Client) Do(", "should still have Do method")
	assert.Contains(t, src, "NewClient", "should still have NewClient constructor")
}

// --- Config env var documentation ---

// TestGenerateConfig_DocumentsAPIKeyEnvVar verifies that GenerateConfig produces
// config that documents the API key env var, not just _TOKEN.
func TestGenerateConfig_DocumentsAPIKeyEnvVar(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"apiKeyAuth": apiKeyScheme("X-API-Key", "MYAPI_API_KEY"),
	})

	src, err := generator.GenerateConfig(spec, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, "MYAPI_API_KEY",
		"config.go should document the API key env var for API key auth specs")
}

// TestGenerateConfig_DocumentsBearerEnvVar verifies that GenerateConfig
// documents the token env var for Bearer auth specs.
func TestGenerateConfig_DocumentsBearerEnvVar(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"bearerAuth": bearerScheme("MYAPI_TOKEN"),
	})

	src, err := generator.GenerateConfig(spec, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, "MYAPI_TOKEN",
		"config.go should document the token env var for Bearer auth specs")
}

// TestGenerateConfig_DocumentsAllSchemesEnvVars verifies that GenerateConfig
// documents env vars for all detected schemes, not just the first one.
func TestGenerateConfig_DocumentsAllSchemesEnvVars(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"bearerAuth": bearerScheme("MYAPI_TOKEN"),
		"apiKeyAuth": apiKeyScheme("X-Custom-Key", "MYAPI_API_KEY"),
	})

	src, err := generator.GenerateConfig(spec, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, "MYAPI_TOKEN",
		"config.go should document bearer token env var")
	assert.Contains(t, src, "MYAPI_API_KEY",
		"config.go should document API key env var")
}

// --- Amy's compile bug findings ---

// TestGenerateClient_DuplicateAPIKeyEnvVar verifies that two API key schemes
// sharing the same EnvVar (but with different HeaderNames) do NOT produce
// duplicate variable declarations in the generated client, which causes a
// compile error ("var myapiApiKey redeclared in this block").
//
// FAILS until GenerateClient deduplicates env-var-derived variable names before
// emitting var decls (e.g. deduplicate by EnvVar → Go identifier mapping).
func TestGenerateClient_DuplicateAPIKeyEnvVar(t *testing.T) {
	// Two schemes share the same EnvVar but have different HeaderNames.
	// The current impl derives the Go var name from EnvVar, so both schemes
	// produce `var myapiApiKey = ""` — a redeclaration compile error.
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"apiKey1": apiKeyScheme("X-API-Key", "MYAPI_API_KEY"),
		"apiKey2": apiKeyScheme("X-Alt-Key", "MYAPI_API_KEY"), // same EnvVar, different header
	})

	src, err := generator.GenerateClient(spec)
	// GenerateClient may either return an error (acceptable) or produce valid Go.
	// What it must NOT do is silently emit duplicate `var myapiApiKey` declarations.
	if err != nil {
		return
	}

	// go/parser does not catch redeclarations (that's a type-checker error), so
	// we count occurrences of the generated var name directly. With two schemes
	// sharing the same EnvVar "MYAPI_API_KEY", the impl derives the identifier
	// "myapiApiKey" for both — it must appear as a var declaration at most once.
	varDeclCount := strings.Count(src, "var myapiApiKey")
	assert.LessOrEqual(t, varDeclCount, 1,
		"two API key schemes with the same EnvVar must not produce duplicate var declarations "+
			"(found %d occurrences of 'var myapiApiKey'):\n%s", varDeclCount, src)
}

// TestGenerateClient_EmptyEnvVar verifies that an API key scheme with an empty
// EnvVar does not produce invalid Go syntax (e.g. `var  = ""`).
//
// FAILS until GenerateClient validates or skips schemes with empty EnvVar.
func TestGenerateClient_EmptyEnvVar(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"apiKeyAuth": apiKeyScheme("X-API-Key", ""), // EnvVar intentionally empty
	})

	src, err := generator.GenerateClient(spec)
	// Acceptable outcomes: return an error, OR produce valid Go.
	// Unacceptable: silently produce unparseable Go like `var  = ""`.
	if err != nil {
		// Error return is fine — caller can report it cleanly.
		return
	}

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "client.go", src, parser.AllErrors)
	assert.NoError(t, parseErr,
		"API key scheme with empty EnvVar must not produce invalid Go (e.g. `var  = \"\"`):\n%s", src)
}

// TestGenerateClient_BearerAndBasicPrecedence verifies that when both Bearer
// and Basic schemes are present, the generated Do() sets the Authorization
// header exactly once (first-non-empty credential wins), not twice.
//
// FAILS until GenerateClient emits a precedence chain (if/else if) instead of
// two independent if blocks that overwrite each other.
func TestGenerateClient_BearerAndBasicPrecedence(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"bearerAuth": bearerScheme("MYAPI_TOKEN"),
		"basicAuth":  basicScheme("MYAPI_TOKEN"),
	})

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	// Count the number of times Authorization is SET (not just referenced).
	// A correct implementation uses if/else if so only one branch executes;
	// a buggy one has two independent ifs that both call req.Header.Set("Authorization",...).
	setAuthCount := strings.Count(src, `Set("Authorization"`)
	assert.LessOrEqual(t, setAuthCount, 1,
		"Do() should set Authorization header at most once (if/else if chain), "+
			"but found %d Set(\"Authorization\",...) calls — Basic overwrites Bearer:\n%s",
		setAuthCount, src)
}

// TestGenerateClient_EmptyHeaderName verifies that an API key scheme with an
// empty HeaderName does not emit req.Header.Set("", apiKey) in the generated client.
//
// FAILS until GenerateClient skips or errors on schemes with empty HeaderName.
func TestGenerateClient_EmptyHeaderName(t *testing.T) {
	spec := specWithSchemes(map[string]model.SecurityScheme{
		"apiKeyAuth": apiKeyScheme("", "MYAPI_API_KEY"), // HeaderName intentionally empty
	})

	src, err := generator.GenerateClient(spec)
	// Acceptable: return an error, or skip the scheme, or use a safe default header.
	// Unacceptable: emit Set("", apiKey) which is a silent no-op at best.
	if err != nil {
		return
	}

	assert.NotContains(t, src, `Set("",`,
		"generated client must not call req.Header.Set(\"\", ...) for an API key scheme with empty HeaderName:\n%s", src)
}
