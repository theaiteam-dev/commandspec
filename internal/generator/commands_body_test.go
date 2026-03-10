// Package generator_test contains tests for --body and --body-file flags
// in generated write operation commands.
package generator_test

import (
	"strings"
	"testing"

	"github.com/queso/swagger-jack/internal/generator"
	"github.com/queso/swagger-jack/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeWriteCmd returns a model.Command for a write operation with the given HTTP method.
func makeWriteCmd(method string) model.Command {
	return model.Command{
		Name:       strings.ToLower(method),
		HTTPMethod: method,
		Path:       "/items",
		Flags: []model.Flag{
			{Name: "name", Type: model.FlagTypeString, Required: true, Source: model.FlagSourceBody},
		},
	}
}

// makeWriteResource wraps a command in a minimal resource.
func makeWriteResource(cmd model.Command) model.Resource {
	return model.Resource{
		Name:     "items",
		Commands: []model.Command{cmd},
	}
}

// TestBodyFlagPresentForPost verifies that POST commands get a --body flag.
// FAILS until commands.go GenerateVerbCmd adds --body for write operations.
func TestBodyFlagPresentForPost(t *testing.T) {
	cmd := makeWriteCmd("POST")
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)
	assert.Contains(t, src, `"body"`,
		"POST command should register a --body flag")
	assert.Contains(t, src, "--body",
		"POST command source should reference --body flag usage")
}

// TestBodyFlagPresentForPut verifies that PUT commands get a --body flag.
// FAILS until commands.go GenerateVerbCmd adds --body for write operations.
func TestBodyFlagPresentForPut(t *testing.T) {
	cmd := makeWriteCmd("PUT")
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)
	assert.Contains(t, src, `"body"`,
		"PUT command should register a --body flag")
}

// TestBodyFlagPresentForPatch verifies that PATCH commands get a --body flag.
// FAILS until commands.go GenerateVerbCmd adds --body for write operations.
func TestBodyFlagPresentForPatch(t *testing.T) {
	cmd := makeWriteCmd("PATCH")
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)
	assert.Contains(t, src, `"body"`,
		"PATCH command should register a --body flag")
}

// TestBodyFileFlagPresentForPost verifies that POST commands get a --body-file flag.
// FAILS until commands.go GenerateVerbCmd adds --body-file for write operations.
func TestBodyFileFlagPresentForPost(t *testing.T) {
	cmd := makeWriteCmd("POST")
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)
	assert.Contains(t, src, "body-file",
		"POST command should register a --body-file flag")
}

// TestBodyFileFlagPresentForPut verifies that PUT commands get a --body-file flag.
// FAILS until commands.go GenerateVerbCmd adds --body-file for write operations.
func TestBodyFileFlagPresentForPut(t *testing.T) {
	cmd := makeWriteCmd("PUT")
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)
	assert.Contains(t, src, "body-file",
		"PUT command should register a --body-file flag")
}

// TestBodyFlagAbsentForGet verifies that GET commands do NOT get --body or --body-file flags.
func TestBodyFlagAbsentForGet(t *testing.T) {
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
	}
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// Ensure --body is not added for GET. Allow "body" only if it is a substring
	// of something else (like "response body" comments), but not as a flag registration.
	// We check for the flag registration pattern.
	assert.NotContains(t, src, `StringVar(&body,`,
		"GET command should NOT register a --body flag variable")
	assert.NotContains(t, src, `StringVar(&bodyFile,`,
		"GET command should NOT register a --body-file flag variable")
}

// TestBodyFlagAbsentForDelete verifies that DELETE commands do NOT get --body or --body-file flags.
func TestBodyFlagAbsentForDelete(t *testing.T) {
	cmd := model.Command{
		Name:       "delete",
		HTTPMethod: "DELETE",
		Path:       "/items/{id}",
		Args: []model.Arg{
			{Name: "id", Required: true},
		},
	}
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)
	assert.NotContains(t, src, `StringVar(&body,`,
		"DELETE command should NOT register a --body flag variable")
	assert.NotContains(t, src, `StringVar(&bodyFile,`,
		"DELETE command should NOT register a --body-file flag variable")
}

// TestBodyFlagOverridesIndividualFlags verifies that the generated RunE contains
// logic to check --body before building from individual flags.
// FAILS until commands.go buildRunEBody adds --body override logic.
func TestBodyFlagOverridesIndividualFlags(t *testing.T) {
	cmd := makeWriteCmd("POST")
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// The generated code should check body != "" before using individual flags
	hasBodyCheck := strings.Contains(src, `body != ""`) ||
		strings.Contains(src, "if body")
	assert.True(t, hasBodyCheck,
		"POST RunE should check --body flag before using individual body flags")
}

// TestBodyFileReadLogic verifies that the generated RunE contains
// logic to read --body-file contents.
// FAILS until commands.go buildRunEBody adds --body-file read logic.
func TestBodyFileReadLogic(t *testing.T) {
	cmd := makeWriteCmd("POST")
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	hasFileRead := strings.Contains(src, "ReadFile") ||
		strings.Contains(src, "os.ReadFile") ||
		strings.Contains(src, "ioutil.ReadFile") ||
		strings.Contains(src, "bodyFile")
	assert.True(t, hasFileRead,
		"POST RunE should contain file-reading logic for --body-file flag")
}

// TestBodyJSONValidation verifies that the generated RunE validates the --body JSON.
// FAILS until commands.go buildRunEBody validates JSON input.
func TestBodyJSONValidation(t *testing.T) {
	cmd := makeWriteCmd("POST")
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	hasJSONValidation := strings.Contains(src, "json.Valid") ||
		strings.Contains(src, "json.Unmarshal") ||
		strings.Contains(src, "valid JSON")
	assert.True(t, hasJSONValidation,
		"POST RunE should validate that --body contains valid JSON")
}

// TestBodyFlagGeneratedGoSyntax verifies that the generated POST command
// with --body and --body-file is valid Go syntax.
// FAILS until the implementation is correct.
func TestBodyFlagGeneratedGoSyntax(t *testing.T) {
	cmd := makeWriteCmd("POST")
	resource := makeWriteResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)
	// Only validate syntax if body flag is present (post-implementation)
	if strings.Contains(src, "body-file") {
		mustParseGoSrc(t, "items_post.go", src)
	} else {
		t.Skip("--body-file not yet implemented; skipping syntax check")
	}
}
