// Package generator_test contains integration-style tests for the generator
// package. These tests use the 3-argument GenerateVerbCmd(resource, cmd, cliName)
// signature that B.A. will introduce. All tests that exercise RunE behaviour
// FAIL against the current 2-arg stub implementation.
package generator_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// verbCmdWithArgs builds a typical single-path-param GET command.
func verbCmdWithArgs() (model.Resource, model.Command) {
	res := model.Resource{Name: "users", Description: "Manage user accounts"}
	cmd := model.Command{
		Name:       "get",
		HTTPMethod: "GET",
		Path:       "/users/{userId}",
		Args: []model.Arg{
			{Name: "user-id", Description: "The user ID", Required: true},
		},
	}
	return res, cmd
}

// verbCmdWithQueryFlags builds a list command with query flags.
func verbCmdWithQueryFlags() (model.Resource, model.Command) {
	res := model.Resource{Name: "pets"}
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/pets",
		Flags: []model.Flag{
			{Name: "limit", Type: model.FlagTypeInt, Source: model.FlagSourceQuery},
			{Name: "tags", Type: model.FlagTypeStringSlice, Source: model.FlagSourceQuery},
		},
	}
	return res, cmd
}

// verbCmdWithBodyFlags builds a create command with body flags.
func verbCmdWithBodyFlags() (model.Resource, model.Command) {
	res := model.Resource{Name: "pets"}
	cmd := model.Command{
		Name:       "create",
		HTTPMethod: "POST",
		Path:       "/pets",
		Flags: []model.Flag{
			{Name: "name", Type: model.FlagTypeString, Required: true, Source: model.FlagSourceBody},
			{Name: "tag", Type: model.FlagTypeString, Source: model.FlagSourceBody},
		},
	}
	return res, cmd
}

// ---------------------------------------------------------------------------
// Updated existing tests — 3-arg signature
// ---------------------------------------------------------------------------

// TestGenerateVerbCmd3_ArgsWiring is the 3-arg equivalent of the existing
// TestGenerateVerbCmd_ArgsWiring. Verifies cobra.ExactArgs(1) for 1 path arg.
func TestGenerateVerbCmd3_ArgsWiring(t *testing.T) {
	res, cmd := verbCmdWithArgs()

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, "cobra.ExactArgs(1)",
		"should wire cobra.ExactArgs(1) for a command with 1 path arg")
}

// TestGenerateVerbCmd3_ZeroArgs is the 3-arg equivalent of TestGenerateVerbCmd_ZeroArgs.
func TestGenerateVerbCmd3_ZeroArgs(t *testing.T) {
	res := model.Resource{Name: "users"}
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/users",
		Args:       []model.Arg{},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	hasZeroArgs := strings.Contains(src, "cobra.ExactArgs(0)") ||
		strings.Contains(src, "cobra.NoArgs")
	assert.True(t, hasZeroArgs,
		"should wire ExactArgs(0) or NoArgs for a command with 0 path args")
}

// TestGenerateVerbCmd3_FlagTypes is the 3-arg equivalent of TestGenerateVerbCmd_FlagTypes.
func TestGenerateVerbCmd3_FlagTypes(t *testing.T) {
	res := model.Resource{Name: "items"}
	cmd := model.Command{
		Name:       "create",
		HTTPMethod: "POST",
		Path:       "/items",
		Flags: []model.Flag{
			{Name: "name", Type: model.FlagTypeString, Source: model.FlagSourceBody},
			{Name: "count", Type: model.FlagTypeInt, Source: model.FlagSourceBody},
			{Name: "active", Type: model.FlagTypeBool, Source: model.FlagSourceQuery},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, "StringVar", "should register string flags with StringVar")
	assert.Contains(t, src, "IntVar", "should register int flags with IntVar")
	assert.Contains(t, src, "BoolVar", "should register bool flags with BoolVar")
}

// TestGenerateVerbCmd3_RequiredFlags is the 3-arg equivalent of TestGenerateVerbCmd_RequiredFlags.
func TestGenerateVerbCmd3_RequiredFlags(t *testing.T) {
	res := model.Resource{Name: "orders"}
	cmd := model.Command{
		Name:       "create",
		HTTPMethod: "POST",
		Path:       "/orders",
		Flags: []model.Flag{
			{Name: "customer-id", Type: model.FlagTypeString, Required: true, Source: model.FlagSourceBody},
			{Name: "notes", Type: model.FlagTypeString, Required: false, Source: model.FlagSourceBody},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, "MarkFlagRequired",
		"required flags should generate a MarkFlagRequired call")
	assert.Contains(t, src, "customer-id",
		"the required flag name should appear in the generated source")
}

// TestGenerateVerbCmd3_ValidGoSyntax is the 3-arg equivalent of TestGenerateVerbCmd_ValidGoSyntax.
func TestGenerateVerbCmd3_ValidGoSyntax(t *testing.T) {
	res := model.Resource{Name: "users", Description: "Manage user accounts"}
	cmd := model.Command{
		Name:        "update",
		HTTPMethod:  "PATCH",
		Path:        "/users/{userId}",
		Description: "Update a user by ID",
		Args: []model.Arg{
			{Name: "user-id", Description: "The user ID", Required: true},
		},
		Flags: []model.Flag{
			{Name: "email", Type: model.FlagTypeString, Required: false, Source: model.FlagSourceBody},
			{Name: "role", Type: model.FlagTypeString, Required: true, Source: model.FlagSourceBody},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)
	require.NotEmpty(t, src)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "users_update.go", src, parser.AllErrors)
	assert.NoError(t, parseErr,
		"generated Go source should parse without syntax errors:\n%s", src)
}

// TestGenerateVerbCmd3_UseField is the 3-arg equivalent of TestGenerateVerbCmd_UseField.
func TestGenerateVerbCmd3_UseField(t *testing.T) {
	res := model.Resource{Name: "repos"}
	cmd := model.Command{
		Name:       "get",
		HTTPMethod: "GET",
		Path:       "/repos/{owner}/{repo}",
		Args: []model.Arg{
			{Name: "owner", Required: true},
			{Name: "repo", Required: true},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, `"get`, "Use field should start with the command name")
	assert.Contains(t, src, "owner", "Use field should include arg placeholder for owner")
	assert.Contains(t, src, "repo", "Use field should include arg placeholder for repo")
}

// ---------------------------------------------------------------------------
// New tests — RunE behaviour (all FAIL against current stub Run implementation)
// ---------------------------------------------------------------------------

// TestGenerateVerbCmdRunESignature verifies that the generated source uses
// RunE (returning an error) rather than Run (fire-and-forget).
// FAILS against current impl which emits `Run:`.
func TestGenerateVerbCmdRunESignature(t *testing.T) {
	res, cmd := verbCmdWithArgs()

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, "RunE", "generated command should use RunE, not Run")
	assert.NotContains(t, src, "\tRun: func", "generated command should not use Run (without E)")
}

// TestGenerateVerbCmdImportsClient verifies that the generated source imports
// the CLI's internal client package using the provided cliName.
// FAILS against current impl which only imports "fmt".
func TestGenerateVerbCmdImportsClient(t *testing.T) {
	res, cmd := verbCmdWithArgs()

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, `"testcli/internal/client"`,
		"generated source should import the testcli internal client package")
}

// TestGenerateVerbCmdPathParams verifies that for a command with path args,
// the generated RunE maps positional args[N] into a pathParams map passed to
// client.Do().
// FAILS against current impl which has a stub handler.
func TestGenerateVerbCmdPathParams(t *testing.T) {
	res, cmd := verbCmdWithArgs()

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	// RunE must reference args[0] and map it to the path param key "userId"
	assert.Contains(t, src, "args[0]",
		"RunE should read args[0] for the first positional argument")
	assert.Contains(t, src, "pathParams",
		"RunE should build a pathParams map for client.Do()")
}

// TestGenerateVerbCmdQueryFlags verifies that for a command with query flags,
// the generated RunE reads the flag values and builds a queryParams map.
// FAILS against current impl which has a stub handler.
func TestGenerateVerbCmdQueryFlags(t *testing.T) {
	res, cmd := verbCmdWithQueryFlags()

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, "queryParams",
		"RunE should build a queryParams map for query flags")
	// The flag names should appear in the RunE body
	assert.Contains(t, src, "limit",
		"RunE should reference the 'limit' query flag")
}

// TestGenerateVerbCmdBodyFlags verifies that for a command with body flags,
// the generated RunE builds a body map populated from the flag values.
// FAILS against current impl which has a stub handler.
func TestGenerateVerbCmdBodyFlags(t *testing.T) {
	res, cmd := verbCmdWithBodyFlags()

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, "body",
		"RunE should build a body map for body flags")
	assert.Contains(t, src, "name",
		"RunE should reference the 'name' body flag")
}

// TestGenerateVerbCmdNoBodyWhenNoBodyFlags verifies that for a command with
// only query flags (no body flags), the generated RunE does NOT build a body
// map (passes nil to client.Do for the body argument).
// FAILS against current impl which has a stub handler.
func TestGenerateVerbCmdNoBodyWhenNoBodyFlags(t *testing.T) {
	res, cmd := verbCmdWithQueryFlags() // only query flags, no body flags

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	// Should not construct a body map when no FlagSourceBody flags exist.
	// A nil literal or no body variable is acceptable.
	hasNoBody := !strings.Contains(src, `"body"`) &&
		(strings.Contains(src, "nil") || !strings.Contains(src, "bodyMap"))
	assert.True(t, hasNoBody,
		"RunE should not build a body map when there are no body flags; got:\n%s", src)
}

// TestGenerateVerbCmdReadsBaseURL verifies that the generated source reads the
// --base-url flag from the root persistent flags so it can construct the client.
// FAILS against current impl which has a stub handler.
func TestGenerateVerbCmdReadsBaseURL(t *testing.T) {
	res, cmd := verbCmdWithArgs()

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	hasBaseURL := strings.Contains(src, "base-url") ||
		strings.Contains(src, "baseURL") ||
		strings.Contains(src, "BaseURL")
	assert.True(t, hasBaseURL,
		"generated RunE should read the --base-url flag to configure the client")
}

// TestGenerateVerbCmdReadsToken verifies that the generated source reads an
// auth token from an environment variable to pass to the client.
// FAILS against current impl which has a stub handler.
func TestGenerateVerbCmdReadsToken(t *testing.T) {
	res, cmd := verbCmdWithArgs()

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	hasToken := strings.Contains(src, "os.Getenv") ||
		strings.Contains(src, "TOKEN") ||
		strings.Contains(src, "token")
	assert.True(t, hasToken,
		"generated RunE should read a token from an environment variable")
}

// ---------------------------------------------------------------------------
// Conditional import tests — all FAIL against current impl (Bug 1, 2, 3)
// ---------------------------------------------------------------------------

// TestNoStrconvForStringOnlyFlags verifies that when a command has only string
// query flags (no int or bool), the generated file does NOT import "strconv".
// FAILS: current buildImports always includes "strconv" unconditionally.
func TestNoStrconvForStringOnlyFlags(t *testing.T) {
	res := model.Resource{Name: "widgets"}
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/widgets",
		Flags: []model.Flag{
			{Name: "filter", Type: model.FlagTypeString, Source: model.FlagSourceQuery},
			{Name: "sort", Type: model.FlagTypeString, Source: model.FlagSourceQuery},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.NotContains(t, src, `"strconv"`,
		"generated source should NOT import strconv when only string query flags exist;\ngot:\n%s", src)
}

// TestStrconvForIntFlags verifies that when a command has an int query flag,
// the generated file DOES import "strconv".
func TestStrconvForIntFlags(t *testing.T) {
	res := model.Resource{Name: "widgets"}
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/widgets",
		Flags: []model.Flag{
			{Name: "limit", Type: model.FlagTypeInt, Source: model.FlagSourceQuery},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, `"strconv"`,
		"generated source MUST import strconv when an int query flag exists")
}

// TestNoJsonImportForGetCommand verifies that GET commands do NOT import
// "encoding/json" — they have no body marshaling and output uses raw bytes.
func TestNoJsonImportForGetCommand(t *testing.T) {
	res := model.Resource{Name: "widgets"}
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/widgets",
		Flags: []model.Flag{
			{Name: "filter", Type: model.FlagTypeString, Source: model.FlagSourceQuery},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.NotContains(t, src, `"encoding/json"`,
		"GET commands do not marshal a body, so encoding/json is not needed")
}

// TestJsonImportForBodyFlags verifies that when a command has body flags,
// the generated file DOES import "encoding/json".
func TestJsonImportForBodyFlags(t *testing.T) {
	res, cmd := verbCmdWithBodyFlags()

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, `"encoding/json"`,
		"generated source MUST import encoding/json when body flags exist")
}

// TestStringSliceQueryFlagHandled verifies that a FlagTypeStringSlice query
// flag is handled without assigning a []string directly into map[string]string.
// FAILS: current switch falls through to default, emitting `queryParams["tags"] = tagsVar`
// where tagsVar is []string — a type mismatch that prevents compilation.
func TestStringSliceQueryFlagHandled(t *testing.T) {
	res := model.Resource{Name: "pets"}
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/pets",
		Flags: []model.Flag{
			{Name: "tags", Type: model.FlagTypeStringSlice, Source: model.FlagSourceQuery},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	// The generated code must compile as valid Go. Parse it to detect type errors
	// indirectly: if []string is assigned directly to map[string]string the
	// variable declaration will contain "[]string" next to a bare queryParams assignment,
	// which we can detect heuristically.
	//
	// A correct implementation either joins the slice ("strings.Join"), URL-encodes
	// it, or uses repeated query params. It must NOT produce a bare bare assignment of
	// a []string variable into the string map.
	//
	// The bug: flagGoType returns "[]string" for FlagTypeStringSlice, and the switch
	// in buildRunEBody falls through to default, emitting:
	//   queryParams["tags"] = petsListCmd_tags
	// where petsListCmd_tags is []string — a compile-time type mismatch.
	// We detect this by checking that the generated source has a var declared as
	// []string AND is then directly assigned (without a join/encode function) into queryParams.
	hasBareSliceVarDecl := strings.Contains(src, "[]string") &&
		strings.Contains(src, "var (")
	hasDirectAssign := strings.Contains(src, `queryParams["tags"] = petsListCmd_tags`)
	hasBareSliceAssign := hasBareSliceVarDecl && hasDirectAssign
	assert.False(t, hasBareSliceAssign,
		"StringSlice query flag must not be assigned directly ([]string) into map[string]string;\ngot:\n%s", src)
}

// TestGenerateVerbCmdJsonFlagForQueryOnlyCommand verifies that commands with
// only query flags (no body flags) still check the --json persistent flag in
// their generated RunE, producing consistent output behaviour across all commands.
func TestGenerateVerbCmdJsonFlagForQueryOnlyCommand(t *testing.T) {
	res := model.Resource{Name: "widgets"}
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/widgets",
		Flags: []model.Flag{
			{Name: "limit", Type: model.FlagTypeInt, Source: model.FlagSourceQuery},
		},
	}

	src, err := generator.GenerateVerbCmd(res, cmd, "testcli")
	require.NoError(t, err)

	assert.Contains(t, src, `GetBool("json")`,
		"commands without body flags must still check the --json persistent flag")
}

// TestCliNameToEnvPrefixViaGeneratedCode exercises the cliNameToEnvPrefix
// conversion indirectly by checking that the generated RunE reads the expected
// env var token name for various cliName inputs.
func TestCliNameToEnvPrefixViaGeneratedCode(t *testing.T) {
	res := model.Resource{Name: "items"}
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
	}

	tests := []struct {
		cliName    string
		wantEnvVar string
	}{
		{"my-cli", "MY_CLI_TOKEN"},
		{"my-api.v1", "MY_API_V1_TOKEN"},
		{"myapi", "MYAPI_TOKEN"},
		{"api2cli", "API2CLI_TOKEN"},
	}

	for _, tt := range tests {
		t.Run(tt.cliName, func(t *testing.T) {
			src, err := generator.GenerateVerbCmd(res, cmd, tt.cliName)
			require.NoError(t, err)
			assert.Contains(t, src, tt.wantEnvVar,
				"generated code should read token from %s env var", tt.wantEnvVar)
		})
	}
}
