package parser_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testdataPath returns the absolute path to a testdata fixture file.
// Uses the source file location so tests work regardless of working directory.
func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "testdata", name)
}

// TestLoadMinimalSpec verifies that a minimal valid OpenAPI spec loads without
// error and returns a non-nil Result with a populated Spec.
func TestLoadMinimalSpec(t *testing.T) {
	result, err := parser.Load(testdataPath("minimal.json"))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Spec)
	assert.Equal(t, "Minimal API", result.Spec.Title)
	assert.Equal(t, "1.0.0", result.Spec.Version)
}

// TestLoadPetstoreRefsResolved verifies that $ref references in petstore.json
// are resolved and the spec is fully parsed. The loader must inline $refs so
// that schema data from components is accessible on the returned APISpec.
func TestLoadPetstoreRefsResolved(t *testing.T) {
	result, err := parser.Load(testdataPath("petstore.json"))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Spec)

	assert.Equal(t, "Petstore", result.Spec.Title)
	// Spec should have resources parsed from paths (/pets, /owners)
	assert.NotEmpty(t, result.Spec.Resources, "expected resources parsed from petstore paths")
	// Bearer auth security scheme should be present
	scheme, ok := result.Spec.SecuritySchemes["bearerAuth"]
	require.True(t, ok, "expected bearerAuth security scheme")
	assert.Equal(t, "bearer", string(scheme.Type))
}

// TestLoadMissingFile verifies that loading a non-existent file returns an
// error rather than panicking or returning nil.
func TestLoadMissingFile(t *testing.T) {
	result, err := parser.Load("/nonexistent/path/that/does/not/exist.json")
	assert.Error(t, err, "expected error for missing file")
	assert.Nil(t, result)
}

// TestLoadMalformedJSON verifies that loading invalid JSON returns an error.
func TestLoadMalformedJSON(t *testing.T) {
	result, err := parser.Load(testdataPath("invalid.json"))
	assert.Error(t, err, "expected error for malformed JSON")
	assert.Nil(t, result)
}

// TestLoadRawJSONPreserved verifies that result.RawJSON contains the raw bytes
// of the original spec file.
func TestLoadRawJSONPreserved(t *testing.T) {
	result, err := parser.Load(testdataPath("minimal.json"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.RawJSON, "RawJSON should contain the original spec bytes")
	assert.Contains(t, string(result.RawJSON), "Minimal API")
}
