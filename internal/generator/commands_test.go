package generator_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/queso/swagger-jack/internal/generator"
	"github.com/queso/swagger-jack/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateResourceCmd_PackageAndUse verifies that the generated resource
// group file declares "package cmd" and a Cobra command with Use set to the
// resource name.
func TestGenerateResourceCmd_PackageAndUse(t *testing.T) {
	res := model.Resource{
		Name:        "users",
		Description: "Manage user accounts",
	}

	src, err := generator.GenerateResourceCmd(res)
	require.NoError(t, err)

	assert.Contains(t, src, "package cmd", "should declare package cmd")
	assert.Contains(t, src, `Use: "users"`, "should set Use to the resource name")
}

// TestGenerateResourceCmd_ShortDescription verifies that the resource group
// command uses the resource Description in its Short field, falling back to
// the resource name when Description is empty.
func TestGenerateResourceCmd_ShortDescription(t *testing.T) {
	t.Run("uses Description when provided", func(t *testing.T) {
		res := model.Resource{
			Name:        "orders",
			Description: "Manage orders in the system",
		}

		src, err := generator.GenerateResourceCmd(res)
		require.NoError(t, err)

		assert.Contains(t, src, "Manage orders in the system",
			"Short should contain the resource Description")
	})

	t.Run("falls back to name when Description is empty", func(t *testing.T) {
		res := model.Resource{
			Name:        "widgets",
			Description: "",
		}

		src, err := generator.GenerateResourceCmd(res)
		require.NoError(t, err)

		assert.Contains(t, src, "widgets",
			"Short should contain the resource name when Description is empty")
	})
}

// TestGenerateResourceCmd_ValidGoSyntax verifies that the generated resource
// command file is syntactically valid Go.
func TestGenerateResourceCmd_ValidGoSyntax(t *testing.T) {
	res := model.Resource{
		Name:        "products",
		Description: "Product catalog management",
	}

	src, err := generator.GenerateResourceCmd(res)
	require.NoError(t, err)
	require.NotEmpty(t, src, "generated source should not be empty")

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "products.go", src, parser.AllErrors)
	assert.NoError(t, parseErr,
		"generated Go source should parse without syntax errors:\n%s", src)
}

// TestGenerateVerbCmd_ArgsWiring verifies that a command with one path argument
// generates cobra.ExactArgs(1) in the verb command file.
func TestGenerateVerbCmd_ArgsWiring(t *testing.T) {
	res := model.Resource{
		Name:        "users",
		Description: "Manage user accounts",
	}
	cmd := model.Command{
		Name:       "get",
		HTTPMethod: "GET",
		Path:       "/users/{userId}",
		Args: []model.Arg{
			{Name: "user-id", Description: "The user ID", Required: true},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd)
	require.NoError(t, err)

	assert.Contains(t, src, "cobra.ExactArgs(1)",
		"should wire cobra.ExactArgs(1) for a command with 1 path arg")
}

// TestGenerateVerbCmd_ZeroArgs verifies that a command with no path arguments
// generates cobra.ExactArgs(0) (or NoArgs).
func TestGenerateVerbCmd_ZeroArgs(t *testing.T) {
	res := model.Resource{
		Name: "users",
	}
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/users",
		Args:       []model.Arg{},
	}

	src, err := generator.GenerateVerbCmd(res, cmd)
	require.NoError(t, err)

	hasZeroArgs := strings.Contains(src, "cobra.ExactArgs(0)") ||
		strings.Contains(src, "cobra.NoArgs")
	assert.True(t, hasZeroArgs,
		"should wire ExactArgs(0) or NoArgs for a command with 0 path args")
}

// TestGenerateVerbCmd_FlagTypes verifies that string, int, and bool flags are
// all registered correctly using the appropriate Flags().XxxVar methods.
func TestGenerateVerbCmd_FlagTypes(t *testing.T) {
	res := model.Resource{
		Name: "items",
	}
	cmd := model.Command{
		Name:       "create",
		HTTPMethod: "POST",
		Path:       "/items",
		Flags: []model.Flag{
			{
				Name:   "name",
				Type:   model.FlagTypeString,
				Source: model.FlagSourceBody,
			},
			{
				Name:   "count",
				Type:   model.FlagTypeInt,
				Source: model.FlagSourceBody,
			},
			{
				Name:   "active",
				Type:   model.FlagTypeBool,
				Source: model.FlagSourceQuery,
			},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd)
	require.NoError(t, err)

	assert.Contains(t, src, "StringVar", "should register string flags with StringVar")
	assert.Contains(t, src, "IntVar", "should register int flags with IntVar")
	assert.Contains(t, src, "BoolVar", "should register bool flags with BoolVar")
}

// TestGenerateVerbCmd_RequiredFlags verifies that flags marked Required: true
// generate a MarkFlagRequired call so Cobra enforces them at runtime.
func TestGenerateVerbCmd_RequiredFlags(t *testing.T) {
	res := model.Resource{
		Name: "orders",
	}
	cmd := model.Command{
		Name:       "create",
		HTTPMethod: "POST",
		Path:       "/orders",
		Flags: []model.Flag{
			{
				Name:     "customer-id",
				Type:     model.FlagTypeString,
				Required: true,
				Source:   model.FlagSourceBody,
			},
			{
				Name:     "notes",
				Type:     model.FlagTypeString,
				Required: false,
				Source:   model.FlagSourceBody,
			},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd)
	require.NoError(t, err)

	assert.Contains(t, src, "MarkFlagRequired",
		"required flags should generate a MarkFlagRequired call")
	// The required flag name should appear near MarkFlagRequired
	assert.Contains(t, src, "customer-id",
		"the required flag name should appear in the generated source")
}

// TestGenerateVerbCmd_ValidGoSyntax verifies that the generated verb command
// file for a realistic command is syntactically valid Go.
func TestGenerateVerbCmd_ValidGoSyntax(t *testing.T) {
	res := model.Resource{
		Name:        "users",
		Description: "Manage user accounts",
	}
	cmd := model.Command{
		Name:        "update",
		HTTPMethod:  "PATCH",
		Path:        "/users/{userId}",
		Description: "Update a user by ID",
		Args: []model.Arg{
			{Name: "user-id", Description: "The user ID", Required: true},
		},
		Flags: []model.Flag{
			{
				Name:     "email",
				Type:     model.FlagTypeString,
				Required: false,
				Source:   model.FlagSourceBody,
			},
			{
				Name:     "role",
				Type:     model.FlagTypeString,
				Required: true,
				Source:   model.FlagSourceBody,
			},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd)
	require.NoError(t, err)
	require.NotEmpty(t, src, "generated source should not be empty")

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "users_update.go", src, parser.AllErrors)
	assert.NoError(t, parseErr,
		"generated Go source should parse without syntax errors:\n%s", src)
}

// TestGenerateVerbCmd_UseField verifies that the generated verb command has a
// Use field that includes the command name and positional arg placeholders.
func TestGenerateVerbCmd_UseField(t *testing.T) {
	res := model.Resource{
		Name: "repos",
	}
	cmd := model.Command{
		Name:       "get",
		HTTPMethod: "GET",
		Path:       "/repos/{owner}/{repo}",
		Args: []model.Arg{
			{Name: "owner", Required: true},
			{Name: "repo", Required: true},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd)
	require.NoError(t, err)

	// Use should start with the command verb name
	assert.Contains(t, src, `"get`, "Use field should start with the command name")
	// Positional arg placeholders should be present
	assert.Contains(t, src, "owner", "Use field should include arg placeholder for owner")
	assert.Contains(t, src, "repo", "Use field should include arg placeholder for repo")
}
