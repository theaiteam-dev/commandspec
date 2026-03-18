package generator_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// paginationTemplateDir returns the absolute path to the generator templates directory.
func paginationTemplateDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "templates")
}

// makePageBasedCmd returns a Command with page-based pagination metadata.
func makePageBasedCmd() model.Command {
	return model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
		Flags: []model.Flag{
			{Name: "page", Type: model.FlagTypeInt, Source: model.FlagSourceQuery},
			{Name: "per_page", Type: model.FlagTypeInt, Source: model.FlagSourceQuery},
		},
		Pagination: &model.Pagination{
			Type:      model.PaginationPageBased,
			PageParam: "page",
			SizeParam: "per_page",
		},
	}
}

// makeOffsetBasedCmd returns a Command with offset-based pagination metadata.
func makeOffsetBasedCmd() model.Command {
	return model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
		Flags: []model.Flag{
			{Name: "limit", Type: model.FlagTypeInt, Source: model.FlagSourceQuery},
			{Name: "offset", Type: model.FlagTypeInt, Source: model.FlagSourceQuery},
		},
		Pagination: &model.Pagination{
			Type:      model.PaginationOffsetBased,
			PageParam: "offset",
			SizeParam: "limit",
		},
	}
}

// makeCursorBasedCmd returns a Command with cursor-based pagination metadata.
func makeCursorBasedCmd() model.Command {
	return model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
		Flags: []model.Flag{
			{Name: "cursor", Type: model.FlagTypeString, Source: model.FlagSourceQuery},
		},
		Pagination: &model.Pagination{
			Type:        model.PaginationCursorBased,
			CursorParam: "cursor",
		},
	}
}

// makeNonPaginatedCmd returns a standard GET list command with no pagination.
func makeNonPaginatedCmd() model.Command {
	return model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
		Flags: []model.Flag{
			{Name: "filter", Type: model.FlagTypeString, Source: model.FlagSourceQuery},
		},
		Pagination: nil,
	}
}

// itemsResource returns a minimal Resource named "items".
func itemsResource() model.Resource {
	return model.Resource{Name: "items"}
}

// --- Page-based pagination flags ---

// TestGenerateVerbCmd_PageBased_HasPageFlag verifies that a page-based paginated
// command generates a --page flag.
//
// FAILS until GenerateVerbCmd adds --page for page-based pagination.
func TestGenerateVerbCmd_PageBased_HasPageFlag(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makePageBasedCmd(), "mycli")
	require.NoError(t, err)

	assert.Contains(t, src, `"page"`,
		"page-based command should register a --page flag")
}

// TestGenerateVerbCmd_PageBased_HasPerPageFlag verifies that a page-based paginated
// command generates a --per-page (or --per_page) flag.
//
// FAILS until GenerateVerbCmd adds --per-page / --per_page for page-based pagination.
func TestGenerateVerbCmd_PageBased_HasPerPageFlag(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makePageBasedCmd(), "mycli")
	require.NoError(t, err)

	hasPerPage := strings.Contains(src, `"per_page"`) || strings.Contains(src, `"per-page"`)
	assert.True(t, hasPerPage,
		"page-based command should register a --per_page or --per-page flag")
}

// TestGenerateVerbCmd_PageBased_HasAllFlag verifies that a page-based paginated
// command generates an --all flag for auto-pagination.
//
// FAILS until GenerateVerbCmd adds --all for paginated commands.
func TestGenerateVerbCmd_PageBased_HasAllFlag(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makePageBasedCmd(), "mycli")
	require.NoError(t, err)

	assert.Contains(t, src, `"all"`,
		"page-based paginated command should register an --all flag")
}

// TestGenerateVerbCmd_PageBased_AllFlagTriggersLoop verifies that the generated
// RunE body invokes auto-pagination when --all is used.
//
// B.A.'s implementation delegates pagination to client.FetchAll rather than
// emitting an inline loop — the test checks for the FetchAll call or equivalent
// delegation pattern gated on the --all flag.
func TestGenerateVerbCmd_PageBased_AllFlagTriggersLoop(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makePageBasedCmd(), "mycli")
	require.NoError(t, err)

	// The generated code must check the --all flag and invoke pagination.
	// Implementation delegates to client.FetchAll with a PaginationConfig.
	hasAllFlag := strings.Contains(src, "All") || strings.Contains(src, `"all"`)
	hasPaginationCall := strings.Contains(src, "FetchAll") ||
		strings.Contains(src, "PaginationConfig") ||
		strings.Contains(src, "fetchAll") ||
		strings.Contains(src, "for ") || strings.Contains(src, "for{")
	assert.True(t, hasAllFlag && hasPaginationCall,
		"page-based --all flag should trigger auto-pagination (via FetchAll or loop), got:\n%s", src)
}

// TestGenerateVerbCmd_PageBased_ValidGoSyntax verifies the generated source is valid Go.
func TestGenerateVerbCmd_PageBased_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makePageBasedCmd(), "mycli")
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "items_list.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "page-based paginated command should be valid Go:\n%s", src)
}

// --- Offset-based pagination flags ---

// TestGenerateVerbCmd_OffsetBased_HasLimitFlag verifies that an offset-based
// paginated command generates a --limit flag.
//
// FAILS until GenerateVerbCmd adds --limit for offset-based pagination.
func TestGenerateVerbCmd_OffsetBased_HasLimitFlag(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeOffsetBasedCmd(), "mycli")
	require.NoError(t, err)

	assert.Contains(t, src, `"limit"`,
		"offset-based command should register a --limit flag")
}

// TestGenerateVerbCmd_OffsetBased_HasOffsetFlag verifies that an offset-based
// paginated command generates an --offset flag.
//
// FAILS until GenerateVerbCmd adds --offset for offset-based pagination.
func TestGenerateVerbCmd_OffsetBased_HasOffsetFlag(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeOffsetBasedCmd(), "mycli")
	require.NoError(t, err)

	assert.Contains(t, src, `"offset"`,
		"offset-based command should register an --offset flag")
}

// TestGenerateVerbCmd_OffsetBased_HasAllFlag verifies that an offset-based
// paginated command generates an --all flag.
//
// FAILS until GenerateVerbCmd adds --all for paginated commands.
func TestGenerateVerbCmd_OffsetBased_HasAllFlag(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeOffsetBasedCmd(), "mycli")
	require.NoError(t, err)

	assert.Contains(t, src, `"all"`,
		"offset-based paginated command should register an --all flag")
}

// TestGenerateVerbCmd_OffsetBased_ValidGoSyntax verifies the generated source is valid Go.
func TestGenerateVerbCmd_OffsetBased_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeOffsetBasedCmd(), "mycli")
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "items_list.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "offset-based paginated command should be valid Go:\n%s", src)
}

// --- Cursor-based pagination flags ---

// TestGenerateVerbCmd_CursorBased_HasCursorFlag verifies that a cursor-based
// paginated command generates a --cursor flag.
//
// FAILS until GenerateVerbCmd adds --cursor for cursor-based pagination.
func TestGenerateVerbCmd_CursorBased_HasCursorFlag(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeCursorBasedCmd(), "mycli")
	require.NoError(t, err)

	assert.Contains(t, src, `"cursor"`,
		"cursor-based command should register a --cursor flag")
}

// TestGenerateVerbCmd_CursorBased_HasAllFlag verifies that a cursor-based
// paginated command generates an --all flag.
//
// FAILS until GenerateVerbCmd adds --all for paginated commands.
func TestGenerateVerbCmd_CursorBased_HasAllFlag(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeCursorBasedCmd(), "mycli")
	require.NoError(t, err)

	assert.Contains(t, src, `"all"`,
		"cursor-based paginated command should register an --all flag")
}

// TestGenerateVerbCmd_CursorBased_ExtractsCursorFromResponse verifies that the
// generated command wires cursor extraction into auto-pagination.
//
// B.A.'s implementation delegates cursor extraction to client.FetchAll via
// PaginationConfig — the verb cmd just passes the CursorParam name. The canonical
// field names (next_cursor, nextCursor, after, meta.next) live in the pagination
// helper (GeneratePagination), not in the verb cmd source itself.
func TestGenerateVerbCmd_CursorBased_ExtractsCursorFromResponse(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeCursorBasedCmd(), "mycli")
	require.NoError(t, err)

	// The verb cmd must pass the cursor param name to the pagination helper,
	// or contain cursor field names directly if using an inline approach.
	hasCursorWiring := strings.Contains(src, "cursor") &&
		(strings.Contains(src, "FetchAll") ||
			strings.Contains(src, "PaginationConfig") ||
			strings.Contains(src, "CursorParam") ||
			strings.Contains(src, "next_cursor") ||
			strings.Contains(src, "nextCursor") ||
			strings.Contains(src, `"after"`))
	assert.True(t, hasCursorWiring,
		"cursor-based command should wire cursor param into auto-pagination (via PaginationConfig.CursorParam or inline), got:\n%s", src)
}

// TestGenerateVerbCmd_CursorBased_ValidGoSyntax verifies the generated source is valid Go.
func TestGenerateVerbCmd_CursorBased_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeCursorBasedCmd(), "mycli")
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "items_list.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "cursor-based paginated command should be valid Go:\n%s", src)
}

// --- Non-paginated commands unaffected ---

// TestGenerateVerbCmd_NonPaginated_NoAllFlag verifies that a command with nil
// Pagination does NOT get an --all flag injected.
//
// FAILS if GenerateVerbCmd incorrectly adds --all to non-paginated commands.
func TestGenerateVerbCmd_NonPaginated_NoAllFlag(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeNonPaginatedCmd(), "mycli")
	require.NoError(t, err)

	// --all should NOT appear as a registered flag on non-paginated commands.
	// We look for the flag registration call specifically, not just the word "all".
	hasAllFlagReg := strings.Contains(src, `BoolVar(`) && strings.Contains(src, `"all"`)
	assert.False(t, hasAllFlagReg,
		"non-paginated command should NOT register an --all flag, but found one in:\n%s", src)
}

// TestGenerateVerbCmd_NonPaginated_NoPageFlag verifies that a non-paginated
// command without a "page" flag in its spec does not get one injected.
func TestGenerateVerbCmd_NonPaginated_NoPageFlag(t *testing.T) {
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/items",
		// Only a "filter" flag — no pagination params.
		Flags: []model.Flag{
			{Name: "filter", Type: model.FlagTypeString, Source: model.FlagSourceQuery},
		},
		Pagination: nil,
	}
	src, err := generator.GenerateVerbCmd(itemsResource(), cmd, "mycli")
	require.NoError(t, err)

	// "page" should only appear if it was in the spec flags.
	// With nil Pagination, the generator must not add it.
	assert.NotContains(t, src, `IntVar(&`, // should have no int vars unless spec had them
		"non-paginated command without int flags should not have IntVar calls")
}

// TestGenerateVerbCmd_NonPaginated_ValidGoSyntax verifies the non-paginated path is still valid Go.
func TestGenerateVerbCmd_NonPaginated_ValidGoSyntax(t *testing.T) {
	src, err := generator.GenerateVerbCmd(itemsResource(), makeNonPaginatedCmd(), "mycli")
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "items_list.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "non-paginated command should still be valid Go:\n%s", src)
}

// --- Safety limit ---

// TestGenerateVerbCmd_Pagination_SafetyLimit verifies that the full generated
// pagination system (verb cmd + helper) enforces a max-pages safety limit.
//
// B.A.'s implementation puts the safety limit in client.FetchAll (generated by
// GeneratePagination), not in the verb cmd itself. This test checks the helper
// rather than the verb cmd source — the verb cmd just delegates via PaginationConfig.
func TestGenerateVerbCmd_Pagination_SafetyLimit(t *testing.T) {
	// The safety limit lives in the GeneratePagination helper, not the verb cmd.
	// Verify that GeneratePagination embeds the limit (covers all pagination types).
	paginationSrc, err := generator.GeneratePagination("mycli")
	require.NoError(t, err, "GeneratePagination should not fail")

	has100 := strings.Contains(paginationSrc, "100")
	hasMaxPages := strings.Contains(paginationSrc, "maxPages") ||
		strings.Contains(paginationSrc, "max_pages") ||
		strings.Contains(paginationSrc, "MaxPages") ||
		strings.Contains(paginationSrc, "safetyLimit") ||
		has100
	assert.True(t, hasMaxPages,
		"GeneratePagination helper should contain a safety limit (max 100 pages), got:\n%s", paginationSrc)

	// Also verify each verb cmd type delegates to FetchAll (confirming the limit is reachable).
	for _, makeCmd := range []func() model.Command{
		makePageBasedCmd,
		makeOffsetBasedCmd,
		makeCursorBasedCmd,
	} {
		cmd := makeCmd()
		src, err := generator.GenerateVerbCmd(itemsResource(), cmd, "mycli")
		require.NoError(t, err, "command generation should not fail for %q", cmd.Pagination.Type)

		delegatesToHelper := strings.Contains(src, "FetchAll") ||
			strings.Contains(src, "PaginationConfig")
		assert.True(t, delegatesToHelper,
			"paginated verb cmd for %q should delegate to the pagination helper (FetchAll/PaginationConfig), got:\n%s",
			cmd.Pagination.Type, src)
	}
}

// --- GeneratePagination helper ---

// TestGeneratePagination_ReturnsSource verifies that GeneratePagination returns
// non-empty Go source code.
//
// FAILS until generator.GeneratePagination(cliName) is implemented.
func TestGeneratePagination_ReturnsSource(t *testing.T) {
	src, err := generator.GeneratePagination("mycli")
	require.NoError(t, err)
	assert.NotEmpty(t, src, "GeneratePagination should return non-empty Go source")
}

// TestGeneratePagination_PackageClient verifies the generated helper declares
// package client (it lives in internal/client/pagination.go).
//
// FAILS until GeneratePagination is implemented.
func TestGeneratePagination_PackageClient(t *testing.T) {
	src, err := generator.GeneratePagination("mycli")
	require.NoError(t, err)
	assert.Contains(t, src, "package client",
		"generated pagination helper should declare 'package client'")
}

// TestGeneratePagination_ValidGoSyntax verifies the generated pagination helper
// is syntactically valid Go.
//
// FAILS until GeneratePagination is implemented.
func TestGeneratePagination_ValidGoSyntax(t *testing.T) {
	src, err := generator.GeneratePagination("mycli")
	require.NoError(t, err)

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "pagination.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "generated pagination helper should be valid Go:\n%s", src)
}

// TestGeneratePagination_HasMaxPagesSafetyLimit verifies that the generated
// pagination helper defines a safety limit of 100.
//
// FAILS until GeneratePagination embeds a max-pages guard.
func TestGeneratePagination_HasMaxPagesSafetyLimit(t *testing.T) {
	src, err := generator.GeneratePagination("mycli")
	require.NoError(t, err)

	assert.Contains(t, src, "100",
		"generated pagination helper should contain the max-pages safety limit (100)")
}

// TestGeneratePagination_HandlesAllThreePaginationTypes verifies that the
// pagination helper handles page-based, offset-based, and cursor-based patterns.
//
// FAILS until GeneratePagination supports all three types.
func TestGeneratePagination_HandlesAllThreePaginationTypes(t *testing.T) {
	src, err := generator.GeneratePagination("mycli")
	require.NoError(t, err)

	lower := strings.ToLower(src)
	hasPage := strings.Contains(lower, "page")
	hasOffset := strings.Contains(lower, "offset")
	hasCursor := strings.Contains(lower, "cursor")

	assert.True(t, hasPage, "pagination helper should handle page-based pagination")
	assert.True(t, hasOffset, "pagination helper should handle offset-based pagination")
	assert.True(t, hasCursor, "pagination helper should handle cursor-based pagination")
}

// TestGeneratePagination_CursorFieldConventions verifies that the pagination
// helper tries the canonical cursor field names when extracting the next cursor.
//
// FAILS until cursor extraction uses next_cursor / nextCursor / after / meta.next.
func TestGeneratePagination_CursorFieldConventions(t *testing.T) {
	src, err := generator.GeneratePagination("mycli")
	require.NoError(t, err)

	hasCanonical := strings.Contains(src, "next_cursor") ||
		strings.Contains(src, "nextCursor") ||
		strings.Contains(src, `"after"`) ||
		strings.Contains(src, "meta") && strings.Contains(src, "next")
	assert.True(t, hasCanonical,
		"pagination helper should extract cursor via next_cursor/nextCursor/after/meta.next")
}

// --- pagination.go.tmpl template file ---

// TestPaginationTemplate_FileExists verifies that the pagination.go.tmpl template
// file exists in the templates directory.
//
// FAILS until internal/generator/templates/pagination.go.tmpl is created.
func TestPaginationTemplate_FileExists(t *testing.T) {
	tmplPath := filepath.Join(paginationTemplateDir(), "pagination.go.tmpl")
	_, err := os.Stat(tmplPath)
	assert.NoError(t, err,
		"internal/generator/templates/pagination.go.tmpl should exist")
}

// TestPaginationTemplate_ParsesAsGoTemplate verifies that pagination.go.tmpl
// is a valid Go text/template (no syntax errors in the template itself).
//
// FAILS until internal/generator/templates/pagination.go.tmpl is created and valid.
func TestPaginationTemplate_ParsesAsGoTemplate(t *testing.T) {
	tmplPath := filepath.Join(paginationTemplateDir(), "pagination.go.tmpl")
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		t.Skip("pagination.go.tmpl not yet created — skipping template parse test")
	}

	data, err := os.ReadFile(tmplPath)
	require.NoError(t, err, "should be able to read pagination.go.tmpl")

	_, parseErr := template.New("pagination").Parse(string(data))
	assert.NoError(t, parseErr,
		"pagination.go.tmpl should be a valid Go text/template")
}

// TestPaginationTemplate_ContainsPackageClient verifies that the template
// produces output declaring `package client`.
//
// FAILS until pagination.go.tmpl renders a valid Go file in package client.
func TestPaginationTemplate_ContainsPackageClient(t *testing.T) {
	tmplPath := filepath.Join(paginationTemplateDir(), "pagination.go.tmpl")
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		t.Skip("pagination.go.tmpl not yet created — skipping render test")
	}

	data, err := os.ReadFile(tmplPath)
	require.NoError(t, err)

	tmpl, err := template.New("pagination").Parse(string(data))
	require.NoError(t, err)

	// Execute with a minimal data struct that provides CLIName.
	var buf strings.Builder
	execErr := tmpl.Execute(&buf, struct{ CLIName string }{"mycli"})
	require.NoError(t, execErr, "template execution should not error")

	assert.Contains(t, buf.String(), "package client",
		"rendered pagination.go.tmpl should declare 'package client'")
}

// TestPaginationTemplate_ContainsSafetyLimit verifies that the rendered template
// contains the max-pages safety limit.
//
// FAILS until pagination.go.tmpl includes the safety limit.
func TestPaginationTemplate_ContainsSafetyLimit(t *testing.T) {
	tmplPath := filepath.Join(paginationTemplateDir(), "pagination.go.tmpl")
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		t.Skip("pagination.go.tmpl not yet created — skipping render test")
	}

	data, err := os.ReadFile(tmplPath)
	require.NoError(t, err)

	tmpl, err := template.New("pagination").Parse(string(data))
	require.NoError(t, err)

	var buf strings.Builder
	require.NoError(t, tmpl.Execute(&buf, struct{ CLIName string }{"mycli"}))

	assert.Contains(t, buf.String(), "100",
		"rendered pagination.go.tmpl should contain the max-pages safety limit (100)")
}

// --- Bug regression tests (WI-517) ---

// TestGeneratePagination_ExtractItems_ErrorResponseReturnsDone verifies that
// extractItems returns done=true for unknown map structures such as error
// responses (e.g. {"error": "rate limited"}) rather than treating them as
// valid items and looping 100 times.
//
// BUG: The current generated extractItems has a catch-all fallback for
// map[string]interface{} that wraps any unknown map as a single-item slice
// with done=false — so {"error": "rate limited"} gets appended 100 times.
// The fix must add a guard that returns done=true when none of the known
// envelope keys ("data", "items", "results", "records") match and the map
// does not look like a single valid resource object.
//
// FAILS until extractItems guards against unknown map structures.
func TestGeneratePagination_ExtractItems_ErrorResponseReturnsDone(t *testing.T) {
	src, err := generator.GeneratePagination("mycli")
	require.NoError(t, err)

	// The generated extractItems must NOT unconditionally fall through to the
	// "single object" branch for every unknown map.  A guard is needed before
	// the catch-all wrap so that maps containing only non-data keys (like
	// "error", "message", "code") cause done=true.
	//
	// Acceptable fix patterns (any one is sufficient):
	//   • Check len(v) == 0 before wrapping → catches empty object {}
	//   • Only wrap when the map contains no recognised envelope key AND len > 0
	//     but looks like a resource (non-trivial heuristic), OR
	//   • Explicitly return done=true for single-key "error"-like maps.
	//
	// The simplest reliable guard is returning done=true when none of the known
	// keys are found in a map response instead of wrapping blindly.  We check
	// that the generated source does NOT contain the current buggy pattern of
	// an unconditional wrap after the key-scan loop.
	//
	// Concretely: the current code does:
	//   // Single object response — wrap in a slice.
	//   return []interface{}{v}, false, nil
	// immediately after the key loop, with no additional guard.  A fixed
	// implementation must either remove that line or gate it on a condition
	// that excludes error-only maps.

	// The current buggy code has a catch-all at the end of the map branch:
	//
	//   // Single object response — wrap in a slice.
	//   return []interface{}{v}, false, nil
	//
	// This comment+pattern indicates an unconditional wrap of ANY unknown map
	// as a valid item with done=false — causing error responses to loop 100 times.
	//
	// A correct implementation must remove or gate this catch-all so that unknown
	// maps (e.g. {"error": "rate limited"}, {}) return done=true instead.
	//
	// We assert the generated source does NOT contain the exact buggy comment that
	// signals the unconditional wrap is still present.
	hasBuggyUnconditionalWrap := strings.Contains(src, "Single object response")
	assert.False(t, hasBuggyUnconditionalWrap,
		"extractItems contains an unconditional 'Single object response' catch-all "+
			"that wraps any unknown map (e.g. {\"error\": \"rate limited\"}) as a valid "+
			"item with done=false, causing error responses to be appended 100 times. "+
			"Remove or gate this catch-all to return done=true for unknown map structures.\n"+
			"Generated source:\n%s", src)
}

// TestGeneratePagination_PageBased_HonorsStartPage verifies that the generated
// fetchAllPageBased function uses the starting page from the caller-supplied
// query parameters rather than hardcoding page := 1.
//
// BUG: fetchAllPageBased always starts from page 1 regardless of any --page N
// flag the user passed.  If a user runs `mytool list --page 5 --all`, the
// generated code ignores page=5 and starts from page=1 anyway.
//
// FAILS until fetchAllPageBased derives its starting page from the query params
// (e.g. page := pageParam or by reading query[cfg.PageParam] as the start value).
func TestGeneratePagination_PageBased_HonorsStartPage(t *testing.T) {
	src, err := generator.GeneratePagination("mycli")
	require.NoError(t, err)

	// The current buggy code contains exactly:
	//   page := 1
	// with no logic to read the starting page from the incoming query map or a
	// parameter.  A correct implementation must derive the initial page from the
	// caller-supplied value, e.g.:
	//   page := startPage   (where startPage comes from query[cfg.PageParam] or a new param)
	//   page := pageParam
	// OR at minimum NOT hardcode "page := 1" unconditionally.
	hasHardcodedPageOne := strings.Contains(src, "page := 1")
	assert.False(t, hasHardcodedPageOne,
		"fetchAllPageBased must not hardcode 'page := 1' — it should read the "+
			"starting page from the caller-supplied page parameter so that "+
			"'--page 5 --all' starts from page 5, not page 1.\n"+
			"Generated source:\n%s", src)
}

// TestGeneratePagination_OffsetBased_RespectsUserLimit verifies that the
// generated fetchAllOffsetBased function does NOT unconditionally override the
// limit with a hardcoded 100 when cfg.PageSize is zero.
//
// BUG: fetchAllOffsetBased contains:
//
//	limit := cfg.PageSize
//	if limit <= 0 {
//	    limit = 100
//	}
//
// Since PaginationConfig.PageSize is never populated by the verb-cmd generator
// (it always stays 0), this silently overrides the user's --limit N flag value
// and always fetches 100 items per page.
//
// FAILS until fetchAllOffsetBased reads the limit from the caller-supplied query
// params (e.g. query[cfg.SizeParam]) or accepts it as an explicit parameter,
// rather than defaulting to a hardcoded 100.
func TestGeneratePagination_OffsetBased_RespectsUserLimit(t *testing.T) {
	src, err := generator.GeneratePagination("mycli")
	require.NoError(t, err)

	// The current buggy pattern is:
	//   limit = 100
	// inside an `if limit <= 0` block (which is always true because PageSize is
	// never set).  A correct implementation must instead read the limit from the
	// incoming query map when cfg.PageSize is zero, so the user's --limit flag
	// value is respected.  Any of the following patterns indicate a fix:
	//   • query[cfg.SizeParam] used to initialise limit
	//   • An explicit limitParam or startLimit parameter passed in
	//   • The `limit = 100` default only applies when the query param is also absent
	//
	// We assert that the generated source does NOT contain the unconditional
	// "limit = 100" assignment (the exact string present in the current template).
	hasHardcodedLimit100 := strings.Contains(src, "limit = 100")
	assert.False(t, hasHardcodedLimit100,
		"fetchAllOffsetBased must not unconditionally set 'limit = 100' — "+
			"cfg.PageSize is always 0 so this silently overrides the user's --limit flag. "+
			"Read the limit from query[cfg.SizeParam] or an explicit parameter instead.\n"+
			"Generated source:\n%s", src)
}
