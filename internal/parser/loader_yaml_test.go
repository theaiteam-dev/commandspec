// Package parser_test contains tests for YAML spec loading in the parser.
package parser_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// yamlFixtureDir returns the absolute path to the top-level testdata directory.
func yamlFixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "testdata")
}

const minimalYAML = `openapi: "3.0.3"
info:
  title: Minimal YAML API
  version: "1.0.0"
paths:
  /items:
    get:
      summary: List all items
      responses:
        "200":
          description: A list of items
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
`

const yamlWithRefs = `openapi: "3.0.3"
info:
  title: YAML Refs API
  version: "1.0.0"
components:
  schemas:
    Item:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
paths:
  /items:
    get:
      summary: List items
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Item"
`

const invalidYAML = `openapi: "3.0.3"
info:
  title: [broken
  key: : : :
`

// writeTempYAML writes content to a temp file with the given extension and returns its path.
func writeTempYAML(t *testing.T, content, ext string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "spec*"+ext)
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// TestLoadYAMLDotYaml verifies that parser.Load accepts a .yaml file.
func TestLoadYAMLDotYaml(t *testing.T) {
	path := writeTempYAML(t, minimalYAML, ".yaml")
	result, err := parser.Load(path)
	require.NoError(t, err, "parser.Load should succeed for a .yaml file")
	require.NotNil(t, result)
	require.NotNil(t, result.Spec)
	assert.Equal(t, "Minimal YAML API", result.Spec.Title)
}

// TestLoadYAMLDotYml verifies that parser.Load accepts a .yml file.
func TestLoadYAMLDotYml(t *testing.T) {
	path := writeTempYAML(t, minimalYAML, ".yml")
	result, err := parser.Load(path)
	require.NoError(t, err, "parser.Load should succeed for a .yml file")
	require.NotNil(t, result)
	require.NotNil(t, result.Spec)
	assert.Equal(t, "Minimal YAML API", result.Spec.Title)
}

// TestLoadJSONStillWorks verifies that JSON loading is not broken by YAML support.
func TestLoadYAMLJSONStillWorks(t *testing.T) {
	path := filepath.Join(yamlFixtureDir(), "minimal.json")
	result, err := parser.Load(path)
	require.NoError(t, err, "parser.Load should still work for .json files")
	require.NotNil(t, result)
	require.NotNil(t, result.Spec)
	assert.Equal(t, "Minimal API", result.Spec.Title)
}

// TestLoadYAMLWithRefs verifies that $ref references are resolved in YAML specs.
func TestLoadYAMLWithRefs(t *testing.T) {
	path := writeTempYAML(t, yamlWithRefs, ".yaml")
	result, err := parser.Load(path)
	require.NoError(t, err, "parser.Load should resolve $ref references in YAML specs")
	require.NotNil(t, result)
	require.NotNil(t, result.Spec)
	// The raw JSON bytes should not contain any $ref (all refs resolved)
	assert.NotContains(t, string(result.RawJSON), `"$ref"`,
		"raw JSON should have all $ref references resolved")
}

// TestLoadInvalidYAML verifies that malformed YAML returns a descriptive error.
func TestLoadInvalidYAML(t *testing.T) {
	path := writeTempYAML(t, invalidYAML, ".yaml")
	_, err := parser.Load(path)
	require.Error(t, err, "parser.Load should return an error for invalid YAML")
	assert.Contains(t, err.Error(), "yaml",
		"error message should mention yaml for .yaml files")
}

// TestLoadYAMLResourcesBuilt verifies that resources are parsed from a YAML spec.
func TestLoadYAMLResourcesBuilt(t *testing.T) {
	path := writeTempYAML(t, minimalYAML, ".yaml")
	result, err := parser.Load(path)
	require.NoError(t, err)
	require.NotNil(t, result.Spec)
	assert.NotEmpty(t, result.Spec.Resources,
		"YAML spec should produce at least one resource")
}
