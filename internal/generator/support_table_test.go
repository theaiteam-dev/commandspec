// Package generator_test contains tests for table output formatting in generated CLIs.
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

// minimalSpecForTable returns a minimal APISpec suitable for generator tests.
func minimalSpecForTable() *model.APISpec {
	return &model.APISpec{
		Title:   "Table Test API",
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

// TestGenerateOutput_PrintTableFunctionPresent verifies that the generated output.go
// template contains a PrintTable function.
// FAILS until support.go outputTemplate is updated with PrintTable.
func TestGenerateOutput_PrintTableFunctionPresent(t *testing.T) {
	src, err := generator.GenerateOutput()
	require.NoError(t, err)
	assert.Contains(t, src, "PrintTable",
		"generated output.go should contain a PrintTable function")
}

// TestGenerateOutput_PrintTableSignature verifies that PrintTable accepts
// data []byte and noColor bool parameters.
// FAILS until support.go outputTemplate is updated with PrintTable.
func TestGenerateOutput_PrintTableSignature(t *testing.T) {
	src, err := generator.GenerateOutput()
	require.NoError(t, err)
	assert.True(t,
		strings.Contains(src, "PrintTable(data []byte, noColor bool") ||
			strings.Contains(src, "PrintTable(data []byte,"),
		"PrintTable should accept (data []byte, noColor bool) parameters")
}

// TestGenerateOutput_TableArrayRendering verifies that the generated output.go
// contains logic to render JSON arrays as tabular output.
// FAILS until support.go outputTemplate is updated with array-handling logic.
func TestGenerateOutput_TableArrayRendering(t *testing.T) {
	src, err := generator.GenerateOutput()
	require.NoError(t, err)
	hasArrayHandling := strings.Contains(src, "[]interface{}") ||
		strings.Contains(src, "[]map[string]") ||
		strings.Contains(src, "[0] == '['") ||
		strings.Contains(src, `startsWith("[")`)
	assert.True(t, hasArrayHandling,
		"generated output.go should contain logic to detect and render JSON arrays as tables")
}

// TestGenerateOutput_TableObjectRendering verifies that the generated output.go
// handles top-level JSON objects as key-value pairs.
// FAILS until support.go outputTemplate is updated with object-handling logic.
func TestGenerateOutput_TableObjectRendering(t *testing.T) {
	src, err := generator.GenerateOutput()
	require.NoError(t, err)
	hasObjectHandling := strings.Contains(src, "map[string]interface{}") ||
		(strings.Contains(src, "Key") && strings.Contains(src, "Value"))
	assert.True(t, hasObjectHandling,
		"generated output.go should handle JSON objects as key-value table rows")
}

// TestGenerateOutput_TablewriterDependency verifies that the generated output.go
// imports or references tablewriter.
// FAILS until support.go outputTemplate references olekukonko/tablewriter.
func TestGenerateOutput_TablewriterDependency(t *testing.T) {
	src, err := generator.GenerateOutput()
	require.NoError(t, err)
	assert.Contains(t, src, "tablewriter",
		"generated output.go should import or reference tablewriter for column formatting")
}

// TestGenerateOutput_NoColorRespected verifies that the generated output.go
// contains logic to disable ANSI formatting when noColor is true.
// FAILS until support.go outputTemplate includes noColor-aware logic.
func TestGenerateOutput_NoColorRespected(t *testing.T) {
	src, err := generator.GenerateOutput()
	require.NoError(t, err)
	assert.Contains(t, src, "noColor",
		"generated output.go should reference noColor parameter to disable ANSI output")
}

// TestGenerateOutput_ValidGoSyntaxWithTable verifies that the updated output.go
// (with PrintTable) is still valid Go syntax.
// FAILS until support.go outputTemplate is updated.
func TestGenerateOutput_ValidGoSyntaxWithTable(t *testing.T) {
	src, err := generator.GenerateOutput()
	require.NoError(t, err)
	// Only validate syntax if PrintTable is present (i.e., after implementation)
	if strings.Contains(src, "PrintTable") {
		mustParseGoSrc(t, "output.go", src)
	} else {
		t.Skip("PrintTable not yet implemented; skipping syntax check")
	}
}

// TestGenerateGoMod_ContainsTablewriter verifies that the generated project's
// go.mod includes the tablewriter dependency.
// FAILS until generator.go writeGoMod() adds tablewriter.
func TestGenerateGoMod_ContainsTablewriter(t *testing.T) {
	spec := minimalSpecForTable()
	outputDir := t.TempDir()
	err := generator.Generate(spec, "tabletest", outputDir)
	require.NoError(t, err, "Generate should succeed")

	goModPath := filepath.Join(outputDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	require.NoError(t, err, "go.mod should exist")

	assert.Contains(t, string(data), "tablewriter",
		"generated go.mod should include olekukonko/tablewriter dependency")
}
