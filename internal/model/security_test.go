package model_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/queso/swagger-jack/internal/model"
	"github.com/queso/swagger-jack/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// securityFixtureDir returns the absolute path to testdata from this file's location.
func securityFixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "testdata")
}

func loadSecurityFixture(t *testing.T, name string) *parser.Result {
	t.Helper()
	result, err := parser.Load(filepath.Join(securityFixtureDir(), name))
	require.NoError(t, err, "fixture %s should load without error", name)
	return result
}

// TestExtractBearerToken verifies that an http+bearer security scheme produces
// a SecurityScheme with Type "bearer" and an env var derived from the CLI name.
func TestExtractBearerToken(t *testing.T) {
	result := loadSecurityFixture(t, "petstore.json")
	schemes, err := model.ExtractSecuritySchemes(result, "mycli")
	require.NoError(t, err)

	scheme, ok := schemes["bearerAuth"]
	require.True(t, ok, "expected 'bearerAuth' scheme")
	assert.Equal(t, model.SecuritySchemeBearer, scheme.Type)
	assert.Equal(t, "MYCLI_TOKEN", scheme.EnvVar)
}

// TestExtractAPIKey verifies that an apiKey-in-header scheme produces a
// SecurityScheme with Type "apikey", the correct HeaderName, and an env var.
func TestExtractAPIKey(t *testing.T) {
	result := loadSecurityFixture(t, "apikey_auth.json")
	schemes, err := model.ExtractSecuritySchemes(result, "mycli")
	require.NoError(t, err)

	scheme, ok := schemes["apiKeyAuth"]
	require.True(t, ok, "expected 'apiKeyAuth' scheme")
	assert.Equal(t, model.SecuritySchemeAPIKey, scheme.Type)
	assert.Equal(t, "X-API-Key", scheme.HeaderName)
	assert.Equal(t, "MYCLI_API_KEY", scheme.EnvVar)
}

// TestExtractBasicAuth verifies that an http+basic scheme produces a
// SecurityScheme with Type "basic" and an env var.
func TestExtractBasicAuth(t *testing.T) {
	result := loadSecurityFixture(t, "basic_auth.json")
	schemes, err := model.ExtractSecuritySchemes(result, "mycli")
	require.NoError(t, err)

	scheme, ok := schemes["basicAuth"]
	require.True(t, ok, "expected 'basicAuth' scheme")
	assert.Equal(t, model.SecuritySchemeBasic, scheme.Type)
	assert.Equal(t, "MYCLI_TOKEN", scheme.EnvVar)
}

// TestExtractNoSecuritySection verifies that a spec with no securitySchemes
// returns an empty map and no error.
func TestExtractNoSecuritySection(t *testing.T) {
	result := loadSecurityFixture(t, "minimal.json")
	schemes, err := model.ExtractSecuritySchemes(result, "mycli")
	require.NoError(t, err)
	assert.Empty(t, schemes, "expected empty schemes map for spec with no security")
}

// TestExtractEnvVarNaming verifies that the CLI name is uppercased and
// dashes are converted to underscores when forming the env var name.
func TestExtractEnvVarNaming(t *testing.T) {
	result := loadSecurityFixture(t, "petstore.json")
	schemes, err := model.ExtractSecuritySchemes(result, "petstore-api")
	require.NoError(t, err)

	scheme, ok := schemes["bearerAuth"]
	require.True(t, ok)
	// "petstore-api" → "PETSTORE_API_TOKEN"
	assert.Equal(t, "PETSTORE_API_TOKEN", scheme.EnvVar)
}
