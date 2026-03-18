package model_test

import (
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makePathParam builds a minimal OpenAPI parameter object for a path parameter.
func makePathParam(name string, schemaType string, required bool) map[string]interface{} {
	return map[string]interface{}{
		"name":     name,
		"in":       "path",
		"required": required,
		"schema": map[string]interface{}{
			"type": schemaType,
		},
	}
}

// makeQueryParam builds a minimal OpenAPI parameter object for a query parameter.
func makeQueryParam(name string, schemaType string, required bool) map[string]interface{} {
	return map[string]interface{}{
		"name":     name,
		"in":       "query",
		"required": required,
		"schema": map[string]interface{}{
			"type": schemaType,
		},
	}
}

// makeQueryParamWithDefault builds a query parameter with a default value in the schema.
func makeQueryParamWithDefault(name string, schemaType string, defaultVal interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":     name,
		"in":       "query",
		"required": false,
		"schema": map[string]interface{}{
			"type":    schemaType,
			"default": defaultVal,
		},
	}
}

// makeArrayQueryParam builds a query parameter of array type with string items.
func makeArrayQueryParam(name string, required bool) map[string]interface{} {
	return map[string]interface{}{
		"name":     name,
		"in":       "query",
		"required": required,
		"schema": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "string",
			},
		},
	}
}

// TestExtractParamsPathNameCleaning verifies that path parameter names are cleaned:
// camelCase braced names → lowercase kebab-case, simple names unchanged.
func TestExtractParamsPathNameCleaning(t *testing.T) {
	params := []interface{}{
		makePathParam("userId", "string", true),
		makePathParam("petId", "integer", true),
		makePathParam("id", "string", true),
		makePathParam("orderId", "string", true),
	}

	args, flags, err := model.ExtractParams(nil, params)
	require.NoError(t, err)
	assert.Empty(t, flags, "path params should not produce flags")
	require.Len(t, args, 4)

	assert.Equal(t, "user-id", args[0].Name, "{userId} should become user-id")
	assert.Equal(t, "pet-id", args[1].Name, "{petId} should become pet-id")
	assert.Equal(t, "id", args[2].Name, "{id} should remain id")
	assert.Equal(t, "order-id", args[3].Name, "{orderId} should become order-id")
}

// TestExtractParamsQueryTypeMapping verifies that OpenAPI schema types map to
// the correct model.FlagType values.
func TestExtractParamsQueryTypeMapping(t *testing.T) {
	params := []interface{}{
		makeQueryParam("name", "string", false),
		makeQueryParam("limit", "integer", false),
		makeQueryParam("active", "boolean", false),
		makeArrayQueryParam("tags", false),
	}

	args, flags, err := model.ExtractParams(nil, params)
	require.NoError(t, err)
	assert.Empty(t, args, "query params should not produce args")
	require.Len(t, flags, 4)

	assert.Equal(t, model.FlagTypeString, flags[0].Type, "string → FlagTypeString")
	assert.Equal(t, model.FlagTypeInt, flags[1].Type, "integer → FlagTypeInt")
	assert.Equal(t, model.FlagTypeBool, flags[2].Type, "boolean → FlagTypeBool")
	assert.Equal(t, model.FlagTypeStringSlice, flags[3].Type, "array(string) → FlagTypeStringSlice")
}

// TestExtractParamsRequiredField verifies that the required field is correctly
// propagated for both path args and query flags.
func TestExtractParamsRequiredField(t *testing.T) {
	params := []interface{}{
		makeQueryParam("status", "string", true),
		makeQueryParam("format", "string", false),
	}

	_, flags, err := model.ExtractParams(nil, params)
	require.NoError(t, err)
	require.Len(t, flags, 2)

	assert.True(t, flags[0].Required, "required query param should produce required flag")
	assert.False(t, flags[1].Required, "optional query param should produce non-required flag")
}

// TestExtractParamsDefaultValue verifies that schema.default is preserved on
// the resulting Flag as a string.
func TestExtractParamsDefaultValue(t *testing.T) {
	params := []interface{}{
		makeQueryParamWithDefault("limit", "integer", "20"),
		makeQueryParamWithDefault("format", "string", "json"),
	}

	_, flags, err := model.ExtractParams(nil, params)
	require.NoError(t, err)
	require.Len(t, flags, 2)

	assert.Equal(t, "20", flags[0].Default, "integer param with default '20' should set Default to '20'")
	assert.Equal(t, "json", flags[1].Default, "string param with default 'json' should set Default to 'json'")
}

// TestExtractParamsMixedOperation verifies that an operation with one path param
// and two query params produces the correct number and types of args and flags,
// and that query flags have Source set to FlagSourceQuery.
func TestExtractParamsMixedOperation(t *testing.T) {
	params := []interface{}{
		makePathParam("userId", "string", true),
		makeQueryParam("limit", "integer", false),
		makeQueryParam("status", "string", true),
	}

	args, flags, err := model.ExtractParams(nil, params)
	require.NoError(t, err)

	require.Len(t, args, 1, "expected 1 path arg")
	assert.Equal(t, "user-id", args[0].Name)
	assert.True(t, args[0].Required, "path param should always be required")

	require.Len(t, flags, 2, "expected 2 query flags")
	for _, f := range flags {
		assert.Equal(t, model.FlagSourceQuery, f.Source, "query param flag should have Source=FlagSourceQuery")
	}

	assert.Equal(t, "limit", flags[0].Name)
	assert.Equal(t, model.FlagTypeInt, flags[0].Type)
	assert.False(t, flags[0].Required)

	assert.Equal(t, "status", flags[1].Name)
	assert.Equal(t, model.FlagTypeString, flags[1].Type)
	assert.True(t, flags[1].Required)
}

// TestExtractParamsEmptyParams verifies that an empty params slice returns
// empty slices and no error.
func TestExtractParamsEmptyParams(t *testing.T) {
	args, flags, err := model.ExtractParams(nil, []interface{}{})
	require.NoError(t, err)
	assert.Empty(t, args)
	assert.Empty(t, flags)
}

// TestExtractParamsNilParams verifies that a nil params slice is handled
// gracefully, returning empty slices and no error.
func TestExtractParamsNilParams(t *testing.T) {
	args, flags, err := model.ExtractParams(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, args)
	assert.Empty(t, flags)
}
