// Package parser_test contains integration tests that load OpenAPI fixtures
// through parser.Load then model.Build and assert on the resulting model.
// These tests exercise the full Parse → Model pipeline.
package parser_test

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
	// tests/parser/parser_test.go → project root is three levels up
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "testdata")
}

func loadAndBuild(t *testing.T, fixture string) []model.Resource {
	t.Helper()
	result, err := parser.Load(filepath.Join(fixtureDir(), fixture))
	require.NoError(t, err, "parser.Load(%q) should succeed", fixture)
	resources, err := model.Build(result)
	require.NoError(t, err, "model.Build should succeed for %q", fixture)
	return resources
}

// findResource returns the named resource or nil.
func findResource(resources []model.Resource, name string) *model.Resource {
	for i := range resources {
		if resources[i].Name == name {
			return &resources[i]
		}
	}
	return nil
}

// findCommand returns the named command within a resource or nil.
func findCommand(res *model.Resource, name string) *model.Command {
	for i := range res.Commands {
		if res.Commands[i].Name == name {
			return &res.Commands[i]
		}
	}
	return nil
}

// TestParseAndBuildPetstoreArgs verifies that loading petstore.json and
// building the model produces a GET /pets/{petId} command with a "pet-id" arg.
// FAILS until Build() calls ExtractParams (WI-487).
func TestParseAndBuildPetstoreArgs(t *testing.T) {
	resources := loadAndBuild(t, "petstore.json")

	pets := findResource(resources, "pets")
	require.NotNil(t, pets, "expected 'pets' resource")

	// operationId "getPet" → command name "getPet"
	cmd := findCommand(pets, "getPet")
	require.NotNil(t, cmd, "expected 'getPet' command from GET /pets/{petId}")

	require.NotEmpty(t, cmd.Args,
		"getPet command should have Args populated from {petId} path param")

	found := false
	for _, arg := range cmd.Args {
		if arg.Name == "pet-id" {
			found = true
			assert.True(t, arg.Required, "path param should be required")
		}
	}
	assert.True(t, found,
		"expected Arg Name='pet-id' (camelCase→kebab), got: %+v", cmd.Args)
}

// TestParseAndBuildPetstoreQueryFlags verifies that loading petstore.json and
// building the model produces a GET /pets command with a "limit" flag of type
// int sourced from query.
// FAILS until Build() calls ExtractParams (WI-487).
func TestParseAndBuildPetstoreQueryFlags(t *testing.T) {
	resources := loadAndBuild(t, "petstore.json")

	pets := findResource(resources, "pets")
	require.NotNil(t, pets, "expected 'pets' resource")

	// operationId "listPets" → command name "listPets"
	cmd := findCommand(pets, "listPets")
	require.NotNil(t, cmd, "expected 'listPets' command from GET /pets")

	require.NotEmpty(t, cmd.Flags,
		"listPets command should have Flags populated from query params")

	var limitFlag *model.Flag
	for i := range cmd.Flags {
		if cmd.Flags[i].Name == "limit" {
			limitFlag = &cmd.Flags[i]
			break
		}
	}
	require.NotNil(t, limitFlag, "expected a 'limit' flag, got: %+v", cmd.Flags)
	assert.Equal(t, model.FlagTypeInt, limitFlag.Type, "'limit' should be int type")
	assert.Equal(t, model.FlagSourceQuery, limitFlag.Source, "'limit' should have Source=query")
}

// TestParseAndBuildBodyFlags verifies that loading collisions.json and
// building the model produces a PUT /widgets/{id} command with a body flag
// sourced from the requestBody schema.
// FAILS until Build() calls ExtractBodyFlags (WI-487).
func TestParseAndBuildBodyFlags(t *testing.T) {
	resources := loadAndBuild(t, "collisions.json")

	widgets := findResource(resources, "widgets")
	require.NotNil(t, widgets, "expected 'widgets' resource")

	// The PUT on /widgets/{id} has no operationId so its name is "update-put"
	// after collision resolution.
	cmd := findCommand(widgets, "update-put")
	require.NotNil(t, cmd, "expected 'update-put' command (PUT /widgets/{id}), got commands: %+v", widgets.Commands)

	require.NotEmpty(t, cmd.Flags,
		"update-put command should have Flags from requestBody schema")

	hasBodyFlag := false
	for _, f := range cmd.Flags {
		if f.Source == model.FlagSourceBody {
			hasBodyFlag = true
			break
		}
	}
	assert.True(t, hasBodyFlag,
		"expected at least one flag with Source=FlagSourceBody, got: %+v", cmd.Flags)
}

// TestParseAndBuildNoParams verifies that loading minimal.json (which has no
// parameters) produces commands with empty Args and Flags.
func TestParseAndBuildNoParams(t *testing.T) {
	resources := loadAndBuild(t, "minimal.json")

	items := findResource(resources, "items")
	require.NotNil(t, items, "expected 'items' resource")

	require.NotEmpty(t, items.Commands, "expected at least one command in 'items' resource")

	for _, cmd := range items.Commands {
		assert.Empty(t, cmd.Args,
			"command %q in minimal.json should have no Args", cmd.Name)
		assert.Empty(t, cmd.Flags,
			"command %q in minimal.json should have no Flags", cmd.Name)
	}
}

// TestParseAndBuildMultipleResources verifies that loading petstore.json
// produces both 'pets' and 'owners' resource groups.
func TestParseAndBuildMultipleResources(t *testing.T) {
	resources := loadAndBuild(t, "petstore.json")

	pets := findResource(resources, "pets")
	owners := findResource(resources, "owners")

	require.NotNil(t, pets, "expected 'pets' resource")
	require.NotNil(t, owners, "expected 'owners' resource")

	assert.NotEmpty(t, pets.Commands, "'pets' resource should have commands")
	assert.NotEmpty(t, owners.Commands, "'owners' resource should have commands")
}
