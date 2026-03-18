package model_test

import (
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeMultipartRequestBody builds a minimal OpenAPI requestBody map with a
// multipart/form-data schema containing the given properties.
func makeMultipartRequestBody(properties map[string]interface{}, required []string) map[string]interface{} {
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
			"multipart/form-data": map[string]interface{}{
				"schema": schema,
			},
		},
	}
}

// --- FlagTypeFile constant ---

// TestFlagTypeFileConstantExists verifies that FlagTypeFile constant is defined.
func TestFlagTypeFileConstantExists(t *testing.T) {
	assert.Equal(t, model.FlagType("file"), model.FlagTypeFile,
		"FlagTypeFile should equal FlagType(\"file\")")
}

// --- RequestBody.IsFileUpload field ---

// TestRequestBodyIsFileUploadField verifies that RequestBody has an IsFileUpload field.
func TestRequestBodyIsFileUploadField(t *testing.T) {
	rb := model.RequestBody{
		ContentType:  "multipart/form-data",
		Required:     true,
		IsFileUpload: true,
	}
	assert.True(t, rb.IsFileUpload)

	rb2 := model.RequestBody{
		ContentType: "application/json",
	}
	assert.False(t, rb2.IsFileUpload)
}

// --- ExtractBodyFlags multipart detection ---

// TestExtractBodyFlags_MultipartPureFileUpload verifies that a multipart/form-data
// body with a single binary field produces a FlagTypeFile flag.
func TestExtractBodyFlags_MultipartPureFileUpload(t *testing.T) {
	body := makeMultipartRequestBody(map[string]interface{}{
		"file": map[string]interface{}{
			"type":   "string",
			"format": "binary",
		},
	}, []string{"file"})

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 1, "one binary field should produce one flag")

	assert.Equal(t, "file", flags[0].Name)
	assert.Equal(t, model.FlagTypeFile, flags[0].Type,
		"binary format string should become FlagTypeFile")
	assert.True(t, flags[0].Required, "required binary field should be Required=true")
}

// TestExtractBodyFlags_MultipartMixedFields verifies that a multipart/form-data
// body with both binary and text fields produces a mix of FlagTypeFile and
// regular flags.
func TestExtractBodyFlags_MultipartMixedFields(t *testing.T) {
	body := makeMultipartRequestBody(map[string]interface{}{
		"file": map[string]interface{}{
			"type":   "string",
			"format": "binary",
		},
		"name": map[string]interface{}{
			"type": "string",
		},
		"description": map[string]interface{}{
			"type": "string",
		},
	}, []string{"file", "name"})

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 3, "three properties should produce three flags")

	byName := make(map[string]model.Flag, len(flags))
	for _, f := range flags {
		byName[f.Name] = f
	}

	// Binary field → FlagTypeFile
	fileFlag, ok := byName["file"]
	require.True(t, ok, "expected flag named 'file'")
	assert.Equal(t, model.FlagTypeFile, fileFlag.Type, "binary field should be FlagTypeFile")
	assert.True(t, fileFlag.Required)

	// Text fields → FlagTypeString
	nameFlag, ok := byName["name"]
	require.True(t, ok, "expected flag named 'name'")
	assert.Equal(t, model.FlagTypeString, nameFlag.Type, "text field should be FlagTypeString")
	assert.True(t, nameFlag.Required)

	descFlag, ok := byName["description"]
	require.True(t, ok, "expected flag named 'description'")
	assert.Equal(t, model.FlagTypeString, descFlag.Type, "text field should be FlagTypeString")
	assert.False(t, descFlag.Required)
}

// TestExtractBodyFlags_MultipartFileSource verifies that multipart file flags
// have the correct Source (FlagSourceBody or a dedicated file source).
func TestExtractBodyFlags_MultipartFileSource(t *testing.T) {
	body := makeMultipartRequestBody(map[string]interface{}{
		"attachment": map[string]interface{}{
			"type":   "string",
			"format": "binary",
		},
	}, nil)

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 1)

	// The source should indicate a body/file upload (either FlagSourceBody or a dedicated source).
	assert.NotEmpty(t, string(flags[0].Source),
		"file upload flag must have a non-empty Source")
}

// TestExtractBodyFlags_MultipartMultipleBinaryFields verifies that multiple
// binary fields all become FlagTypeFile.
func TestExtractBodyFlags_MultipartMultipleBinaryFields(t *testing.T) {
	body := makeMultipartRequestBody(map[string]interface{}{
		"avatar": map[string]interface{}{
			"type":   "string",
			"format": "binary",
		},
		"cover": map[string]interface{}{
			"type":   "string",
			"format": "binary",
		},
	}, nil)

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 2)

	for _, f := range flags {
		assert.Equal(t, model.FlagTypeFile, f.Type,
			"flag %q should be FlagTypeFile", f.Name)
	}
}

// TestExtractBodyFlags_MultipartIntegerField verifies that a non-binary, non-string
// field (e.g. integer) in a multipart form is handled correctly as a regular flag.
func TestExtractBodyFlags_MultipartIntegerField(t *testing.T) {
	body := makeMultipartRequestBody(map[string]interface{}{
		"count": map[string]interface{}{
			"type": "integer",
		},
		"upload": map[string]interface{}{
			"type":   "string",
			"format": "binary",
		},
	}, nil)

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 2)

	byName := make(map[string]model.Flag, len(flags))
	for _, f := range flags {
		byName[f.Name] = f
	}

	assert.Equal(t, model.FlagTypeInt, byName["count"].Type,
		"integer field in multipart form should be FlagTypeInt")
	assert.Equal(t, model.FlagTypeFile, byName["upload"].Type,
		"binary field in multipart form should be FlagTypeFile")
}

// --- Regression: JSON handling unchanged ---

// TestExtractBodyFlags_JSONUnchangedByMultipartAddition verifies that adding
// multipart/form-data support does not affect application/json handling.
func TestExtractBodyFlags_JSONUnchangedByMultipartAddition(t *testing.T) {
	body := makeRequestBody(map[string]interface{}{
		"title": map[string]interface{}{"type": "string"},
		"count": map[string]interface{}{"type": "integer"},
	}, []string{"title"})

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	require.Len(t, flags, 2, "JSON body with two props should still yield two flags")

	byName := make(map[string]model.Flag, len(flags))
	for _, f := range flags {
		byName[f.Name] = f
	}

	assert.Equal(t, model.FlagTypeString, byName["title"].Type)
	assert.True(t, byName["title"].Required)
	assert.Equal(t, model.FlagTypeInt, byName["count"].Type)
	assert.False(t, byName["count"].Required)

	// Crucially, JSON flags should NOT be FlagTypeFile.
	for _, f := range flags {
		assert.NotEqual(t, model.FlagTypeFile, f.Type,
			"JSON body flags should never be FlagTypeFile, but got %q for flag %q", f.Type, f.Name)
	}
}

// TestExtractBodyFlags_NilBodyUnchanged verifies nil body still returns no flags.
func TestExtractBodyFlags_NilBodyUnchanged(t *testing.T) {
	flags, err := model.ExtractBodyFlags(nil)
	require.NoError(t, err)
	assert.Empty(t, flags)
}

// TestExtractBodyFlags_MultipartNoSchema verifies that a multipart body with
// no schema properties returns empty flags without error.
func TestExtractBodyFlags_MultipartNoSchema(t *testing.T) {
	body := map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"multipart/form-data": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					// no "properties" key
				},
			},
		},
	}

	flags, err := model.ExtractBodyFlags(body)
	require.NoError(t, err)
	assert.Empty(t, flags, "multipart body with no schema properties should produce no flags")
}

// --- Pipeline integration test ---

// TestExtractBodyFlags_MultipartSetsRequestBody verifies that the full builder
// pipeline sets cmd.RequestBody correctly for an operation with a
// multipart/form-data request body containing a binary field.
//
// This test FAILS until builder.go is updated to construct and populate
// cmd.RequestBody (IsFileUpload, ContentType) when multipart/form-data is
// detected — currently the builder extracts flags but never sets RequestBody.
func TestExtractBodyFlags_MultipartSetsRequestBody(t *testing.T) {
	// Minimal OpenAPI 3.0 spec with a single POST /uploads endpoint that has
	// a multipart/form-data request body with one binary field.
	specJSON := []byte(`{
		"openapi": "3.0.3",
		"info": {"title": "Upload Test", "version": "1.0.0"},
		"paths": {
			"/uploads": {
				"post": {
					"summary": "Upload a file",
					"requestBody": {
						"required": true,
						"content": {
							"multipart/form-data": {
								"schema": {
									"type": "object",
									"required": ["file"],
									"properties": {
										"file": {
											"type": "string",
											"format": "binary"
										}
									}
								}
							}
						}
					},
					"responses": {
						"200": {"description": "OK"}
					}
				}
			}
		}
	}`)

	// Use the rawJSONResult helper defined in pagination_test.go (same package).
	result := &rawJSONResult{data: specJSON}

	resources, err := model.Build(result)
	require.NoError(t, err, "Build should succeed for the multipart upload spec")
	require.NotEmpty(t, resources, "should produce at least one resource")

	// Find the uploads resource and its create command.
	var uploadCmd *model.Command
	for i := range resources {
		if resources[i].Name == "uploads" {
			for j := range resources[i].Commands {
				if resources[i].Commands[j].HTTPMethod == "POST" {
					uploadCmd = &resources[i].Commands[j]
					break
				}
			}
		}
	}
	require.NotNil(t, uploadCmd, "expected a POST /uploads command in the built model")

	// The pipeline must populate RequestBody for multipart operations.
	require.NotNil(t, uploadCmd.RequestBody,
		"cmd.RequestBody must not be nil for a multipart/form-data operation — builder.go needs to set it")

	assert.True(t, uploadCmd.RequestBody.IsFileUpload,
		"cmd.RequestBody.IsFileUpload must be true for a multipart/form-data operation with binary fields")

	assert.Equal(t, "multipart/form-data", uploadCmd.RequestBody.ContentType,
		"cmd.RequestBody.ContentType must be \"multipart/form-data\"")
}
