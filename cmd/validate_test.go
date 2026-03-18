// Package cmd_test contains tests for the enhanced validate command.
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/theaiteam-dev/commandspec/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateFixtureDir returns the absolute path to the testdata directory.
func validateFixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(file)), "testdata")
}

// executeValidate runs `cmdspec validate --schema <path>` and returns output and error.
func executeValidate(t *testing.T, schemaPath string) (string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"validate", "--schema", schemaPath})
	err := root.Execute()
	return out.String(), err
}

// TestValidate_ValidSpecExitsZero verifies that a valid spec produces no error.
func TestValidate_ValidSpecExitsZero(t *testing.T) {
	path := filepath.Join(validateFixtureDir(), "minimal.json")
	_, err := executeValidate(t, path)
	assert.NoError(t, err, "valid spec should produce no error (exit code 0)")
}

// TestValidate_ValidSpecShowsResourceCount verifies that the output includes
// the resource count.
func TestValidate_ValidSpecShowsResourceCount(t *testing.T) {
	path := filepath.Join(validateFixtureDir(), "minimal.json")
	out, err := executeValidate(t, path)
	require.NoError(t, err)
	assert.Contains(t, out, "resource",
		"validate output should include resource count")
}

// TestValidate_ValidSpecShowsCommandCount verifies that the output includes
// the command count.
func TestValidate_ValidSpecShowsCommandCount(t *testing.T) {
	path := filepath.Join(validateFixtureDir(), "minimal.json")
	out, err := executeValidate(t, path)
	require.NoError(t, err)
	assert.Contains(t, out, "command",
		"validate output should include command count")
}

// TestValidate_ShowsAuthLineBearerToken verifies that a spec with Bearer auth
// shows an explicit "Auth:" line containing "Bearer" — not from the spec title.
// FAILS until validate command detects and prints auth schemes.
func TestValidate_ShowsAuthLineBearerToken(t *testing.T) {
	path := filepath.Join(validateFixtureDir(), "petstore.json")
	out, err := executeValidate(t, path)
	require.NoError(t, err)
	// The petstore spec title is "Petstore" — so "Bearer" can only appear from auth detection
	assert.Contains(t, out, "Auth:",
		"validate output should contain an 'Auth:' line for petstore.json with bearer auth")
	assert.Contains(t, out, "Bearer",
		"validate output should show 'Bearer' for petstore.json which has bearerAuth security scheme")
}

// TestValidate_ShowsAuthLineAPIKey verifies that a spec with API key auth
// shows an "Auth:" line containing "API key" or the header name.
// FAILS until validate command detects and prints auth schemes.
func TestValidate_ShowsAuthLineAPIKey(t *testing.T) {
	path := filepath.Join(validateFixtureDir(), "apikey_auth.json")
	out, err := executeValidate(t, path)
	require.NoError(t, err)
	// Must contain "Auth:" line (not just "Auth" from spec title)
	assert.Contains(t, out, "Auth:",
		"validate output should contain an 'Auth:' line for apikey_auth.json")
}

// TestValidate_ShowsAuthLineBasic verifies that a spec with Basic auth
// shows an "Auth:" line containing "Basic".
// FAILS until validate command detects and prints auth schemes.
func TestValidate_ShowsAuthLineBasic(t *testing.T) {
	path := filepath.Join(validateFixtureDir(), "basic_auth.json")
	out, err := executeValidate(t, path)
	require.NoError(t, err)
	assert.Contains(t, out, "Auth:",
		"validate output should contain an 'Auth:' line for basic_auth.json")
	// "Basic" in the Auth: line, not just in the title
	// We check that "Auth:" precedes "Basic" somewhere in output
	authLineIdx := bytes.Index([]byte(out), []byte("Auth:"))
	assert.NotEqual(t, -1, authLineIdx,
		"output must contain 'Auth:' line")
	if authLineIdx >= 0 {
		afterAuth := string(out[authLineIdx:])
		assert.Contains(t, afterAuth, "Basic",
			"'Auth:' line should mention 'Basic' for basic_auth.json")
	}
}

// TestValidate_ShowsNoneWhenNoAuth verifies that a spec with no security schemes
// shows "None" in the Auth line.
// FAILS until validate command detects and prints auth schemes.
func TestValidate_ShowsNoneWhenNoAuth(t *testing.T) {
	path := filepath.Join(validateFixtureDir(), "minimal.json")
	out, err := executeValidate(t, path)
	require.NoError(t, err)
	assert.Contains(t, out, "Auth:",
		"validate output should always include an 'Auth:' line")
	if bytes.Contains([]byte(out), []byte("Auth:")) {
		authIdx := bytes.Index([]byte(out), []byte("Auth:"))
		afterAuth := string(out[authIdx:])
		assert.True(t,
			bytes.Contains([]byte(afterAuth), []byte("None")) ||
				bytes.Contains([]byte(afterAuth), []byte("none")),
			"'Auth:' line should say 'None' when no security scheme is detected, got: %q", afterAuth)
	}
}

// TestValidate_InvalidSpecReturnsError verifies that an invalid spec returns
// an error (non-zero exit).
func TestValidate_InvalidSpecReturnsError(t *testing.T) {
	path := filepath.Join(validateFixtureDir(), "invalid.json")
	_, err := executeValidate(t, path)
	assert.Error(t, err, "invalid spec should return an error (exit code 1)")
}

// TestValidate_InvalidSpecPrintsErrorPrefix verifies that error messages
// are prefixed with "Error: ".
// FAILS until validate command prints per-error lines with "Error:" prefix.
func TestValidate_InvalidSpecPrintsErrorPrefix(t *testing.T) {
	path := filepath.Join(validateFixtureDir(), "invalid.json")
	out, _ := executeValidate(t, path)
	assert.Contains(t, out, "Error:",
		"invalid spec output should prefix errors with 'Error:'")
}

// TestValidate_MissingSchemaReturnsError verifies that running validate without
// --schema returns an error.
func TestValidate_MissingSchemaReturnsError(t *testing.T) {
	root := cmd.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"validate"})
	err := root.Execute()
	assert.Error(t, err, "validate without --schema should return an error")
}

// TestValidate_YAMLSpecValidated verifies that a YAML spec can be validated.
// FAILS until WI-501 (YAML parsing) support lands in validate command.
func TestValidate_YAMLSpecValidated(t *testing.T) {
	yamlContent := `openapi: "3.0.3"
info:
  title: YAML Validate Test
  version: "1.0.0"
paths:
  /items:
    get:
      summary: List items
      responses:
        "200":
          description: OK
`
	f, err := os.CreateTemp(t.TempDir(), "spec*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(yamlContent)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	out, err := executeValidate(t, f.Name())
	require.NoError(t, err, "YAML spec should validate without error")
	assert.Contains(t, out, "YAML Validate Test",
		"validate output should show the spec title from a YAML file")
}

// TestValidate_AuthLineAlwaysPresent verifies the "Auth:" line appears in output
// for all valid specs — not just those where the title happens to contain "Auth".
// FAILS until validate command adds auth detection output.
func TestValidate_AuthLineAlwaysPresent(t *testing.T) {
	// Use petstore.json whose title is "Petstore" (no "Auth" in title)
	path := filepath.Join(validateFixtureDir(), "petstore.json")
	out, err := executeValidate(t, path)
	require.NoError(t, err, "petstore.json should be valid")
	assert.Contains(t, out, "Auth:",
		"validate output for petstore.json should always include an 'Auth:' line")
}
