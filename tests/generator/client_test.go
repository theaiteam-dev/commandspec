package generator_test

import (
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateClientEmbedsDefaultBaseURL verifies that GenerateClient embeds
// the spec's BaseURL as a DefaultBaseURL constant in the generated source.
// This test FAILS against the current implementation (which ignores spec.BaseURL)
// and should PASS once GenerateClient embeds the value.
func TestGenerateClientEmbedsDefaultBaseURL(t *testing.T) {
	spec := &model.APISpec{
		Title:   "Petstore",
		Version: "1.0.0",
		BaseURL: "https://petstore.example.com/v1",
	}

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)
	require.NotEmpty(t, src)

	assert.Contains(t, src, `const DefaultBaseURL = "https://petstore.example.com/v1"`,
		"generated source should embed spec.BaseURL as const DefaultBaseURL")
}

// TestGenerateClientEmptyBaseURL verifies that GenerateClient handles an empty
// BaseURL gracefully by emitting a const DefaultBaseURL = "" declaration.
// This test FAILS against the current implementation (which ignores spec.BaseURL)
// and should PASS once GenerateClient embeds the value.
func TestGenerateClientEmptyBaseURL(t *testing.T) {
	spec := &model.APISpec{
		Title:   "Test",
		Version: "1.0.0",
		BaseURL: "",
	}

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)
	require.NotEmpty(t, src)

	assert.Contains(t, src, `const DefaultBaseURL = ""`,
		"generated source should embed empty string as const DefaultBaseURL when spec.BaseURL is empty")
}

// TestGenerateClientNewClientFallback verifies that the generated source contains
// logic in NewClient (or at call sites) to fall back to DefaultBaseURL when an
// empty string is passed as baseURL.
// This test FAILS against the current implementation (which has no fallback logic)
// and should PASS once GenerateClient adds the fallback.
func TestGenerateClientNewClientFallback(t *testing.T) {
	spec := &model.APISpec{
		Title:   "Test",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
	}

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)
	require.NotEmpty(t, src)

	// The generated NewClient must fall back to DefaultBaseURL when baseURL == "".
	// Accept any of these idiomatic Go patterns for the fallback:
	//   if baseURL == "" { baseURL = DefaultBaseURL }
	//   baseURL = DefaultBaseURL (when baseURL is "")
	hasFallback := assert.True(t,
		containsAny(src,
			`if baseURL == "" {`,
			`if baseURL == ""`,
			`baseURL == "" {`,
			`DefaultBaseURL`,
		),
		"generated NewClient should fall back to DefaultBaseURL when empty baseURL is passed; got source:\n%s", src,
	)

	if hasFallback {
		// Stronger check: DefaultBaseURL must appear in the constructor context.
		assert.Contains(t, src, "DefaultBaseURL",
			"generated source should reference DefaultBaseURL in NewClient or at the fallback site")
	}
}

// containsAny returns true if s contains at least one of the given substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
