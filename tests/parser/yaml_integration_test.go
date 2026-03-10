// Package parser_test contains integration tests for YAML spec loading.
package parser_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/queso/swagger-jack/internal/model"
	"github.com/queso/swagger-jack/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// yamlIntegrationFixtureDir returns the testdata directory local to this package.
func yamlIntegrationFixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func loadYAMLAndBuild(t *testing.T, fixture string) ([]model.Resource, *parser.Result) {
	t.Helper()
	path := filepath.Join(yamlIntegrationFixtureDir(), fixture)
	result, err := parser.Load(path)
	require.NoError(t, err, "parser.Load(%q) should succeed", fixture)
	resources, err := model.Build(result)
	require.NoError(t, err, "model.Build should succeed for %q", fixture)
	return resources, result
}

// TestYAMLIntegration_TitleParsed verifies the spec title is correctly read from YAML.
func TestYAMLIntegration_TitleParsed(t *testing.T) {
	_, result := loadYAMLAndBuild(t, "petstore.yaml")
	assert.Equal(t, "Petstore YAML API", result.Spec.Title)
}

// TestYAMLIntegration_BaseURLParsed verifies the server URL is extracted from YAML.
func TestYAMLIntegration_BaseURLParsed(t *testing.T) {
	_, result := loadYAMLAndBuild(t, "petstore.yaml")
	assert.Equal(t, "https://petstore.example.com", result.Spec.BaseURL)
}

// TestYAMLIntegration_ResourcesBuilt verifies that resources are produced from the YAML spec.
func TestYAMLIntegration_ResourcesBuilt(t *testing.T) {
	resources, _ := loadYAMLAndBuild(t, "petstore.yaml")
	require.NotEmpty(t, resources, "YAML spec should produce at least one resource")

	var found bool
	for _, r := range resources {
		if r.Name == "pets" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'pets' resource from YAML spec")
}

// TestYAMLIntegration_CommandsBuilt verifies that commands are produced for the pets resource.
func TestYAMLIntegration_CommandsBuilt(t *testing.T) {
	resources, _ := loadYAMLAndBuild(t, "petstore.yaml")

	var petsResource *model.Resource
	for i := range resources {
		if resources[i].Name == "pets" {
			petsResource = &resources[i]
			break
		}
	}
	require.NotNil(t, petsResource, "pets resource should exist")
	assert.NotEmpty(t, petsResource.Commands, "pets resource should have commands")

	// Expect list, create, get, delete
	names := make(map[string]bool)
	for _, c := range petsResource.Commands {
		names[c.Name] = true
	}
	assert.True(t, names["list"], "expected 'list' command from GET /pets")
	assert.True(t, names["create"], "expected 'create' command from POST /pets")
}

// TestYAMLIntegration_RefsResolved verifies that $ref references are resolved in YAML specs.
func TestYAMLIntegration_RefsResolved(t *testing.T) {
	_, result := loadYAMLAndBuild(t, "petstore.yaml")
	assert.NotContains(t, string(result.RawJSON), `"$ref"`,
		"all $ref references should be resolved in the raw JSON")
}

// TestYAMLIntegration_EnumsExtracted verifies that enum values on query params are extracted.
func TestYAMLIntegration_EnumsExtracted(t *testing.T) {
	resources, _ := loadYAMLAndBuild(t, "petstore.yaml")

	var listCmd *model.Command
	for i := range resources {
		if resources[i].Name == "pets" {
			for j := range resources[i].Commands {
				if resources[i].Commands[j].Name == "list" {
					listCmd = &resources[i].Commands[j]
					break
				}
			}
		}
	}
	require.NotNil(t, listCmd, "expected 'list' command on pets resource")

	var statusFlag *model.Flag
	for i := range listCmd.Flags {
		if listCmd.Flags[i].Name == "status" {
			statusFlag = &listCmd.Flags[i]
			break
		}
	}
	require.NotNil(t, statusFlag, "expected 'status' flag on list command")
	assert.Equal(t, []string{"available", "pending", "sold"}, statusFlag.Enum,
		"enum values should be extracted from YAML query param schema")
}
