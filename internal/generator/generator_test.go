package generator_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSpec returns a minimal APISpec for exercising Generate.
func testSpec() *model.APISpec {
	return &model.APISpec{
		Title:   "Test API",
		Version: "1.0.0",
	}
}

// TestGenerate_CreatesDirectoryStructure verifies that Generate creates the
// expected subdirectory layout: cmd/ and internal/ under outputDir.
func TestGenerate_CreatesDirectoryStructure(t *testing.T) {
	outputDir := t.TempDir()

	err := generator.Generate(testSpec(), "myapp", outputDir)
	require.NoError(t, err)

	cmdDir := filepath.Join(outputDir, "cmd")
	internalDir := filepath.Join(outputDir, "internal")

	info, err := os.Stat(cmdDir)
	require.NoError(t, err, "cmd/ directory should exist")
	assert.True(t, info.IsDir(), "cmd/ should be a directory")

	info, err = os.Stat(internalDir)
	require.NoError(t, err, "internal/ directory should exist")
	assert.True(t, info.IsDir(), "internal/ should be a directory")
}

// TestGenerate_MainGoContent verifies that the generated main.go contains the
// expected package declaration, import of the cmd package, and a call to
// cmd.Execute().
func TestGenerate_MainGoContent(t *testing.T) {
	outputDir := t.TempDir()

	err := generator.Generate(testSpec(), "myapp", outputDir)
	require.NoError(t, err)

	mainGoPath := filepath.Join(outputDir, "main.go")
	data, err := os.ReadFile(mainGoPath)
	require.NoError(t, err, "main.go should exist")

	src := string(data)
	assert.Contains(t, src, "package main", "main.go should declare package main")
	assert.Contains(t, src, "cmd", "main.go should import the cmd package")
	assert.Contains(t, src, "cmd.Execute()", "main.go should call cmd.Execute()")
}

// TestGenerate_GoModContent verifies that the generated go.mod includes the
// module name derived from the name param, the go directive, and the cobra
// dependency.
func TestGenerate_GoModContent(t *testing.T) {
	outputDir := t.TempDir()

	err := generator.Generate(testSpec(), "myapp", outputDir)
	require.NoError(t, err)

	goModPath := filepath.Join(outputDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	require.NoError(t, err, "go.mod should exist")

	src := string(data)
	assert.Contains(t, src, "myapp", "go.mod should contain the module name")
	assert.Contains(t, src, "go 1.21", "go.mod should specify go 1.21")
	assert.Contains(t, src, "github.com/spf13/cobra", "go.mod should require cobra")
}

// TestGenerate_GoModFormattedCorrectly verifies that the generated go.mod is
// well-formed: it must start with a module directive and include a require block.
func TestGenerate_GoModFormattedCorrectly(t *testing.T) {
	outputDir := t.TempDir()

	err := generator.Generate(testSpec(), "myapp", outputDir)
	require.NoError(t, err)

	goModPath := filepath.Join(outputDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	require.NoError(t, err, "go.mod should exist")

	src := string(data)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(src), "module "),
		"go.mod must start with a module directive")
	assert.Contains(t, src, "require", "go.mod must contain a require block")
}

// TestGenerate_MainGoValidGoSyntax verifies that the generated main.go is
// syntactically valid Go by parsing it with go/parser.
func TestGenerate_MainGoValidGoSyntax(t *testing.T) {
	outputDir := t.TempDir()

	err := generator.Generate(testSpec(), "myapp", outputDir)
	require.NoError(t, err)

	mainGoPath := filepath.Join(outputDir, "main.go")
	data, err := os.ReadFile(mainGoPath)
	require.NoError(t, err, "main.go should exist")

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "main.go", data, parser.AllErrors)
	assert.NoError(t, parseErr, "generated main.go should be valid Go syntax:\n%s", string(data))
}

// TestGenerate_NameValidation_InvalidCharacters verifies that Generate rejects
// names containing characters that would produce invalid shell environment
// variable names (e.g. @, !, (, ), space, backtick). Such characters survive
// cliNameToEnvPrefix unchanged and cause auth tokens to be silently unreadable.
func TestGenerate_NameValidation_InvalidCharacters(t *testing.T) {
	invalidNames := []string{
		"my.api@v1", // @ is invalid
		"my-cli!",   // ! is invalid
		"my(cli)",   // parentheses are invalid
		"my cli",    // space is invalid
		"my`cli",    // backtick is invalid
		"cli#1",     // # is invalid
		"cli$name",  // $ is invalid
		"cli%name",  // % is invalid
		"cli^name",  // ^ is invalid
		"cli&name",  // & is invalid
		"cli*name",  // * is invalid
		"cli=name",  // = is invalid
		"cli+name",  // + is invalid
		"cli[name]", // brackets are invalid
		"cli{name}", // braces are invalid
		"cli|name",  // pipe is invalid
		"cli\\name", // backslash is invalid
		"cli;name",  // semicolon is invalid
		"cli:name",  // colon is invalid
		"cli'name",  // single quote is invalid
		`cli"name`,  // double quote is invalid
		"cli<name",  // angle bracket is invalid
		"cli>name",  // angle bracket is invalid
		"cli,name",  // comma is invalid
		"cli?name",  // question mark is invalid
		"cli/name",  // slash is invalid
		"cli~name",  // tilde is invalid
		"cli`name",  // backtick is invalid
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			outputDir := t.TempDir()
			err := generator.Generate(testSpec(), name, outputDir)
			assert.Error(t, err,
				"Generate should reject name %q containing an invalid character", name)
			assert.Contains(t, err.Error(), "invalid character",
				"error message should mention 'invalid character' for name %q", name)
		})
	}
}

// TestGenerate_NameValidation_ValidNames verifies that Generate accepts names
// containing only alphanumerics, hyphens, and dots — the allowed character set
// for shell-safe env var prefixes and go.mod module paths.
func TestGenerate_NameValidation_ValidNames(t *testing.T) {
	validNames := []string{
		"myapp",
		"my-app",
		"my.app",
		"my-api.v1",
		"MyApp",
		"myapp123",
		"my-api-v2",
		"my.api.v2",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			outputDir := t.TempDir()
			err := generator.Generate(testSpec(), name, outputDir)
			assert.NoError(t, err,
				"Generate should accept valid name %q", name)
		})
	}
}

// TestGenerate_ErrorOnBadOutputDir verifies that Generate returns an error
// when the output directory cannot be created (e.g., a file blocks the path).
func TestGenerate_ErrorOnBadOutputDir(t *testing.T) {
	// Create a regular file where Generate would try to create a directory.
	parent := t.TempDir()
	blocker := filepath.Join(parent, "blocker")
	err := os.WriteFile(blocker, []byte("not a dir"), 0o600)
	require.NoError(t, err)

	// Attempt to use a nested path under that file — impossible to mkdir.
	outputDir := filepath.Join(blocker, "nested")
	err = generator.Generate(testSpec(), "myapp", outputDir)
	assert.Error(t, err, "Generate should return an error when outputDir cannot be created")
}
