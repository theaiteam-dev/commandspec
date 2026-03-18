package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateCommand_ValidSpec runs validate against the petstore spec and verifies
// the success output contains the spec title and resource count, and exits 0.
func TestValidateCommand_ValidSpec(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"validate", "--schema", "../../testdata/petstore.json"})

	err := root.Execute()
	output := buf.String()

	require.NoError(t, err, "validate should exit 0 for a valid spec; output: %s", output)
	assert.Contains(t, output, "Petstore", "output should include the spec title")
	// petstore has 2 resources (pets, owners)
	assert.True(t,
		strings.Contains(output, "2 resource") || strings.Contains(output, "resources: 2"),
		"output should report resource count, got: %s", output,
	)
}

// TestValidateCommand_InvalidSpec runs validate against truncated/invalid JSON and
// expects a non-zero exit (returned error) or error text in output.
func TestValidateCommand_InvalidSpec(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"validate", "--schema", "../../testdata/invalid.json"})

	err := root.Execute()
	output := buf.String()

	// Either the command returns an error OR it prints an error message.
	hasError := err != nil ||
		strings.Contains(strings.ToLower(output), "error") ||
		strings.Contains(strings.ToLower(output), "invalid") ||
		strings.Contains(strings.ToLower(output), "failed")
	assert.True(t, hasError,
		"validate should report failure for invalid/truncated JSON; err=%v output=%s", err, output)
}

// TestValidateCommand_MissingFile runs validate with a schema path that does not exist
// and expects an error indicating the file could not be opened.
func TestValidateCommand_MissingFile(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"validate", "--schema", "nonexistent.json"})

	err := root.Execute()
	output := buf.String()

	assert.True(t,
		err != nil ||
			strings.Contains(strings.ToLower(output), "no such file") ||
			strings.Contains(strings.ToLower(output), "not found") ||
			strings.Contains(strings.ToLower(output), "error") ||
			strings.Contains(strings.ToLower(output), "open"),
		"validate should report a missing-file error; err=%v output=%s", err, output,
	)
}

// TestValidateCommand_OutputContainsVersion verifies that on a successful validation
// the output includes the version string from the spec's info.version field.
func TestValidateCommand_OutputContainsVersion(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"validate", "--schema", "../../testdata/petstore.json"})

	err := root.Execute()
	output := buf.String()

	require.NoError(t, err, "validate should exit 0 for a valid spec; output: %s", output)
	// petstore info.version is "1.0.0"
	assert.Contains(t, output, "1.0.0", "output should include the spec version from info.version")
}
