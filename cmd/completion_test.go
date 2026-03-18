// Package cmd_test contains tests for the completion command in CommandSpec CLI.
package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeCompletion runs the root command with `completion <shell>` and
// returns stdout output and any error.
func executeCompletion(t *testing.T, shell string) (string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"completion", shell})
	err := root.Execute()
	return out.String(), err
}

// TestCompletionBash verifies that `cmdspec completion bash` outputs a bash completion script.
// FAILS until cmd/completion.go is created and registered.
func TestCompletionBash(t *testing.T) {
	out, err := executeCompletion(t, "bash")
	require.NoError(t, err, "completion bash should not error")
	assert.NotEmpty(t, out, "completion bash should produce output")
	// Bash completion scripts typically start with a shebang or bash function
	assert.True(t, strings.Contains(out, "bash") || strings.Contains(out, "complete") || strings.Contains(out, "#"),
		"bash completion output should contain bash-specific content, got: %q", out)
}

// TestCompletionZsh verifies that `cmdspec completion zsh` outputs a zsh completion script.
// FAILS until cmd/completion.go is created and registered.
func TestCompletionZsh(t *testing.T) {
	out, err := executeCompletion(t, "zsh")
	require.NoError(t, err, "completion zsh should not error")
	assert.NotEmpty(t, out, "completion zsh should produce output")
	assert.True(t, strings.Contains(out, "#") || strings.Contains(out, "zsh"),
		"zsh completion output should contain zsh-specific content, got: %q", out)
}

// TestCompletionFish verifies that `cmdspec completion fish` outputs a fish completion script.
// FAILS until cmd/completion.go is created and registered.
func TestCompletionFish(t *testing.T) {
	out, err := executeCompletion(t, "fish")
	require.NoError(t, err, "completion fish should not error")
	assert.NotEmpty(t, out, "completion fish should produce output")
}

// TestCompletionPowershell verifies that `cmdspec completion powershell` outputs a powershell script.
// FAILS until cmd/completion.go is created and registered.
func TestCompletionPowershell(t *testing.T) {
	out, err := executeCompletion(t, "powershell")
	require.NoError(t, err, "completion powershell should not error")
	assert.NotEmpty(t, out, "completion powershell should produce output")
}

// TestCompletionInvalidShell verifies that an unknown shell name returns an error.
// FAILS until cmd/completion.go is created and registered.
func TestCompletionInvalidShell(t *testing.T) {
	_, err := executeCompletion(t, "invalidshell")
	assert.Error(t, err, "completion with an invalid shell should return an error")
}

// TestCompletionCommandRegistered verifies that the completion subcommand
// appears in the root command's available subcommands.
// FAILS until cmd/completion.go is created and registered in root.go.
func TestCompletionCommandRegistered(t *testing.T) {
	root := cmd.NewRootCmd()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "completion" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'completion' subcommand to be registered on root command")
}

// TestCompletionRequiresShellArg verifies that running `completion` without
// any argument returns an error (ExactArgs(1) enforced).
// FAILS until cmd/completion.go is created and registered.
func TestCompletionRequiresShellArg(t *testing.T) {
	root := cmd.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"completion"})
	err := root.Execute()
	assert.Error(t, err, "completion without a shell arg should return an error")
}
