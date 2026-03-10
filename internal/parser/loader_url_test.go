// Package parser_test contains tests for URL-based spec loading with
// Content-Type format detection.
package parser_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/queso/swagger-jack/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalJSONSpec = `{
  "openapi": "3.0.3",
  "info": { "title": "URL JSON API", "version": "1.0.0" },
  "paths": {
    "/items": {
      "get": {
        "summary": "List items",
        "responses": { "200": { "description": "OK" } }
      }
    }
  }
}`

const minimalYAMLSpec = `openapi: "3.0.3"
info:
  title: URL YAML API
  version: "1.0.0"
paths:
  /items:
    get:
      summary: List items
      responses:
        "200":
          description: OK
`

// serveSpec starts a test HTTP server that serves the given body with the given
// Content-Type, and returns the server URL and a cleanup func.
func serveSpec(t *testing.T, body, contentType string, statusCode int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestLoadURL_JSONByContentType verifies that parser.Load fetches a URL and
// parses it as JSON when Content-Type is application/json.
// FAILS until loader.go detects http:// URLs and fetches them.
func TestLoadURL_JSONByContentType(t *testing.T) {
	url := serveSpec(t, minimalJSONSpec, "application/json", 200)
	result, err := parser.Load(url)
	require.NoError(t, err, "parser.Load should fetch and parse JSON from URL")
	require.NotNil(t, result)
	assert.Equal(t, "URL JSON API", result.Spec.Title)
}

// TestLoadURL_YAMLByContentType verifies that parser.Load fetches a URL and
// parses it as YAML when Content-Type is application/x-yaml.
// FAILS until loader.go detects URLs and handles YAML Content-Type.
func TestLoadURL_YAMLByContentType(t *testing.T) {
	url := serveSpec(t, minimalYAMLSpec, "application/x-yaml", 200)
	result, err := parser.Load(url)
	require.NoError(t, err, "parser.Load should fetch and parse YAML from URL with application/x-yaml")
	require.NotNil(t, result)
	assert.Equal(t, "URL YAML API", result.Spec.Title)
}

// TestLoadURL_YAMLByTextYAMLContentType verifies that text/yaml Content-Type
// also triggers YAML parsing.
// FAILS until loader.go handles text/yaml Content-Type.
func TestLoadURL_YAMLByTextYAMLContentType(t *testing.T) {
	url := serveSpec(t, minimalYAMLSpec, "text/yaml", 200)
	result, err := parser.Load(url)
	require.NoError(t, err, "parser.Load should parse YAML from URL with text/yaml Content-Type")
	require.NotNil(t, result)
	assert.Equal(t, "URL YAML API", result.Spec.Title)
}

// TestLoadURL_JSONByExtensionFallback verifies that when Content-Type is
// ambiguous, the URL path extension (.json) is used for format detection.
// FAILS until loader.go implements extension-based fallback.
func TestLoadURL_JSONByExtensionFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Content-Type is generic — should fall back to URL path extension
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(minimalJSONSpec))
	}))
	t.Cleanup(srv.Close)

	result, err := parser.Load(srv.URL + "/api.json")
	require.NoError(t, err, "parser.Load should fall back to .json extension for format detection")
	require.NotNil(t, result)
	assert.Equal(t, "URL JSON API", result.Spec.Title)
}

// TestLoadURL_YAMLByExtensionFallback verifies that .yaml extension triggers
// YAML parsing when Content-Type is ambiguous.
// FAILS until loader.go implements extension-based fallback.
func TestLoadURL_YAMLByExtensionFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(minimalYAMLSpec))
	}))
	t.Cleanup(srv.Close)

	result, err := parser.Load(srv.URL + "/api.yaml")
	require.NoError(t, err, "parser.Load should fall back to .yaml extension for format detection")
	require.NotNil(t, result)
	assert.Equal(t, "URL YAML API", result.Spec.Title)
}

// TestLoadURL_YmlExtensionFallback verifies that .yml extension also works.
// FAILS until loader.go handles .yml extension fallback.
func TestLoadURL_YmlExtensionFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(minimalYAMLSpec))
	}))
	t.Cleanup(srv.Close)

	result, err := parser.Load(srv.URL + "/api.yml")
	require.NoError(t, err, "parser.Load should handle .yml extension from URL path")
	require.NotNil(t, result)
	assert.Equal(t, "URL YAML API", result.Spec.Title)
}

// TestLoadURL_Non200StatusReturnsError verifies that a non-200 HTTP response
// produces a descriptive error.
// FAILS until loader.go checks HTTP status code for URL requests.
func TestLoadURL_Non200StatusReturnsError(t *testing.T) {
	url := serveSpec(t, `{"error": "not found"}`, "application/json", 404)
	_, err := parser.Load(url)
	require.Error(t, err, "non-200 HTTP response should return an error")
	assert.Contains(t, err.Error(), "404",
		"error message should include the HTTP status code")
}

// TestLoadURL_NetworkErrorReturnsError verifies that an unreachable URL
// returns a descriptive error.
// FAILS until loader.go handles network errors for URL requests.
func TestLoadURL_NetworkErrorReturnsError(t *testing.T) {
	// Use a port that is very unlikely to be in use
	_, err := parser.Load("http://127.0.0.1:19999/api.json")
	require.Error(t, err, "unreachable URL should return an error")
}

// TestLoadURL_LocalFilesStillWork verifies that local file loading is not
// broken when URL loading is added.
func TestLoadURL_LocalFilesStillWork(t *testing.T) {
	path := yamlFixtureDir() + "/minimal.json"
	result, err := parser.Load(path)
	require.NoError(t, err, "local file loading should still work after URL support added")
	require.NotNil(t, result)
	assert.Equal(t, "Minimal API", result.Spec.Title)
}

// TestLoadURL_SetsUserAgent verifies that the HTTP request includes a User-Agent header.
// FAILS until loader.go sets User-Agent on HTTP requests.
func TestLoadURL_SetsUserAgent(t *testing.T) {
	var capturedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(minimalJSONSpec))
	}))
	t.Cleanup(srv.Close)

	_, err := parser.Load(srv.URL)
	require.NoError(t, err)
	assert.NotEmpty(t, capturedUA,
		"HTTP request to URL should include a User-Agent header")
}

// TestLoadURL_RefsResolvedFromURL verifies that $ref references in a URL-fetched
// spec are resolved (inline resolution, not remote refs).
// FAILS until loader.go URL support feeds bytes into existing resolveRefs pipeline.
func TestLoadURL_RefsResolvedFromURL(t *testing.T) {
	specWithRefs := `{
  "openapi": "3.0.3",
  "info": { "title": "Refs from URL", "version": "1.0.0" },
  "components": {
    "schemas": {
      "Item": { "type": "object", "properties": { "id": { "type": "integer" } } }
    }
  },
  "paths": {
    "/items": {
      "get": {
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Item" }
              }
            }
          }
        }
      }
    }
  }
}`
	url := serveSpec(t, specWithRefs, "application/json", 200)
	result, err := parser.Load(url)
	require.NoError(t, err, "URL-fetched spec with $refs should parse successfully")
	require.NotNil(t, result)
	assert.NotContains(t, string(result.RawJSON), `"$ref"`,
		"$ref references should be resolved in URL-fetched specs")
}
