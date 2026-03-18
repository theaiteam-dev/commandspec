// Package generator_test contains integration tests for --body/--body-file flag generation.
package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bodySpec returns an APISpec with POST, PUT, PATCH, GET, and DELETE endpoints.
func bodySpec() *model.APISpec {
	return &model.APISpec{
		Title:   "Body Flags Integration API",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
		Resources: []model.Resource{
			{
				Name: "items",
				Commands: []model.Command{
					{
						Name:       "list",
						HTTPMethod: "GET",
						Path:       "/items",
					},
					{
						Name:       "create",
						HTTPMethod: "POST",
						Path:       "/items",
						Flags: []model.Flag{
							{Name: "name", Type: model.FlagTypeString, Required: true, Source: model.FlagSourceBody},
						},
					},
					{
						Name:       "update",
						HTTPMethod: "PUT",
						Path:       "/items/{id}",
						Args:       []model.Arg{{Name: "id", Required: true}},
						Flags: []model.Flag{
							{Name: "name", Type: model.FlagTypeString, Required: false, Source: model.FlagSourceBody},
						},
					},
					{
						Name:       "patch",
						HTTPMethod: "PATCH",
						Path:       "/items/{id}",
						Args:       []model.Arg{{Name: "id", Required: true}},
						Flags: []model.Flag{
							{Name: "name", Type: model.FlagTypeString, Required: false, Source: model.FlagSourceBody},
						},
					},
					{
						Name:       "delete",
						HTTPMethod: "DELETE",
						Path:       "/items/{id}",
						Args:       []model.Arg{{Name: "id", Required: true}},
					},
				},
			},
		},
	}
}

// readGeneratedCmd reads the generated verb command file for the given verb.
func readGeneratedCmd(t *testing.T, dir, verb string) string {
	t.Helper()
	path := filepath.Join(dir, "cmd", "items_"+verb+".go")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "cmd/items_%s.go should exist", verb)
	return string(data)
}

// TestBodyIntegration_PostHasBodyFlag verifies the POST command has --body flag.
func TestBodyIntegration_PostHasBodyFlag(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(bodySpec(), "bodyapi", dir)
	require.NoError(t, err, "Generate should succeed")

	src := readGeneratedCmd(t, dir, "create")
	assert.Contains(t, src, `"body"`,
		"generated POST command should register --body flag")
}

// TestBodyIntegration_PostHasBodyFileFlag verifies the POST command has --body-file flag.
func TestBodyIntegration_PostHasBodyFileFlag(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(bodySpec(), "bodyapi", dir)
	require.NoError(t, err, "Generate should succeed")

	src := readGeneratedCmd(t, dir, "create")
	assert.Contains(t, src, "body-file",
		"generated POST command should register --body-file flag")
}

// TestBodyIntegration_PutHasBodyFlag verifies the PUT command has --body flag.
func TestBodyIntegration_PutHasBodyFlag(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(bodySpec(), "bodyapi", dir)
	require.NoError(t, err, "Generate should succeed")

	src := readGeneratedCmd(t, dir, "update")
	assert.Contains(t, src, `"body"`,
		"generated PUT command should register --body flag")
}

// TestBodyIntegration_PatchHasBodyFlag verifies the PATCH command has --body flag.
func TestBodyIntegration_PatchHasBodyFlag(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(bodySpec(), "bodyapi", dir)
	require.NoError(t, err, "Generate should succeed")

	src := readGeneratedCmd(t, dir, "patch")
	assert.Contains(t, src, `"body"`,
		"generated PATCH command should register --body flag")
}

// TestBodyIntegration_GetNoBodyFlag verifies GET commands do NOT have --body flags.
func TestBodyIntegration_GetNoBodyFlag(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(bodySpec(), "bodyapi", dir)
	require.NoError(t, err, "Generate should succeed")

	src := readGeneratedCmd(t, dir, "list")
	assert.NotContains(t, src, `StringVar(&body,`,
		"generated GET command should NOT register --body flag")
	assert.NotContains(t, src, `StringVar(&bodyFile,`,
		"generated GET command should NOT register --body-file flag")
}

// TestBodyIntegration_DeleteNoBodyFlag verifies DELETE commands do NOT have --body flags.
func TestBodyIntegration_DeleteNoBodyFlag(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(bodySpec(), "bodyapi", dir)
	require.NoError(t, err, "Generate should succeed")

	src := readGeneratedCmd(t, dir, "delete")
	assert.NotContains(t, src, `StringVar(&body,`,
		"generated DELETE command should NOT register --body flag")
}

// TestBodyIntegration_BodyOverrideLogic verifies that write command RunE
// checks --body before using individual flags.
func TestBodyIntegration_BodyOverrideLogic(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(bodySpec(), "bodyapi", dir)
	require.NoError(t, err, "Generate should succeed")

	src := readGeneratedCmd(t, dir, "create")
	hasBodyCheck := strings.Contains(src, `Body != ""`)
	assert.True(t, hasBodyCheck,
		"generated POST command should check --body before individual flags")
}

// TestBodyIntegration_BodyFileReadLogic verifies that write command RunE
// contains file-reading logic for --body-file.
func TestBodyIntegration_BodyFileReadLogic(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(bodySpec(), "bodyapi", dir)
	require.NoError(t, err, "Generate should succeed")

	src := readGeneratedCmd(t, dir, "create")
	hasFileRead := strings.Contains(src, "ReadFile") || strings.Contains(src, "bodyFile")
	assert.True(t, hasFileRead,
		"generated POST command should contain file-reading logic for --body-file")
}

// TestBodyIntegration_JSONValidation verifies that write command RunE validates JSON.
func TestBodyIntegration_JSONValidation(t *testing.T) {
	dir := t.TempDir()
	err := generator.Generate(bodySpec(), "bodyapi", dir)
	require.NoError(t, err, "Generate should succeed")

	src := readGeneratedCmd(t, dir, "create")
	hasValidation := strings.Contains(src, "json.Valid") || strings.Contains(src, "json.Unmarshal")
	assert.True(t, hasValidation,
		"generated POST command should validate --body JSON")
}
