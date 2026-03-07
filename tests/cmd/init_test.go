package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queso/swagger-jack/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitCommand_GeneratesProject runs init against the petstore spec and verifies
// that the output directory contains the expected generated project structure.
func TestInitCommand_GeneratesProject(t *testing.T) {
	tmpDir := t.TempDir()

	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init",
		"--schema", "../../testdata/petstore.json",
		"--name", "mycli",
		"--output-dir", tmpDir,
	})

	err := root.Execute()
	output := buf.String()

	require.NoError(t, err, "init should exit 0 for a valid spec; output: %s", output)

	// Verify the generated project has expected files/directories
	assert.FileExists(t, filepath.Join(tmpDir, "main.go"), "generated project should contain main.go")
	assert.FileExists(t, filepath.Join(tmpDir, "go.mod"), "generated project should contain go.mod")
	assert.DirExists(t, filepath.Join(tmpDir, "cmd"), "generated project should contain cmd/ subdirectory")
}

// TestInitCommand_PrintsSummary verifies that on success the init command prints
// a summary containing resource and command counts.
func TestInitCommand_PrintsSummary(t *testing.T) {
	tmpDir := t.TempDir()

	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init",
		"--schema", "../../testdata/petstore.json",
		"--name", "mycli",
		"--output-dir", tmpDir,
	})

	err := root.Execute()
	output := buf.String()

	require.NoError(t, err, "init should exit 0 for a valid spec; output: %s", output)

	// Output should include resource and command counts
	hasResourceCount := strings.Contains(output, "resource")
	hasCommandCount := strings.Contains(output, "command")
	assert.True(t, hasResourceCount, "summary should mention resources; got: %s", output)
	assert.True(t, hasCommandCount, "summary should mention commands; got: %s", output)
}

// TestInitCommand_MissingSchemaFlag verifies that omitting --schema causes an error.
func TestInitCommand_MissingSchemaFlag(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init",
		"--name", "mycli",
	})

	err := root.Execute()
	output := buf.String()

	// Either Execute returns an error OR the error is printed to output
	hasError := err != nil ||
		strings.Contains(strings.ToLower(output), "required") ||
		strings.Contains(strings.ToLower(output), "error") ||
		strings.Contains(strings.ToLower(output), "schema")
	assert.True(t, hasError,
		"init should report an error when --schema is missing; err=%v output=%s", err, output)
}

// TestInitCommand_InvalidSchema verifies that passing an invalid/malformed schema
// file causes the command to report an error.
func TestInitCommand_InvalidSchema(t *testing.T) {
	tmpDir := t.TempDir()

	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init",
		"--schema", "../../testdata/invalid.json",
		"--name", "mycli",
		"--output-dir", tmpDir,
	})

	err := root.Execute()
	output := buf.String()

	hasError := err != nil ||
		strings.Contains(strings.ToLower(output), "error") ||
		strings.Contains(strings.ToLower(output), "invalid") ||
		strings.Contains(strings.ToLower(output), "failed")
	assert.True(t, hasError,
		"init should report failure for invalid schema; err=%v output=%s", err, output)
}

// TestInitCommand_OutputDirDefaultsToName verifies that when --output-dir is omitted,
// the project is generated into a directory named after the --name flag in the cwd.
func TestInitCommand_OutputDirDefaultsToName(t *testing.T) {
	// Change to a temp dir so the default output directory is created there
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init",
		"--schema", filepath.Join(origDir, "../../testdata/petstore.json"),
		"--name", "defaultcli",
	})

	execErr := root.Execute()
	output := buf.String()

	require.NoError(t, execErr, "init should exit 0 when --output-dir is omitted; output: %s", output)

	// The output directory should default to ./<name>/
	expectedDir := filepath.Join(tmpDir, "defaultcli")
	assert.DirExists(t, expectedDir,
		"init should create output in ./<name>/ when --output-dir is omitted")
	assert.FileExists(t, filepath.Join(expectedDir, "main.go"),
		"generated project under default dir should contain main.go")
}
