package generator_test

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/queso/swagger-jack/internal/generator"
	"github.com/queso/swagger-jack/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rootSpec returns an APISpec suitable for exercising GenerateRoot.
func rootSpec() *model.APISpec {
	return &model.APISpec{
		Title:   "Pet Store API",
		Version: "1.0.0",
		BaseURL: "https://petstore.example.com",
	}
}

// TestGenerateRoot_PackageDeclaration verifies that the generated root.go starts
// with `package cmd`, as required for inclusion in the generated project's cmd/
// directory.
func TestGenerateRoot_PackageDeclaration(t *testing.T) {
	src, err := generator.GenerateRoot(rootSpec(), "petstore")
	require.NoError(t, err)
	require.NotEmpty(t, src, "generated source should not be empty")

	assert.Contains(t, src, "package cmd", "generated root.go must declare package cmd")
}

// TestGenerateRoot_RootCommandUse verifies that the generated root cobra.Command
// has its Use field set to the CLI name provided as the name parameter.
func TestGenerateRoot_RootCommandUse(t *testing.T) {
	src, err := generator.GenerateRoot(rootSpec(), "petstore")
	require.NoError(t, err)

	// The Use field should contain the CLI name so that cobra renders the
	// correct usage line (e.g., "petstore [command]").
	assert.Contains(t, src, "petstore", "generated root command Use field must include the CLI name")
}

// TestGenerateRoot_GlobalFlags verifies that the generated root command registers
// all 5 required global persistent flags: --json (bool), --verbose (bool),
// --config (string), --base-url (string), and --no-color (bool).
func TestGenerateRoot_GlobalFlags(t *testing.T) {
	src, err := generator.GenerateRoot(rootSpec(), "petstore")
	require.NoError(t, err)

	assert.Contains(t, src, `"json"`, `generated code must register --json flag`)
	assert.Contains(t, src, `"verbose"`, `generated code must register --verbose flag`)
	assert.Contains(t, src, `"config"`, `generated code must register --config flag`)
	assert.Contains(t, src, `"base-url"`, `generated code must register --base-url flag`)
	assert.Contains(t, src, `"no-color"`, `generated code must register --no-color flag`)
}

// TestGenerateRoot_ExecuteFunction verifies that the generated source exports an
// Execute() function, which is the conventional cobra entry point called from main.
func TestGenerateRoot_ExecuteFunction(t *testing.T) {
	src, err := generator.GenerateRoot(rootSpec(), "petstore")
	require.NoError(t, err)

	assert.Contains(t, src, "func Execute()", "generated root.go must export an Execute() function")
}

// TestGenerateRoot_ValidGoSyntax verifies that the generated source is
// syntactically valid Go that can be parsed without errors.
func TestGenerateRoot_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateRoot(rootSpec(), "petstore")
	require.NoError(t, err)
	require.NotEmpty(t, src, "generated source should not be empty")

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "root.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "generated root.go should be valid Go syntax:\n%s", src)
}

// TestGenerateRoot_BaseURLDefault verifies that the generated code uses the
// BaseURL from the APISpec as the default value for the --base-url flag,
// so the generated CLI works out of the box without configuration.
func TestGenerateRoot_BaseURLDefault(t *testing.T) {
	src, err := generator.GenerateRoot(rootSpec(), "petstore")
	require.NoError(t, err)

	assert.Contains(t, src, "https://petstore.example.com",
		"generated code must use spec.BaseURL as the default value for --base-url flag")
}

// TestGenerateRoot_TitleInShortDescription verifies that the spec's Title appears
// in the generated root command's Short description field.
func TestGenerateRoot_TitleInShortDescription(t *testing.T) {
	src, err := generator.GenerateRoot(rootSpec(), "petstore")
	require.NoError(t, err)

	assert.Contains(t, src, "Pet Store API",
		"generated root command Short description should include the spec Title")
}
