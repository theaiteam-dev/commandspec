// Package model_test contains tests for enum field extraction in the model builder.
package model_test

import (
	"encoding/json"
	"testing"

	"github.com/queso/swagger-jack/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildQueryParamWithEnum constructs a raw OpenAPI query parameter map that
// includes an enum array on its schema.
func buildQueryParamWithEnum(name string, enumValues []string) map[string]interface{} {
	iface := make([]interface{}, len(enumValues))
	for i, v := range enumValues {
		iface[i] = v
	}
	return map[string]interface{}{
		"name":     name,
		"in":       "query",
		"required": false,
		"schema": map[string]interface{}{
			"type": "string",
			"enum": iface,
		},
	}
}

// buildQueryParamWithoutEnum constructs a raw query parameter with no enum.
func buildQueryParamWithoutEnum(name string) map[string]interface{} {
	return map[string]interface{}{
		"name":     name,
		"in":       "query",
		"required": false,
		"schema": map[string]interface{}{
			"type": "string",
		},
	}
}

// TestEnumQueryParamExtracted verifies that ExtractParams populates Flag.Enum
// when the parameter schema has an enum array.
// FAILS until params.go buildFlag() reads the enum array.
func TestEnumQueryParamExtracted(t *testing.T) {
	params := []interface{}{
		buildQueryParamWithEnum("status", []string{"active", "inactive", "pending"}),
	}
	_, flags, err := model.ExtractParams(nil, params)
	require.NoError(t, err)
	require.Len(t, flags, 1)

	assert.Equal(t, "status", flags[0].Name)
	assert.Equal(t, []string{"active", "inactive", "pending"}, flags[0].Enum,
		"Flag.Enum should be populated from parameter schema enum values")
}

// TestEnumQueryParamNotSetWhenAbsent verifies that a Flag with no enum in its
// schema has an empty Enum field.
func TestEnumQueryParamNotSetWhenAbsent(t *testing.T) {
	params := []interface{}{
		buildQueryParamWithoutEnum("name"),
	}
	_, flags, err := model.ExtractParams(nil, params)
	require.NoError(t, err)
	require.Len(t, flags, 1)

	assert.Empty(t, flags[0].Enum,
		"Flag.Enum should be empty when parameter has no enum constraint")
}

// TestEnumQueryParamMultipleEnumsPreserveOrder verifies that enum values are
// preserved in their original order.
// FAILS until params.go buildFlag() reads the enum array.
func TestEnumQueryParamMultipleEnumsPreserveOrder(t *testing.T) {
	ordered := []string{"z", "a", "m", "b"}
	params := []interface{}{
		buildQueryParamWithEnum("sort", ordered),
	}
	_, flags, err := model.ExtractParams(nil, params)
	require.NoError(t, err)
	require.Len(t, flags, 1)

	assert.Equal(t, ordered, flags[0].Enum,
		"enum values should be preserved in original order")
}

// TestEnumBodyPropertyExtracted verifies that ExtractBodyFlags populates
// Flag.Enum when a body property schema has an enum array.
// FAILS until body.go extractFlagsFromSchema() reads the enum array.
func TestEnumBodyPropertyExtracted(t *testing.T) {
	rawBody := map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"role": map[string]interface{}{
							"type": "string",
							"enum": []interface{}{"admin", "editor", "viewer"},
						},
					},
				},
			},
		},
	}
	flags, err := model.ExtractBodyFlags(rawBody)
	require.NoError(t, err)

	var roleFlag *model.Flag
	for i := range flags {
		if flags[i].Name == "role" {
			roleFlag = &flags[i]
			break
		}
	}
	require.NotNil(t, roleFlag, "expected a 'role' flag from body schema, got: %+v", flags)
	assert.Equal(t, []string{"admin", "editor", "viewer"}, roleFlag.Enum,
		"Flag.Enum should be populated from body property enum values")
}

// TestEnumBodyPropertyNotSetWhenAbsent verifies that body flags without enum
// have empty Enum.
func TestEnumBodyPropertyNotSetWhenAbsent(t *testing.T) {
	rawBody := map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
	}
	flags, err := model.ExtractBodyFlags(rawBody)
	require.NoError(t, err)

	for _, f := range flags {
		assert.Empty(t, f.Enum,
			"flag %q should have empty Enum when no enum in schema", f.Name)
	}
}

// TestEnumFlagStructHasField verifies that the Flag struct exposes an Enum field
// by marshaling/unmarshaling a Flag with enum values set.
// FAILS until Flag struct has the Enum field added.
func TestEnumFlagStructHasField(t *testing.T) {
	f := model.Flag{
		Name:   "color",
		Type:   model.FlagTypeString,
		Source: model.FlagSourceQuery,
		Enum:   []string{"red", "green", "blue"},
	}

	data, err := json.Marshal(f)
	require.NoError(t, err)

	var decoded model.Flag
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, []string{"red", "green", "blue"}, decoded.Enum,
		"Flag.Enum should round-trip through JSON serialization")
}
