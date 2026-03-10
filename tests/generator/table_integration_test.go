// Package generator_test contains integration tests for table output generation.
package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queso/swagger-jack/internal/generator"
	"github.com/queso/swagger-jack/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tableSpec returns a minimal APISpec for table output integration tests.
func tableSpec() *model.APISpec {
	return &model.APISpec{
		Title:   "Table Integration API",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
		Resources: []model.Resource{
			{
				Name: "items",
				Commands: []model.Command{
					{
						Name:       "list",
						HTTPMethod: "GET",
						Path:       "/items",
					},
				},
			},
		},
	}
}

// TestTableIntegration_OutputGoHasPrintTable verifies that the generated
// internal/output.go contains a PrintTable function.
func TestTableIntegration_OutputGoHasPrintTable(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(tableSpec(), "tableapi", dir)
	require.NoError(t, err, "Generate should succeed")

	data, err := os.ReadFile(filepath.Join(dir, "internal", "output", "output.go"))
	require.NoError(t, err, "internal/output.go should exist")

	src := string(data)
	assert.Contains(t, src, "PrintTable",
		"generated output.go should contain PrintTable function")
}

// TestTableIntegration_OutputGoHasTablewriter verifies that the generated
// internal/output.go imports tablewriter.
func TestTableIntegration_OutputGoHasTablewriter(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(tableSpec(), "tableapi", dir)
	require.NoError(t, err, "Generate should succeed")

	data, err := os.ReadFile(filepath.Join(dir, "internal", "output", "output.go"))
	require.NoError(t, err, "internal/output.go should exist")

	assert.Contains(t, string(data), "tablewriter",
		"generated output.go should import tablewriter")
}

// TestTableIntegration_OutputGoNoColorParam verifies the noColor parameter presence.
func TestTableIntegration_OutputGoNoColorParam(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(tableSpec(), "tableapi", dir)
	require.NoError(t, err, "Generate should succeed")

	data, err := os.ReadFile(filepath.Join(dir, "internal", "output", "output.go"))
	require.NoError(t, err)

	assert.Contains(t, string(data), "noColor",
		"generated output.go should reference noColor parameter")
}

// TestTableIntegration_GoModHasTablewriter verifies the generated go.mod
// declares the tablewriter dependency.
func TestTableIntegration_GoModHasTablewriter(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(tableSpec(), "tableapi", dir)
	require.NoError(t, err, "Generate should succeed")

	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	require.NoError(t, err, "go.mod should exist")

	assert.Contains(t, string(data), "tablewriter",
		"generated go.mod should include tablewriter dependency")
}

// TestTableIntegration_OutputGoArrayHandling verifies that the generated
// output.go contains logic to detect JSON arrays.
func TestTableIntegration_OutputGoArrayHandling(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(tableSpec(), "tableapi", dir)
	require.NoError(t, err, "Generate should succeed")

	data, err := os.ReadFile(filepath.Join(dir, "internal", "output", "output.go"))
	require.NoError(t, err)

	src := string(data)
	hasArrayHandling := strings.Contains(src, "[]interface{}") ||
		strings.Contains(src, "[0] == '['")
	assert.True(t, hasArrayHandling,
		"generated output.go should contain logic to render JSON arrays as tables")
}

// TestTableIntegration_OutputGoObjectHandling verifies that generated
// output.go handles JSON objects as key-value pairs.
func TestTableIntegration_OutputGoObjectHandling(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(tableSpec(), "tableapi", dir)
	require.NoError(t, err, "Generate should succeed")

	data, err := os.ReadFile(filepath.Join(dir, "internal", "output", "output.go"))
	require.NoError(t, err)

	src := string(data)
	hasObjectHandling := strings.Contains(src, "map[string]interface{}") ||
		(strings.Contains(src, "Key") && strings.Contains(src, "Value"))
	assert.True(t, hasObjectHandling,
		"generated output.go should handle JSON objects as key-value pairs")
}
