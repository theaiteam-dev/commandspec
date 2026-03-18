package model_test

import (
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeRequestBody builds a minimal OpenAPI requestBody map with an
// application/json schema containing the given properties and required list.
func makeRequestBody(properties map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": schema,
			},
		},
	}
}

// TestExtractBodyFlags_FlatObject verifies that a flat requestBody with string
// and integer properties produces two flags with the correct types and
// Source=FlagSourceBody.
func TestExtractBodyFlags_FlatObject(t *testing.T) {
	body := makeRequestBody(map[string]interface{}{
		"name": map[string]interface{}{"type": "string"},
		"age":  map[string]interface{}{"type": "integer"},
	}, nil)

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 2)

	byName := make(map[string]model.Flag, len(flags))
	for _, f := range flags {
		byName[f.Name] = f
	}

	nameFlag, ok := byName["name"]
	require.True(t, ok, "expected flag named 'name'")
	assert.Equal(t, model.FlagTypeString, nameFlag.Type, "name → FlagTypeString")
	assert.Equal(t, model.FlagSourceBody, nameFlag.Source, "name flag should have Source=FlagSourceBody")

	ageFlag, ok := byName["age"]
	require.True(t, ok, "expected flag named 'age'")
	assert.Equal(t, model.FlagTypeInt, ageFlag.Type, "age → FlagTypeInt")
	assert.Equal(t, model.FlagSourceBody, ageFlag.Source, "age flag should have Source=FlagSourceBody")
}

// TestExtractBodyFlags_NestedObject verifies that a nested object property
// produces dot-notation flags (e.g., address.city, address.zip).
func TestExtractBodyFlags_NestedObject(t *testing.T) {
	body := makeRequestBody(map[string]interface{}{
		"address": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"city": map[string]interface{}{"type": "string"},
				"zip":  map[string]interface{}{"type": "string"},
			},
		},
	}, nil)

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 2)

	byName := make(map[string]model.Flag, len(flags))
	for _, f := range flags {
		byName[f.Name] = f
	}

	_, ok := byName["address.city"]
	assert.True(t, ok, "expected flag named 'address.city'")
	_, ok = byName["address.zip"]
	assert.True(t, ok, "expected flag named 'address.zip'")
}

// TestExtractBodyFlags_RequiredFields verifies that properties listed in
// schema.required have Required=true, while others have Required=false.
func TestExtractBodyFlags_RequiredFields(t *testing.T) {
	body := makeRequestBody(map[string]interface{}{
		"email": map[string]interface{}{"type": "string"},
		"bio":   map[string]interface{}{"type": "string"},
	}, []string{"email"})

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 2)

	byName := make(map[string]model.Flag, len(flags))
	for _, f := range flags {
		byName[f.Name] = f
	}

	emailFlag, ok := byName["email"]
	require.True(t, ok, "expected flag named 'email'")
	assert.True(t, emailFlag.Required, "email should be required")

	bioFlag, ok := byName["bio"]
	require.True(t, ok, "expected flag named 'bio'")
	assert.False(t, bioFlag.Required, "bio should not be required")
}

// TestExtractBodyFlags_NilBody verifies that a nil requestBody returns an
// empty slice and no error.
func TestExtractBodyFlags_NilBody(t *testing.T) {
	flags, err := model.ExtractBodyFlags(nil)
	require.NoError(t, err)
	assert.Empty(t, flags)
}

// TestExtractBodyFlags_TypeMapping verifies that all four supported OpenAPI
// schema types (string, integer, boolean, array) map to the correct FlagType
// constants.
func TestExtractBodyFlags_TypeMapping(t *testing.T) {
	body := makeRequestBody(map[string]interface{}{
		"title":  map[string]interface{}{"type": "string"},
		"count":  map[string]interface{}{"type": "integer"},
		"active": map[string]interface{}{"type": "boolean"},
		"tags": map[string]interface{}{
			"type":  "array",
			"items": map[string]interface{}{"type": "string"},
		},
	}, nil)

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 4)

	byName := make(map[string]model.Flag, len(flags))
	for _, f := range flags {
		byName[f.Name] = f
	}

	assert.Equal(t, model.FlagTypeString, byName["title"].Type, "string → FlagTypeString")
	assert.Equal(t, model.FlagTypeInt, byName["count"].Type, "integer → FlagTypeInt")
	assert.Equal(t, model.FlagTypeBool, byName["active"].Type, "boolean → FlagTypeBool")
	assert.Equal(t, model.FlagTypeStringSlice, byName["tags"].Type, "array → FlagTypeStringSlice")
}

// TestExtractBodyFlags_NoSchema verifies that a requestBody with no
// application/json schema returns an empty slice and no error.
func TestExtractBodyFlags_NoSchema(t *testing.T) {
	bodyNoContent := map[string]interface{}{
		"required": true,
		"content":  map[string]interface{}{},
	}

	flags, err := model.ExtractBodyFlags(bodyNoContent)
	require.NoError(t, err)
	assert.Empty(t, flags, "no content → empty flags")

	bodyNoJSON := map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"text/plain": map[string]interface{}{
				"schema": map[string]interface{}{"type": "string"},
			},
		},
	}

	flags, err = model.ExtractBodyFlags(bodyNoJSON)
	require.NoError(t, err)
	assert.Empty(t, flags, "no application/json content → empty flags")
}
