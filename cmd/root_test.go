package cmd_test

import (
	"bytes"
	"testing"

	"github.com/queso/swagger-jack/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootCommandUse verifies the root command has the correct Use field.
func TestRootCommandUse(t *testing.T) {
	root := cmd.NewRootCmd()
	assert.Equal(t, "swaggerjack", root.Use)
}

// TestRootCommandHelpNonEmpty verifies the root command produces non-empty help text
// and the short description is set.
func TestRootCommandHelpNonEmpty(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--help"})
	_ = root.Execute()
	output := buf.String()
	assert.NotEmpty(t, output, "help output should not be empty")
	// Short description should appear in help output
	assert.Contains(t, output, "OpenAPI")
}

// TestRootCommandExecuteNoArgs verifies Execute doesn't panic when run with no args.
func TestRootCommandExecuteNoArgs(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{})
	require.NotPanics(t, func() {
		_ = root.Execute()
	})
}
