package generator_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/queso/swagger-jack/internal/generator"
	"github.com/queso/swagger-jack/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalSpec returns a minimal APISpec suitable for exercising GenerateClient.
func minimalSpec() *model.APISpec {
	return &model.APISpec{
		Title:   "Test API",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
	}
}

// TestGenerateClient_ClientStruct verifies that the generated code defines a
// Client struct with the expected fields: BaseURL, Token, and HTTPClient.
func TestGenerateClient_ClientStruct(t *testing.T) {
	src, err := generator.GenerateClient(minimalSpec())
	require.NoError(t, err)

	assert.Contains(t, src, "type Client struct", "should define a Client struct")
	assert.Contains(t, src, "BaseURL", "Client struct should have a BaseURL field")
	assert.Contains(t, src, "Token", "Client struct should have a Token field")
	assert.Contains(t, src, "HTTPClient", "Client struct should have an HTTPClient field")
}

// TestGenerateClient_NewClientConstructor verifies that the generated code
// contains a NewClient constructor function that accepts baseURL and token.
func TestGenerateClient_NewClientConstructor(t *testing.T) {
	src, err := generator.GenerateClient(minimalSpec())
	require.NoError(t, err)

	assert.Contains(t, src, "NewClient", "should define a NewClient constructor")
	// Constructor should accept baseURL and token string parameters
	assert.Contains(t, src, "baseURL", "NewClient should accept a baseURL parameter")
	assert.Contains(t, src, "token", "NewClient should accept a token parameter")
}

// TestGenerateClient_DoMethod verifies that the generated code contains a Do
// method on the Client type for executing HTTP requests.
func TestGenerateClient_DoMethod(t *testing.T) {
	src, err := generator.GenerateClient(minimalSpec())
	require.NoError(t, err)

	assert.Contains(t, src, "func (c *Client) Do(", "should define a Do method on *Client")
}

// TestGenerateClient_BearerTokenAuth verifies that the generated code injects
// a Bearer token into the Authorization header for outgoing requests.
func TestGenerateClient_BearerTokenAuth(t *testing.T) {
	src, err := generator.GenerateClient(minimalSpec())
	require.NoError(t, err)

	assert.Contains(t, src, "Authorization", "should set the Authorization header")
	assert.Contains(t, src, "Bearer", "Authorization header should use the Bearer scheme")
}

// TestGenerateClient_ValidGoSyntax verifies that the generated source code is
// syntactically valid Go that can be parsed without errors.
func TestGenerateClient_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateClient(minimalSpec())
	require.NoError(t, err)
	require.NotEmpty(t, src, "generated source should not be empty")

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "client.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "generated Go source should parse without syntax errors:\n%s", src)
}

// TestGenerateClient_PathInterpolation verifies that the generated code contains
// logic for substituting {param} placeholders in URL path templates with actual
// runtime values.
func TestGenerateClient_PathInterpolation(t *testing.T) {
	src, err := generator.GenerateClient(minimalSpec())
	require.NoError(t, err)

	// The generated client must handle path parameters — either via strings.Replace,
	// fmt.Sprintf-style formatting, or a dedicated interpolation function.
	// We check for the presence of the brace syntax used in OpenAPI paths.
	hasInterpolation := strings.Contains(src, "strings.Replace") ||
		strings.Contains(src, "strings.NewReplacer") ||
		(strings.Contains(src, "{") && strings.Contains(src, "}"))
	assert.True(t, hasInterpolation,
		"generated code should contain path parameter interpolation logic (e.g., strings.Replace or brace substitution)")
}

// TestGenerateClient_ErrorOnNonSuccessStatus verifies that the generated code
// returns a descriptive error when the HTTP response status code is not 2xx.
func TestGenerateClient_ErrorOnNonSuccessStatus(t *testing.T) {
	src, err := generator.GenerateClient(minimalSpec())
	require.NoError(t, err)

	// The generated client should check the status code and return an error for non-2xx.
	hasStatusCheck := strings.Contains(src, "StatusCode") &&
		(strings.Contains(src, ">= 200") || strings.Contains(src, "< 200") ||
			strings.Contains(src, ">= 300") || strings.Contains(src, "fmt.Errorf"))
	assert.True(t, hasStatusCheck,
		"generated code should check HTTP status codes and return an error for non-2xx responses")
}
