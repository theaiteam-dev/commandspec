package model_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/theaiteam-dev/commandspec/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureDir returns the absolute path to the top-level testdata directory.
func fixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	// tests/model/builder_params_test.go → project root is three levels up
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "testdata")
}

func loadFixture(t *testing.T, name string) *parser.Result {
	t.Helper()
	result, err := parser.Load(filepath.Join(fixtureDir(), name))
	require.NoError(t, err, "fixture %s should load without error", name)
	return result
}

// inlineSpec wraps raw JSON bytes so Build() can consume them.
// It satisfies both model.SpecProvider and model.RawJSONProvider.
type inlineSpec struct {
	data []byte
}

func (s *inlineSpec) GetRawJSON() []byte      { return s.data }
func (s *inlineSpec) GetSpec() *model.APISpec { return &model.APISpec{} }

// findResource returns the resource with the given name, or nil.
func findResource(resources []model.Resource, name string) *model.Resource {
	for i := range resources {
		if resources[i].Name == name {
			return &resources[i]
		}
	}
	return nil
}

// findCommand returns the command with the given name from a resource, or nil.
func findCommand(resource *model.Resource, name string) *model.Command {
	for i := range resource.Commands {
		if resource.Commands[i].Name == name {
			return &resource.Commands[i]
		}
	}
	return nil
}

// pathParamsSpec is a minimal OpenAPI 3 doc with a single path parameter.
var pathParamsSpec = []byte(`{
  "openapi": "3.0.3",
  "info": { "title": "Test", "version": "1.0.0" },
  "paths": {
    "/pets/{petId}": {
      "get": {
        "operationId": "getPet",
        "summary": "Get a pet",
        "parameters": [
          {
            "name": "petId",
            "in": "path",
            "required": true,
            "schema": { "type": "integer" }
          }
        ],
        "responses": { "200": { "description": "ok" } }
      }
    }
  }
}`)

// queryParamsSpec has two query params: limit (int) and tags (array).
var queryParamsSpec = []byte(`{
  "openapi": "3.0.3",
  "info": { "title": "Test", "version": "1.0.0" },
  "paths": {
    "/pets": {
      "get": {
        "operationId": "listPets",
        "summary": "List pets",
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "schema": { "type": "integer" }
          },
          {
            "name": "tags",
            "in": "query",
            "schema": { "type": "array", "items": { "type": "string" } }
          }
        ],
        "responses": { "200": { "description": "ok" } }
      }
    }
  }
}`)

// bodyParamsSpec has a POST with a requestBody containing name (required) and tag.
var bodyParamsSpec = []byte(`{
  "openapi": "3.0.3",
  "info": { "title": "Test", "version": "1.0.0" },
  "paths": {
    "/pets": {
      "post": {
        "operationId": "createPet",
        "summary": "Create a pet",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "name": { "type": "string" },
                  "tag":  { "type": "string" }
                },
                "required": ["name"]
              }
            }
          }
        },
        "responses": { "201": { "description": "created" } }
      }
    }
  }
}`)

// TestBuildPopulatesArgsFromPathParams verifies that Build() populates Command.Args
// from OpenAPI path parameters, converting camelCase names to kebab-case.
// This test FAILS against the current Build() (which leaves Args empty) and should
// PASS once Build() calls ExtractParams.
func TestBuildPopulatesArgsFromPathParams(t *testing.T) {
	spec := &inlineSpec{data: pathParamsSpec}
	resources, err := model.Build(spec)
	require.NoError(t, err)

	pets := findResource(resources, "pets")
	require.NotNil(t, pets, "expected 'pets' resource")

	cmd := findCommand(pets, "getPet")
	require.NotNil(t, cmd, "expected 'getPet' command")

	require.NotEmpty(t, cmd.Args, "Build() should populate Args from path parameters; got empty slice")

	found := false
	for _, arg := range cmd.Args {
		if arg.Name == "pet-id" {
			found = true
			assert.True(t, arg.Required, "path param arg should be required")
		}
	}
	assert.True(t, found, "expected Arg with Name='pet-id' (camelCase→kebab-case), got: %+v", cmd.Args)
}

// TestBuildPopulatesQueryFlags verifies that Build() populates Command.Flags
// from OpenAPI query parameters with Source=FlagSourceQuery.
// This test FAILS against the current Build() (which leaves Flags empty) and should
// PASS once Build() calls ExtractParams.
func TestBuildPopulatesQueryFlags(t *testing.T) {
	spec := &inlineSpec{data: queryParamsSpec}
	resources, err := model.Build(spec)
	require.NoError(t, err)

	pets := findResource(resources, "pets")
	require.NotNil(t, pets, "expected 'pets' resource")

	cmd := findCommand(pets, "listPets")
	require.NotNil(t, cmd, "expected 'listPets' command")

	require.NotEmpty(t, cmd.Flags, "Build() should populate Flags from query parameters; got empty slice")

	flagsByName := make(map[string]model.Flag)
	for _, f := range cmd.Flags {
		flagsByName[f.Name] = f
	}

	limitFlag, ok := flagsByName["limit"]
	require.True(t, ok, "expected a 'limit' flag, got flags: %+v", cmd.Flags)
	assert.Equal(t, model.FlagSourceQuery, limitFlag.Source, "'limit' flag should have Source=query")
	assert.Equal(t, model.FlagTypeInt, limitFlag.Type, "'limit' flag should have Type=int")

	tagsFlag, ok := flagsByName["tags"]
	require.True(t, ok, "expected a 'tags' flag, got flags: %+v", cmd.Flags)
	assert.Equal(t, model.FlagSourceQuery, tagsFlag.Source, "'tags' flag should have Source=query")
	assert.Equal(t, model.FlagTypeStringSlice, tagsFlag.Type, "'tags' flag should have Type=[]string")
}

// TestBuildPopulatesBodyFlags verifies that Build() populates Command.Flags
// from a requestBody schema with Source=FlagSourceBody.
// This test FAILS against the current Build() (which leaves Flags empty) and should
// PASS once Build() calls ExtractBodyFlags.
func TestBuildPopulatesBodyFlags(t *testing.T) {
	spec := &inlineSpec{data: bodyParamsSpec}
	resources, err := model.Build(spec)
	require.NoError(t, err)

	pets := findResource(resources, "pets")
	require.NotNil(t, pets, "expected 'pets' resource")

	cmd := findCommand(pets, "createPet")
	require.NotNil(t, cmd, "expected 'createPet' command")

	require.NotEmpty(t, cmd.Flags, "Build() should populate Flags from requestBody schema; got empty slice")

	bodyFlags := make(map[string]model.Flag)
	for _, f := range cmd.Flags {
		if f.Source == model.FlagSourceBody {
			bodyFlags[f.Name] = f
		}
	}

	require.NotEmpty(t, bodyFlags, "expected at least one flag with Source=FlagSourceBody, got flags: %+v", cmd.Flags)

	nameFlag, ok := bodyFlags["name"]
	require.True(t, ok, "expected a 'name' body flag, got body flags: %+v", bodyFlags)
	assert.Equal(t, model.FlagTypeString, nameFlag.Type, "'name' flag should have Type=string")
	assert.True(t, nameFlag.Required, "'name' is required per schema")

	tagFlag, ok := bodyFlags["tag"]
	require.True(t, ok, "expected a 'tag' body flag, got body flags: %+v", bodyFlags)
	assert.Equal(t, model.FlagTypeString, tagFlag.Type, "'tag' flag should have Type=string")
	assert.False(t, tagFlag.Required, "'tag' is not required per schema")
}

// TestBuildExistingBehaviourPreserved verifies that the existing collision
// resolution and operationId override behaviour is unaffected by the params wiring.
// Uses the collisions.json fixture (PUT+PATCH on same path → update-put / update-patch).
func TestBuildExistingBehaviourPreserved(t *testing.T) {
	result := loadFixture(t, "collisions.json")
	resources, err := model.Build(result)
	require.NoError(t, err)

	widgets := findResource(resources, "widgets")
	require.NotNil(t, widgets, "expected a 'widgets' resource")

	names := make(map[string]bool)
	for _, cmd := range widgets.Commands {
		names[cmd.Name] = true
	}
	assert.True(t, names["update-put"] && names["update-patch"],
		"collision resolution should still produce 'update-put' and 'update-patch', got: %v", names)
}
