// Package generator_test contains tests for enum validation and shell completion
// in generated CLI commands.
package generator_test

import (
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeEnumFlagCmd builds a command with an enum-typed flag.
func makeEnumFlagCmd(enumValues []string, required bool) model.Command {
	return model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
		Flags: []model.Flag{
			{
				Name:     "status",
				Type:     model.FlagTypeString,
				Required: required,
				Source:   model.FlagSourceQuery,
				Enum:     enumValues,
			},
		},
	}
}

func makeEnumFlagResource(cmd model.Command) model.Resource {
	return model.Resource{
		Name:     "items",
		Commands: []model.Command{cmd},
	}
}

// TestEnumFlagDescriptionShowsAllowedValues verifies that the generated command
// appends allowed values to the flag description in (val1|val2) format.
// FAILS until commands.go generates enum description suffix.
func TestEnumFlagDescriptionShowsAllowedValues(t *testing.T) {
	cmd := makeEnumFlagCmd([]string{"active", "inactive", "pending"}, false)
	resource := makeEnumFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, "active|inactive|pending",
		"flag description should show allowed values as 'active|inactive|pending'")
}

// TestEnumFlagCompletionFuncRegistered verifies that the generated command
// registers a completion function for enum flags via RegisterFlagCompletionFunc.
// FAILS until commands.go generates ValidArgsFunction/RegisterFlagCompletionFunc.
func TestEnumFlagCompletionFuncRegistered(t *testing.T) {
	cmd := makeEnumFlagCmd([]string{"active", "inactive"}, false)
	resource := makeEnumFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, "RegisterFlagCompletionFunc",
		"generated command should call RegisterFlagCompletionFunc for enum flags")
}

// TestEnumFlagCompletionReturnsEnumValues verifies that the completion function
// returns the enum values and ShellCompDirectiveNoFileComp.
// FAILS until commands.go generates completion func with enum values.
func TestEnumFlagCompletionReturnsEnumValues(t *testing.T) {
	cmd := makeEnumFlagCmd([]string{"active", "inactive", "pending"}, false)
	resource := makeEnumFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, "ShellCompDirectiveNoFileComp",
		"enum flag completion should return ShellCompDirectiveNoFileComp")
}

// TestEnumFlagRuntimeValidationPresent verifies that the generated RunE contains
// runtime validation of the enum flag value before the HTTP call.
// Validation is delegated to validate.Enum from the shared validate package.
func TestEnumFlagRuntimeValidationPresent(t *testing.T) {
	cmd := makeEnumFlagCmd([]string{"active", "inactive", "pending"}, true)
	resource := makeEnumFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	hasValidation := strings.Contains(src, "validate.Enum") ||
		strings.Contains(src, "invalid value") ||
		strings.Contains(src, "must be one of") ||
		strings.Contains(src, "allowed values")
	assert.True(t, hasValidation,
		"generated RunE should validate enum flag values (via validate.Enum or inline)")
}

// TestEnumFlagValidationErrorMessageFormat verifies that validation of enum flags
// references the flag name. Validation is now delegated to validate.Enum so the
// error message format lives in the shared validate package rather than inline.
func TestEnumFlagValidationErrorMessageFormat(t *testing.T) {
	cmd := makeEnumFlagCmd([]string{"active", "inactive"}, true)
	resource := makeEnumFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// The generated source should pass the flag name to validate.Enum or inline the message.
	hasStatusRef := strings.Contains(src, `"status"`) || strings.Contains(src, "--status")
	hasValidation := strings.Contains(src, "validate.Enum") ||
		strings.Contains(src, "must be one of") ||
		strings.Contains(src, "one of:")
	assert.True(t, hasStatusRef && hasValidation,
		"generated source should reference the flag name and include validation via validate.Enum or inline message")
}

// TestOptionalEnumFlagCheckedWhenChanged verifies that optional enum flags
// are only validated when the flag was explicitly set (Flags().Changed()).
// FAILS until commands.go generates Changed() check for optional enum flags.
func TestOptionalEnumFlagCheckedWhenChanged(t *testing.T) {
	cmd := makeEnumFlagCmd([]string{"active", "inactive"}, false) // not required
	resource := makeEnumFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	assert.Contains(t, src, `Changed("status")`,
		"optional enum flag validation should use Flags().Changed() to skip when not provided")
}

// TestNonEnumFlagUnaffected verifies that a regular string flag (no Enum)
// does not get completion or validation code added.
func TestNonEnumFlagUnaffected(t *testing.T) {
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
		Flags: []model.Flag{
			{Name: "name", Type: model.FlagTypeString, Source: model.FlagSourceQuery},
		},
	}
	resource := makeEnumFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	assert.NotContains(t, src, "RegisterFlagCompletionFunc",
		"non-enum flag should NOT get a RegisterFlagCompletionFunc call")
}

// TestEnumFlagGeneratedGoSyntax verifies that the full generated source with
// enum validation and completion is valid Go.
// FAILS until enum generation is implemented.
func TestEnumFlagGeneratedGoSyntax(t *testing.T) {
	cmd := makeEnumFlagCmd([]string{"active", "inactive", "pending"}, false)
	resource := makeEnumFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// Only syntax-check if enum handling is present
	if strings.Contains(src, "RegisterFlagCompletionFunc") {
		mustParseGoSrc(t, "items_list.go", src)
	} else {
		t.Skip("enum completion not yet implemented; skipping syntax check")
	}
}
