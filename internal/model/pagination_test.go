package model_test

import (
	"encoding/json"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSpecFromJSON is a helper that wraps raw OpenAPI JSON into a Result-like
// object accepted by model.Build, reusing the parser.Result type via JSON round-trip.
// Because tests in this package use parser.Load for fixtures, we define a minimal
// inline spec builder for pagination-specific test specs.

// paginationSpec builds a minimal OpenAPI JSON spec with a single GET /items
// endpoint that has the given query parameter names.
func paginationSpec(queryParams []string) []byte {
	parameters := make([]map[string]interface{}, 0, len(queryParams))
	for _, name := range queryParams {
		parameters = append(parameters, map[string]interface{}{
			"name":   name,
			"in":     "query",
			"schema": map[string]interface{}{"type": "integer"},
		})
	}

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Pagination Test",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/items": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":    "List items",
					"parameters": parameters,
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "OK"},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(spec)
	return data
}

// loadSpecFromBytes wraps raw JSON bytes into a parser.Result so model.Build can consume it.
// This is necessary because model.Build requires a RawJSONProvider.
func loadSpecFromBytes(t *testing.T, data []byte) interface{ GetRawJSON() []byte } {
	t.Helper()
	return &rawJSONResult{data: data}
}

// rawJSONResult is a minimal SpecProvider + RawJSONProvider that wraps raw bytes.
type rawJSONResult struct {
	data []byte
}

func (r *rawJSONResult) GetRawJSON() []byte { return r.data }

// GetSpec is required to satisfy model.SpecProvider if defined.
func (r *rawJSONResult) GetSpec() *model.APISpec { return &model.APISpec{} }

// findListCommand returns the "list" command from the "items" resource, or fails.
func findListCommand(t *testing.T, resources []model.Resource) *model.Command {
	t.Helper()
	for i := range resources {
		if resources[i].Name == "items" {
			for j := range resources[i].Commands {
				if resources[i].Commands[j].Name == "list" {
					return &resources[i].Commands[j]
				}
			}
		}
	}
	t.Fatal("expected 'items' resource with 'list' command")
	return nil
}

// --- Pagination struct and type constant tests ---

// TestPaginationTypeConstants verifies that PaginationType constants are defined.
func TestPaginationTypeConstants(t *testing.T) {
	assert.Equal(t, model.PaginationType("page"), model.PaginationPageBased)
	assert.Equal(t, model.PaginationType("offset"), model.PaginationOffsetBased)
	assert.Equal(t, model.PaginationType("cursor"), model.PaginationCursorBased)
}

// TestPaginationStructFields verifies that the Pagination struct has the expected fields.
func TestPaginationStructFields(t *testing.T) {
	p := model.Pagination{
		Type:        model.PaginationPageBased,
		PageParam:   "page",
		SizeParam:   "per_page",
		CursorParam: "",
	}
	assert.Equal(t, model.PaginationPageBased, p.Type)
	assert.Equal(t, "page", p.PageParam)
	assert.Equal(t, "per_page", p.SizeParam)
	assert.Empty(t, p.CursorParam)
}

// TestCommandHasPaginationField verifies that Command has a Pagination field.
func TestCommandHasPaginationField(t *testing.T) {
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
	}
	// Initially nil — no pagination detected.
	assert.Nil(t, cmd.Pagination)

	// Can be set.
	cmd.Pagination = &model.Pagination{
		Type:      model.PaginationOffsetBased,
		SizeParam: "limit",
		PageParam: "offset",
	}
	require.NotNil(t, cmd.Pagination)
	assert.Equal(t, model.PaginationOffsetBased, cmd.Pagination.Type)
}

// --- Builder detection tests ---

// TestBuildDetectsPageBasedPagination verifies detection of page/per_page params.
func TestBuildDetectsPageBasedPagination(t *testing.T) {
	data := paginationSpec([]string{"page", "per_page"})
	result := loadSpecFromBytes(t, data)
	resources, err := model.Build(result)
	require.NoError(t, err)

	cmd := findListCommand(t, resources)
	require.NotNil(t, cmd.Pagination, "page+per_page params should set Pagination")
	assert.Equal(t, model.PaginationPageBased, cmd.Pagination.Type)
	assert.Equal(t, "page", cmd.Pagination.PageParam)
	assert.Equal(t, "per_page", cmd.Pagination.SizeParam)
}

// TestBuildDetectsPageBasedPaginationCamelCase verifies detection of page/perPage (camelCase).
func TestBuildDetectsPageBasedPaginationCamelCase(t *testing.T) {
	data := paginationSpec([]string{"page", "perPage"})
	result := loadSpecFromBytes(t, data)
	resources, err := model.Build(result)
	require.NoError(t, err)

	cmd := findListCommand(t, resources)
	require.NotNil(t, cmd.Pagination, "page+perPage params should set Pagination")
	assert.Equal(t, model.PaginationPageBased, cmd.Pagination.Type)
}

// TestBuildDetectsPageBasedPaginationPageSize verifies detection of page/page_size.
func TestBuildDetectsPageBasedPaginationPageSize(t *testing.T) {
	data := paginationSpec([]string{"page", "page_size"})
	result := loadSpecFromBytes(t, data)
	resources, err := model.Build(result)
	require.NoError(t, err)

	cmd := findListCommand(t, resources)
	require.NotNil(t, cmd.Pagination, "page+page_size params should set Pagination")
	assert.Equal(t, model.PaginationPageBased, cmd.Pagination.Type)
}

// TestBuildDetectsOffsetBasedPagination verifies detection of limit/offset params.
func TestBuildDetectsOffsetBasedPagination(t *testing.T) {
	data := paginationSpec([]string{"limit", "offset"})
	result := loadSpecFromBytes(t, data)
	resources, err := model.Build(result)
	require.NoError(t, err)

	cmd := findListCommand(t, resources)
	require.NotNil(t, cmd.Pagination, "limit+offset params should set Pagination")
	assert.Equal(t, model.PaginationOffsetBased, cmd.Pagination.Type)
	assert.Equal(t, "limit", cmd.Pagination.SizeParam)
	assert.Equal(t, "offset", cmd.Pagination.PageParam)
}

// TestBuildDetectsCursorBasedPaginationCursor verifies detection of cursor param.
func TestBuildDetectsCursorBasedPaginationCursor(t *testing.T) {
	data := paginationSpec([]string{"cursor", "limit"})
	result := loadSpecFromBytes(t, data)
	resources, err := model.Build(result)
	require.NoError(t, err)

	cmd := findListCommand(t, resources)
	require.NotNil(t, cmd.Pagination, "cursor param should set Pagination")
	assert.Equal(t, model.PaginationCursorBased, cmd.Pagination.Type)
	assert.Equal(t, "cursor", cmd.Pagination.CursorParam)
}

// TestBuildDetectsCursorBasedPaginationAfter verifies detection of after param.
func TestBuildDetectsCursorBasedPaginationAfter(t *testing.T) {
	data := paginationSpec([]string{"after", "limit"})
	result := loadSpecFromBytes(t, data)
	resources, err := model.Build(result)
	require.NoError(t, err)

	cmd := findListCommand(t, resources)
	require.NotNil(t, cmd.Pagination, "after param should set Pagination")
	assert.Equal(t, model.PaginationCursorBased, cmd.Pagination.Type)
	assert.Equal(t, "after", cmd.Pagination.CursorParam)
}

// TestBuildDetectsCursorBasedPaginationBefore verifies detection of before param.
func TestBuildDetectsCursorBasedPaginationBefore(t *testing.T) {
	data := paginationSpec([]string{"before", "limit"})
	result := loadSpecFromBytes(t, data)
	resources, err := model.Build(result)
	require.NoError(t, err)

	cmd := findListCommand(t, resources)
	require.NotNil(t, cmd.Pagination, "before param should set Pagination")
	assert.Equal(t, model.PaginationCursorBased, cmd.Pagination.Type)
	assert.Equal(t, "before", cmd.Pagination.CursorParam)
}

// TestBuildNoPaginationForNonPaginatedEndpoint verifies nil Pagination for plain endpoints.
func TestBuildNoPaginationForNonPaginatedEndpoint(t *testing.T) {
	data := paginationSpec([]string{"filter", "sort"})
	result := loadSpecFromBytes(t, data)
	resources, err := model.Build(result)
	require.NoError(t, err)

	cmd := findListCommand(t, resources)
	assert.Nil(t, cmd.Pagination, "non-pagination query params should not set Pagination")
}

// TestBuildNoPaginationForEndpointWithNoParams verifies nil Pagination when no params.
func TestBuildNoPaginationForEndpointWithNoParams(t *testing.T) {
	data := paginationSpec([]string{})
	result := loadSpecFromBytes(t, data)
	resources, err := model.Build(result)
	require.NoError(t, err)

	cmd := findListCommand(t, resources)
	assert.Nil(t, cmd.Pagination, "endpoint with no params should have nil Pagination")
}

// TestBuildPaginationDoesNotBreakExistingFixtures verifies that adding pagination
// detection does not break any existing fixture-based tests.
func TestBuildPaginationDoesNotBreakExistingFixtures(t *testing.T) {
	for _, fixture := range []string{"minimal.json", "petstore.json", "collisions.json", "apikey_auth.json", "basic_auth.json"} {
		t.Run(fixture, func(t *testing.T) {
			result := loadFixture(t, fixture)
			resources, err := model.Build(result)
			require.NoError(t, err, "fixture %s should build without error", fixture)
			assert.NotEmpty(t, resources, "fixture %s should produce at least one resource", fixture)
		})
	}
}
