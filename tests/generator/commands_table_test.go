// Package generator_test contains integration tests for table output wiring in
// generated RunE bodies (WI-511).
//
// These tests verify that buildRunEBody() calls PrintTable when --json is not set
// and falls back to raw JSON output when --json is set.
//
// ALL tests in this file FAIL until B.A. implements the feature in
// internal/generator/commands.go buildRunEBody().
package generator_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/queso/swagger-jack/internal/generator"
	"github.com/queso/swagger-jack/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tableCmd returns a minimal GET command for table output wiring tests.
func tableCmd() model.Command {
	return model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
	}
}

// tableResource wraps a command in a minimal resource.
func tableResource(cmd model.Command) model.Resource {
	return model.Resource{
		Name:     "items",
		Commands: []model.Command{cmd},
	}
}

// TestRunECallsPrintTableWhenNotJSONMode verifies that the generated RunE body
// calls PrintTable in the non-JSON output branch.
// FAILS until buildRunEBody() replaces the pretty-print JSON block with a
// PrintTable call.
func TestRunECallsPrintTableWhenNotJSONMode(t *testing.T) {
	cmd := tableCmd()
	resource := tableResource(cmd)

	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, "PrintTable",
		"generated RunE should call PrintTable when --json is not set; got:\n%s", src)
}

// TestRunERawJSONOutputWhenJSONMode verifies that the generated RunE still
// outputs raw JSON bytes (not a table) when --json is set.
// This existing behaviour must be preserved after the PrintTable wiring.
func TestRunERawJSONOutputWhenJSONMode(t *testing.T) {
	cmd := tableCmd()
	resource := tableResource(cmd)

	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// The jsonMode branch must exist.
	assert.Contains(t, src, "jsonMode",
		"generated RunE should read the --json flag into jsonMode; got:\n%s", src)
	assert.Contains(t, src, "if jsonMode",
		"generated RunE should branch on jsonMode; got:\n%s", src)

	// Raw output via fmt.Printf or fmt.Println must appear in the jsonMode branch.
	hasRawOutput := strings.Contains(src, `fmt.Printf("%s\n", string(resp)`) ||
		strings.Contains(src, "fmt.Println(string(resp)")
	assert.True(t, hasRawOutput,
		"generated RunE should output raw JSON bytes when --json is set; got:\n%s", src)
}

// TestRunEImportsInternalOutputPackage verifies that the generated verb command
// file imports the internal output package so PrintTable is resolvable.
// FAILS until buildImports() adds the output package import path.
func TestRunEImportsInternalOutputPackage(t *testing.T) {
	cmd := tableCmd()
	resource := tableResource(cmd)
	cliName := "myapi"

	src, err := generator.GenerateVerbCmd(resource, cmd, cliName)
	require.NoError(t, err)

	// The import block must include something like `"myapi/internal"` or
	// `"myapi/internal/output"`.
	hasOutputImport := strings.Contains(src, cliName+"/internal/output") ||
		strings.Contains(src, cliName+`/internal"`)
	assert.True(t, hasOutputImport,
		"generated verb command should import the internal output package for PrintTable; got:\n%s", src)
}

// TestRunEPrintTableReceivesRespBytes verifies that the PrintTable call passes
// the response body bytes as its first argument.
// FAILS until buildRunEBody() wires the resp bytes into PrintTable.
func TestRunEPrintTableReceivesRespBytes(t *testing.T) {
	cmd := tableCmd()
	resource := tableResource(cmd)

	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// PrintTable should be called with the response bytes variable (resp or respBody).
	hasPrintTableWithResp := strings.Contains(src, "PrintTable(resp") ||
		strings.Contains(src, "PrintTable(respBody")
	assert.True(t, hasPrintTableWithResp,
		"PrintTable call should pass the HTTP response bytes as its first argument; got:\n%s", src)
}

// TestRunEPrintTableReceivesNoColorFlag verifies that the generated RunE reads
// the --no-color persistent flag and passes it to PrintTable.
// FAILS until buildRunEBody() reads noColor and wires it into PrintTable.
func TestRunEPrintTableReceivesNoColorFlag(t *testing.T) {
	cmd := tableCmd()
	resource := tableResource(cmd)

	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, "noColor",
		"generated RunE should read the --no-color flag and pass it to PrintTable; got:\n%s", src)
}

// TestRunEPrintTableErrorFallsBackToRawOutput verifies that when PrintTable
// returns an error, the generated RunE falls back to raw output rather than
// silently swallowing the failure.
// FAILS until buildRunEBody() adds error-check + fallback after the PrintTable call.
func TestRunEPrintTableErrorFallsBackToRawOutput(t *testing.T) {
	cmd := tableCmd()
	resource := tableResource(cmd)

	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// There must be an error check on the PrintTable return value.
	hasPrintTableErrCheck := strings.Contains(src, "if err := output.PrintTable") ||
		strings.Contains(src, "if err := PrintTable") ||
		(strings.Contains(src, "PrintTable") && strings.Contains(src, "if err"))
	assert.True(t, hasPrintTableErrCheck,
		"generated RunE should check the error returned by PrintTable; got:\n%s", src)

	// A raw-output fallback (fmt.Printf or fmt.Println with resp) must exist so
	// that callers see output even when table formatting fails.
	hasFallback := strings.Contains(src, `fmt.Printf("%s\n", string(resp)`) ||
		strings.Contains(src, "fmt.Println(string(resp)")
	assert.True(t, hasFallback,
		"generated RunE should fall back to raw output when PrintTable returns an error; got:\n%s", src)
}

// TestRunEWriteOpAlsoCallsPrintTable verifies that write operations (POST) also
// call PrintTable in the non-JSON branch, not only read operations.
// FAILS until buildRunEBody() wires PrintTable for write-op output paths too.
func TestRunEWriteOpAlsoCallsPrintTable(t *testing.T) {
	cmd := model.Command{
		Name:       "create",
		HTTPMethod: "POST",
		Path:       "/items",
		Flags: []model.Flag{
			{Name: "name", Type: model.FlagTypeString, Required: true, Source: model.FlagSourceBody},
		},
	}
	resource := model.Resource{
		Name:     "items",
		Commands: []model.Command{cmd},
	}

	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, "PrintTable",
		"generated POST RunE should also call PrintTable in the non-JSON output branch; got:\n%s", src)
}

// TestRunETableOutputValidGoSyntax verifies that the updated generated verb
// command file (with PrintTable wiring) is still syntactically valid Go.
// FAILS until buildRunEBody() emits syntactically correct code with PrintTable.
func TestRunETableOutputValidGoSyntax(t *testing.T) {
	cmd := tableCmd()
	resource := tableResource(cmd)

	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	if !strings.Contains(src, "PrintTable") {
		t.Skip("PrintTable not yet implemented; skipping syntax check")
	}

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "items_list.go", src, parser.AllErrors)
	assert.NoError(t, parseErr,
		"generated Go source should parse without syntax errors:\n%s", src)
}
