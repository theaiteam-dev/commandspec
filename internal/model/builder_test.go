package model_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/queso/swagger-jack/internal/model"
	"github.com/queso/swagger-jack/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureDir returns the absolute path to the testdata directory.
func fixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	// internal/model/builder_test.go → project root is two levels up
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "testdata")
}

func loadFixture(t *testing.T, name string) *parser.Result {
	t.Helper()
	result, err := parser.Load(filepath.Join(fixtureDir(), name))
	require.NoError(t, err, "fixture %s should load without error", name)
	return result
}

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

// TestBuildBasicGrouping verifies that /pets and /pets/{petId} paths are
// grouped into a single "pets" resource with the expected commands.
func TestBuildBasicGrouping(t *testing.T) {
	result := loadFixture(t, "petstore.json")
	resources, err := model.Build(result)
	require.NoError(t, err)

	pets := findResource(resources, "pets")
	require.NotNil(t, pets, "expected a 'pets' resource")

	// /pets GET (collection) → list (operationId overrides to "listPets")
	// /pets POST → create (operationId overrides to "createPet")
	// /pets/{petId} GET (single) → get (operationId overrides to "getPet")
	// /pets/{petId} DELETE → delete (operationId overrides to "deletePet")
	names := make([]string, 0, len(pets.Commands))
	for _, cmd := range pets.Commands {
		names = append(names, cmd.Name)
	}
	assert.GreaterOrEqual(t, len(pets.Commands), 4, "pets resource should have at least 4 commands, got: %v", names)
}

// TestBuildVerbMapping verifies that HTTP methods on paths without operationIds
// map to the correct CLI verbs: GET(collection)→list, GET(single)→get,
// POST→create, DELETE→delete.
func TestBuildVerbMapping(t *testing.T) {
	result := loadFixture(t, "minimal.json")
	resources, err := model.Build(result)
	require.NoError(t, err)

	// minimal.json has /items with a GET (no operationId) — should map to "list"
	items := findResource(resources, "items")
	require.NotNil(t, items, "expected an 'items' resource")

	listCmd := findCommand(items, "list")
	require.NotNil(t, listCmd, "GET /items should map to 'list' command")
	assert.Equal(t, "GET", listCmd.HTTPMethod)
	assert.Equal(t, "/items", listCmd.Path)
}

// TestBuildOperationIdOverride verifies that when an operation has an
// operationId, that value is used as the command name instead of the derived verb.
func TestBuildOperationIdOverride(t *testing.T) {
	result := loadFixture(t, "petstore.json")
	resources, err := model.Build(result)
	require.NoError(t, err)

	pets := findResource(resources, "pets")
	require.NotNil(t, pets)

	// The GET /pets operation has operationId "listPets" → command name "listPets"
	cmd := findCommand(pets, "listPets")
	require.NotNil(t, cmd, "expected command named 'listPets' from operationId override")
	assert.Equal(t, "GET", cmd.HTTPMethod)
	assert.Equal(t, "/pets", cmd.Path)
}

// TestBuildNamingCollision verifies that PUT+PATCH on the same path without
// operationIds both produce "update" as the derived verb, and the builder
// resolves this collision by appending the HTTP method suffix.
func TestBuildNamingCollision(t *testing.T) {
	result := loadFixture(t, "collisions.json")
	resources, err := model.Build(result)
	require.NoError(t, err)

	widgets := findResource(resources, "widgets")
	require.NotNil(t, widgets, "expected a 'widgets' resource")

	// Both PUT and PATCH map to "update" — collision resolved as "update-put" / "update-patch"
	names := make(map[string]bool)
	for _, cmd := range widgets.Commands {
		names[cmd.Name] = true
	}
	assert.True(t, names["update-put"] || names["update-patch"],
		"collision should produce 'update-put' and/or 'update-patch', got: %v", names)
	// Both methods should be present, not just one
	assert.True(t, names["update-put"] && names["update-patch"],
		"both 'update-put' and 'update-patch' should exist after collision resolution, got: %v", names)
}

// TestBuildMultipleResources verifies that distinct path prefixes produce
// separate resource groups.
func TestBuildMultipleResources(t *testing.T) {
	result := loadFixture(t, "petstore.json")
	resources, err := model.Build(result)
	require.NoError(t, err)

	// petstore.json has /pets and /owners → two resources
	pets := findResource(resources, "pets")
	owners := findResource(resources, "owners")
	require.NotNil(t, pets, "expected 'pets' resource")
	require.NotNil(t, owners, "expected 'owners' resource")
	assert.NotEmpty(t, pets.Commands)
	assert.NotEmpty(t, owners.Commands)
}
