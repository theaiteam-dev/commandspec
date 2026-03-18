// Package generator_test contains tests for shell completion generation in
// generated CLI projects.
package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalSpecForCompletion returns a minimal APISpec suitable for generator tests.
func minimalSpecForCompletion() *model.APISpec {
	return &model.APISpec{
		Title:   "Completion Test API",
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

// TestGenerateCompletion_ReturnsSource verifies that GenerateCompletion()
// returns non-empty Go source.
// FAILS until internal/generator/completion.go is created.
func TestGenerateCompletion_ReturnsSource(t *testing.T) {
	src, err := generator.GenerateCompletion("myapi")
	require.NoError(t, err)
	assert.NotEmpty(t, src, "GenerateCompletion should return non-empty Go source")
}

// TestGenerateCompletion_PackageCmd verifies the generated file declares package cmd.
// FAILS until internal/generator/completion.go is created.
func TestGenerateCompletion_PackageCmd(t *testing.T) {
	src, err := generator.GenerateCompletion("myapi")
	require.NoError(t, err)
	assert.Contains(t, src, "package cmd",
		"generated completion.go should declare package cmd")
}

// TestGenerateCompletion_SupportsBash verifies the generated command handles bash.
// FAILS until internal/generator/completion.go is created.
func TestGenerateCompletion_SupportsBash(t *testing.T) {
	src, err := generator.GenerateCompletion("myapi")
	require.NoError(t, err)
	assert.Contains(t, src, "bash",
		"generated completion command should support bash shell")
}

// TestGenerateCompletion_SupportsZsh verifies the generated command handles zsh.
// FAILS until internal/generator/completion.go is created.
func TestGenerateCompletion_SupportsZsh(t *testing.T) {
	src, err := generator.GenerateCompletion("myapi")
	require.NoError(t, err)
	assert.Contains(t, src, "zsh",
		"generated completion command should support zsh shell")
}

// TestGenerateCompletion_SupportsFish verifies the generated command handles fish.
// FAILS until internal/generator/completion.go is created.
func TestGenerateCompletion_SupportsFish(t *testing.T) {
	src, err := generator.GenerateCompletion("myapi")
	require.NoError(t, err)
	assert.Contains(t, src, "fish",
		"generated completion command should support fish shell")
}

// TestGenerateCompletion_SupportsPowershell verifies the generated command handles powershell.
// FAILS until internal/generator/completion.go is created.
func TestGenerateCompletion_SupportsPowershell(t *testing.T) {
	src, err := generator.GenerateCompletion("myapi")
	require.NoError(t, err)
	assert.Contains(t, src, "powershell",
		"generated completion command should support powershell shell")
}

// TestGenerateCompletion_UsesCobraGenCompletion verifies that the generated command
// uses Cobra's built-in completion generation methods.
// FAILS until internal/generator/completion.go is created.
func TestGenerateCompletion_UsesCobraGenCompletion(t *testing.T) {
	src, err := generator.GenerateCompletion("myapi")
	require.NoError(t, err)
	hasCobraCompletion := strings.Contains(src, "GenBashCompletion") ||
		strings.Contains(src, "GenZshCompletion") ||
		strings.Contains(src, "GenFishCompletion") ||
		strings.Contains(src, "GenPowerShellCompletion") ||
		strings.Contains(src, "GenCompletion")
	assert.True(t, hasCobraCompletion,
		"generated completion command should use Cobra's built-in completion generation")
}

// TestGenerateCompletion_ValidGoSyntax verifies the generated completion.go is
// valid Go syntax.
// FAILS until internal/generator/completion.go is created.
func TestGenerateCompletion_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateCompletion("myapi")
	require.NoError(t, err)
	mustParseGoSrc(t, "completion.go", src)
}

// TestGenerateCompletion_RegistersInInit verifies that the generated completion.go
// registers the completion command via its own init() function, following the
// same self-registration pattern used by resource command files.
func TestGenerateCompletion_RegistersInInit(t *testing.T) {
	src, err := generator.GenerateCompletion("myapi")
	require.NoError(t, err)
	assert.Contains(t, src, "func init()",
		"generated completion.go should have an init() function")
	assert.Contains(t, src, "rootCmd.AddCommand(completionCmd)",
		"generated completion.go init() should register the completion command")
}

// TestGenerate_CompletionFileCreated verifies that the full Generate() pipeline
// creates a cmd/completion.go file in the output directory.
// FAILS until generator.go calls GenerateCompletion and writes the file.
func TestGenerate_CompletionFileCreated(t *testing.T) {
	spec := minimalSpecForCompletion()
	outputDir := t.TempDir()
	err := generator.Generate(spec, "comptest", outputDir)
	require.NoError(t, err, "Generate should succeed")

	completionPath := filepath.Join(outputDir, "cmd", "completion.go")
	_, err = os.Stat(completionPath)
	assert.NoError(t, err,
		"Generate should create cmd/completion.go in the output directory")
}
