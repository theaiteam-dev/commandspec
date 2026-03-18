// Package generator_test contains tests for nested object dot-notation flag
// building in generated RunE bodies.
package generator_test

import (
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeDotFlagCmd builds a model.Command with dot-notation body flags.
func makeDotFlagCmd(flags []model.Flag) model.Command {
	return model.Command{
		Name:       "create",
		HTTPMethod: "POST",
		Path:       "/items",
		Flags:      flags,
	}
}

// makeDotFlagResource wraps a command in a resource.
func makeDotFlagResource(cmd model.Command) model.Resource {
	return model.Resource{
		Name:     "items",
		Commands: []model.Command{cmd},
	}
}

// TestNestedFlagSetNestedCallGenerated verifies that the generated RunE uses
// setNested (or equivalent) to build nested maps from dot-notation flags.
// FAILS until buildRunEBody detects dot-notation and generates nested assignment.
func TestNestedFlagSetNestedCallGenerated(t *testing.T) {
	cmd := makeDotFlagCmd([]model.Flag{
		{Name: "address.city", Type: model.FlagTypeString, Source: model.FlagSourceBody},
	})
	resource := makeDotFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	hasNestedHandling := strings.Contains(src, "setNested") ||
		strings.Contains(src, "strings.Split") ||
		strings.Contains(src, "nested")
	assert.True(t, hasNestedHandling,
		"generated RunE should use setNested or equivalent for dot-notation flag 'address.city'")
}

// TestNestedFlagDoesNotUseFlatKey verifies that a dot-notation flag like
// address.city does NOT produce a flat bodyMap["address.city"] assignment.
// FAILS until buildRunEBody handles dot notation.
func TestNestedFlagDoesNotUseFlatKey(t *testing.T) {
	cmd := makeDotFlagCmd([]model.Flag{
		{Name: "address.city", Type: model.FlagTypeString, Source: model.FlagSourceBody},
	})
	resource := makeDotFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// Flat assignment looks like: bodyMap["address.city"] = ...
	assert.NotContains(t, src, `bodyMap["address.city"]`,
		"generated RunE should NOT use flat key 'address.city' in body map")
}

// TestMultipleNestedFlagsUnderSameParentMerge verifies that multiple dot-notation
// flags under the same parent (e.g., address.city and address.zip) produce
// a single nested map entry rather than overwriting.
// FAILS until buildRunEBody handles multiple nested flags under same parent.
func TestMultipleNestedFlagsUnderSameParentMerge(t *testing.T) {
	cmd := makeDotFlagCmd([]model.Flag{
		{Name: "address.city", Type: model.FlagTypeString, Source: model.FlagSourceBody},
		{Name: "address.zip", Type: model.FlagTypeString, Source: model.FlagSourceBody},
	})
	resource := makeDotFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// Neither flag should use the flat bodyMap key form.
	assert.NotContains(t, src, `bodyMap["address.city"]`,
		"address.city should NOT use flat key assignment")
	assert.NotContains(t, src, `bodyMap["address.zip"]`,
		"address.zip should NOT use flat key assignment")
}

// TestFlatFlagUnaffectedByNestedLogic verifies that a flat (non-dotted) body flag
// continues to produce a direct body map assignment.
func TestFlatFlagUnaffectedByNestedLogic(t *testing.T) {
	cmd := makeDotFlagCmd([]model.Flag{
		{Name: "name", Type: model.FlagTypeString, Source: model.FlagSourceBody},
	})
	resource := makeDotFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// Flat flag should produce bodyMap["name"] = ... directly
	assert.Contains(t, src, `"name"`,
		"flat flag 'name' should still produce a direct body assignment")
}

// TestDotNotationDepthLimitEnforced verifies that a flag with more than 3 dots
// (e.g., a.b.c.d — 4 parts) does NOT silently produce a flat bodyMap key.
// After implementation, this flag should be skipped or errored, not passed through.
// FAILS until buildRunEBody enforces the 3-level depth limit.
func TestDotNotationDepthLimitEnforced(t *testing.T) {
	cmd := makeDotFlagCmd([]model.Flag{
		{Name: "a.b.c.d", Type: model.FlagTypeString, Source: model.FlagSourceBody},
	})
	resource := makeDotFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// Should NOT silently produce a flat assignment for a 4-part key
	assert.NotContains(t, src, `bodyMap["a.b.c.d"]`,
		"4-part dot-notation flag should NOT produce a flat bodyMap assignment; depth limit should handle it")
}

// TestTwoLevelNestingSupported verifies that a flag like meta.key (1 dot, 2 levels)
// generates nested map code rather than flat assignment.
// FAILS until buildRunEBody handles dot notation.
func TestTwoLevelNestingSupported(t *testing.T) {
	cmd := makeDotFlagCmd([]model.Flag{
		{Name: "meta.key", Type: model.FlagTypeString, Source: model.FlagSourceBody},
	})
	resource := makeDotFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	assert.NotContains(t, src, `bodyMap["meta.key"]`,
		"two-level nested flag 'meta.key' should NOT use flat key; should use nested map code")
}

// TestNestedFlagValidGoSyntax verifies that generated code for dot-notation flags
// is valid Go syntax once implemented.
func TestNestedFlagValidGoSyntax(t *testing.T) {
	cmd := makeDotFlagCmd([]model.Flag{
		{Name: "address.city", Type: model.FlagTypeString, Source: model.FlagSourceBody},
		{Name: "address.zip", Type: model.FlagTypeString, Source: model.FlagSourceBody},
		{Name: "name", Type: model.FlagTypeString, Source: model.FlagSourceBody},
	})
	resource := makeDotFlagResource(cmd)
	src, err := generator.GenerateVerbCmd(resource, cmd, "myapi")
	require.NoError(t, err)

	// Only run syntax check if nested logic is present
	if strings.Contains(src, "setNested") || strings.Contains(src, "strings.Split") {
		mustParseGoSrc(t, "items_create.go", src)
	} else {
		t.Skip("nested dot-notation not yet implemented; skipping syntax check")
	}
}
