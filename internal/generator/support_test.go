package generator_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// supportSpec returns a minimal APISpec for exercising support file generators.
func supportSpec() *model.APISpec {
	return &model.APISpec{
		Title:   "Test API",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
	}
}

// mustParseGoSrc is a helper that asserts the given Go source parses without errors.
func mustParseGoSrc(t *testing.T, filename, src string) {
	t.Helper()
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
	assert.NoError(t, err, "generated Go source should parse without syntax errors:\n%s", src)
}

// TestGenerateConfig_EnvVarLoading verifies that the generated config.go
// references loading a token from an environment variable.
func TestGenerateConfig_EnvVarLoading(t *testing.T) {
	src, err := generator.GenerateConfig(supportSpec(), "myapp")
	require.NoError(t, err)
	require.NotEmpty(t, src)

	// The generated config must read a token from an environment variable.
	// Accept any common pattern: os.Getenv, os.LookupEnv, or an ENV_ / _TOKEN suffix.
	hasEnvLoad := strings.Contains(src, "os.Getenv") ||
		strings.Contains(src, "os.LookupEnv") ||
		strings.Contains(src, "ENV") ||
		strings.Contains(src, "_TOKEN") ||
		strings.Contains(src, "_API_KEY")
	assert.True(t, hasEnvLoad,
		"generated config.go should reference loading a token from an environment variable (os.Getenv / _TOKEN / ENV)")
}

// TestGenerateConfig_ConfigFilePath verifies that the generated config.go
// references the ~/.config/<name>/ directory (or an equivalent XDG-style path).
func TestGenerateConfig_ConfigFilePath(t *testing.T) {
	src, err := generator.GenerateConfig(supportSpec(), "myapp")
	require.NoError(t, err)
	require.NotEmpty(t, src)

	// Accept ".config", "UserConfigDir", "UserHomeDir", or the cli name itself
	// as indicators that a config-directory path is being constructed.
	hasConfigPath := strings.Contains(src, ".config") ||
		strings.Contains(src, "UserConfigDir") ||
		strings.Contains(src, "UserHomeDir") ||
		strings.Contains(src, "myapp")
	assert.True(t, hasConfigPath,
		"generated config.go should reference ~/.config/<name>/ or equivalent config directory")
}

// TestGenerateConfig_ValidGoSyntax verifies that the generated config.go is
// syntactically valid Go that can be parsed without errors.
func TestGenerateConfig_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateConfig(supportSpec(), "myapp")
	require.NoError(t, err)
	require.NotEmpty(t, src)

	mustParseGoSrc(t, "config.go", src)
}

// TestGenerateOutput_JSONFlagHandling verifies that the generated output.go
// contains logic for --json flag output handling.
func TestGenerateOutput_JSONFlagHandling(t *testing.T) {
	src, err := generator.GenerateOutput()
	require.NoError(t, err)
	require.NotEmpty(t, src)

	// Must reference json in some form (encoding/json import or "json" string usage).
	assert.Contains(t, src, "json",
		"generated output.go should reference json output handling")

	// Must contain some form of output writing logic.
	hasOutputLogic := strings.Contains(src, "fmt.") ||
		strings.Contains(src, "os.Stdout") ||
		strings.Contains(src, "io.Writer") ||
		strings.Contains(src, "Print")
	assert.True(t, hasOutputLogic,
		"generated output.go should contain output writing logic (fmt., os.Stdout, io.Writer, or Print)")
}

// TestGenerateOutput_ValidGoSyntax verifies that the generated output.go is
// syntactically valid Go that can be parsed without errors.
func TestGenerateOutput_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateOutput()
	require.NoError(t, err)
	require.NotEmpty(t, src)

	mustParseGoSrc(t, "output.go", src)
}

// TestGenerateErrors_StatusCodeHandling verifies that the generated errors.go
// references HTTP status codes in its error formatting logic.
func TestGenerateErrors_StatusCodeHandling(t *testing.T) {
	src, err := generator.GenerateErrors()
	require.NoError(t, err)
	require.NotEmpty(t, src)

	assert.Contains(t, src, "StatusCode",
		"generated errors.go should reference StatusCode for HTTP error response handling")
}

// TestGenerateErrors_ValidGoSyntax verifies that the generated errors.go is
// syntactically valid Go that can be parsed without errors.
func TestGenerateErrors_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateErrors()
	require.NoError(t, err)
	require.NotEmpty(t, src)

	mustParseGoSrc(t, "errors.go", src)
}
