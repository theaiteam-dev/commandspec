package model_test

import (
	"encoding/json"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPISpecConstruction verifies that APISpec and nested types can be
// constructed and that exported fields are accessible.
func TestAPISpecConstruction(t *testing.T) {
	spec := model.APISpec{
		Title:       "Test API",
		Version:     "1.0.0",
		Description: "A test API",
		BaseURL:     "https://api.example.com",
		Resources: []model.Resource{
			{
				Name:        "users",
				Description: "User management",
				Commands: []model.Command{
					{
						Name:        "list",
						HTTPMethod:  "GET",
						Path:        "/users",
						Description: "List all users",
						Flags: []model.Flag{
							{
								Name:        "limit",
								Type:        model.FlagTypeInt,
								Required:    false,
								Default:     "20",
								Description: "Max results",
								Source:      model.FlagSourceQuery,
							},
						},
					},
					{
						Name:        "get",
						HTTPMethod:  "GET",
						Path:        "/users/{userId}",
						Description: "Get a user by ID",
						Args: []model.Arg{
							{
								Name:        "user-id",
								Description: "The user ID",
								Required:    true,
							},
						},
					},
				},
			},
		},
		SecuritySchemes: map[string]model.SecurityScheme{
			"bearerAuth": {
				Type:   model.SecuritySchemeBearer,
				EnvVar: "API_TOKEN",
			},
		},
	}

	assert.Equal(t, "Test API", spec.Title)
	assert.Equal(t, "1.0.0", spec.Version)
	assert.Len(t, spec.Resources, 1)
	assert.Equal(t, "users", spec.Resources[0].Name)
	assert.Len(t, spec.Resources[0].Commands, 2)

	listCmd := spec.Resources[0].Commands[0]
	assert.Equal(t, "list", listCmd.Name)
	assert.Equal(t, "GET", listCmd.HTTPMethod)
	assert.Len(t, listCmd.Flags, 1)
	assert.Equal(t, model.FlagTypeInt, listCmd.Flags[0].Type)
	assert.Equal(t, model.FlagSourceQuery, listCmd.Flags[0].Source)

	getCmd := spec.Resources[0].Commands[1]
	assert.Len(t, getCmd.Args, 1)
	assert.True(t, getCmd.Args[0].Required)

	scheme, ok := spec.SecuritySchemes["bearerAuth"]
	require.True(t, ok)
	assert.Equal(t, model.SecuritySchemeBearer, scheme.Type)
	assert.Equal(t, "API_TOKEN", scheme.EnvVar)
}

// TestAPISpecJSONSerialization verifies that APISpec serializes to JSON
// with correct field names (snake_case via struct tags).
func TestAPISpecJSONSerialization(t *testing.T) {
	spec := model.APISpec{
		Title:   "My API",
		Version: "2.0.0",
		BaseURL: "https://example.com",
		Resources: []model.Resource{
			{
				Name: "items",
				Commands: []model.Command{
					{
						Name:       "create",
						HTTPMethod: "POST",
						Path:       "/items",
						RequestBody: &model.RequestBody{
							ContentType: "application/json",
							Required:    true,
							Schema: []model.SchemaField{
								{
									Name:     "name",
									Type:     model.FlagTypeString,
									Required: true,
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(spec)
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &out))

	assert.Equal(t, "My API", out["title"])
	assert.Equal(t, "2.0.0", out["version"])
	assert.Equal(t, "https://example.com", out["base_url"])

	resources, ok := out["resources"].([]interface{})
	require.True(t, ok)
	require.Len(t, resources, 1)

	resource := resources[0].(map[string]interface{})
	assert.Equal(t, "items", resource["name"])

	commands := resource["commands"].([]interface{})
	require.Len(t, commands, 1)

	cmd := commands[0].(map[string]interface{})
	assert.Equal(t, "create", cmd["name"])
	assert.Equal(t, "POST", cmd["http_method"])

	body := cmd["request_body"].(map[string]interface{})
	assert.Equal(t, "application/json", body["content_type"])
	assert.Equal(t, true, body["required"])
}

// TestRequestBodySchema verifies RequestBody schema field construction.
func TestRequestBodySchema(t *testing.T) {
	rb := model.RequestBody{
		ContentType: "application/json",
		Required:    true,
		Schema: []model.SchemaField{
			{Name: "email", Type: model.FlagTypeString, Required: true},
			{Name: "age", Type: model.FlagTypeInt, Required: false},
			{Name: "tags", Type: model.FlagTypeStringSlice, Required: false},
			{Name: "active", Type: model.FlagTypeBool, Required: false, Description: "Is active"},
		},
	}

	assert.Len(t, rb.Schema, 4)
	assert.Equal(t, model.FlagTypeString, rb.Schema[0].Type)
	assert.Equal(t, model.FlagTypeInt, rb.Schema[1].Type)
	assert.Equal(t, model.FlagTypeStringSlice, rb.Schema[2].Type)
	assert.Equal(t, model.FlagTypeBool, rb.Schema[3].Type)
	assert.Equal(t, "Is active", rb.Schema[3].Description)
}
