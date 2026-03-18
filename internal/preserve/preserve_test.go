package preserve_test

import (
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/preserve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CustomBlock struct ---

// TestCustomBlockFields verifies that CustomBlock has Label, Content, and Context fields.
func TestCustomBlockFields(t *testing.T) {
	b := preserve.CustomBlock{
		Label:   "my-hook",
		Content: "// do something\n",
		Context: "RunE",
	}
	assert.Equal(t, "my-hook", b.Label)
	assert.Equal(t, "// do something\n", b.Content)
	assert.Equal(t, "RunE", b.Context)
}

// --- Extract ---

// TestExtract_SingleLabeledBlock verifies extraction of a single labeled custom block.
func TestExtract_SingleLabeledBlock(t *testing.T) {
	src := `package main

func RunE() error {
	// commandspec:custom:start my-hook
	doSomethingCustom()
	// commandspec:custom:end
	return nil
}
`
	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	assert.Equal(t, "my-hook", blocks[0].Label)
	assert.Contains(t, blocks[0].Content, "doSomethingCustom()")
	assert.Equal(t, "RunE", blocks[0].Context, "context should be the enclosing function name")
}

// TestExtract_SingleUnlabeledBlock verifies extraction of a block without a label.
func TestExtract_SingleUnlabeledBlock(t *testing.T) {
	src := `package main

func Execute() {
	// commandspec:custom:start
	customInit()
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	assert.Empty(t, blocks[0].Label, "unlabeled block should have empty Label")
	assert.Contains(t, blocks[0].Content, "customInit()")
	assert.Equal(t, "Execute", blocks[0].Context)
}

// TestExtract_MultipleBlocks verifies extraction of multiple custom blocks.
func TestExtract_MultipleBlocks(t *testing.T) {
	src := `package main

func Alpha() {
	// commandspec:custom:start alpha-hook
	alphaCustom()
	// commandspec:custom:end
}

func Beta() {
	// commandspec:custom:start beta-hook
	betaCustom()
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	require.Len(t, blocks, 2)

	labels := map[string]bool{}
	for _, b := range blocks {
		labels[b.Label] = true
	}
	assert.True(t, labels["alpha-hook"], "should extract alpha-hook block")
	assert.True(t, labels["beta-hook"], "should extract beta-hook block")
}

// TestExtract_MultipleBlocksInSameFunction verifies that two blocks inside the
// same function are both extracted with the same Context.
func TestExtract_MultipleBlocksInSameFunction(t *testing.T) {
	src := `package main

func BigFunc() {
	// commandspec:custom:start before-call
	preHook()
	// commandspec:custom:end
	doWork()
	// commandspec:custom:start after-call
	postHook()
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	require.Len(t, blocks, 2)

	for _, b := range blocks {
		assert.Equal(t, "BigFunc", b.Context,
			"both blocks should report 'BigFunc' as context")
	}
}

// TestExtract_FileLevelBlock verifies a block at file level (outside any function)
// has an empty Context.
func TestExtract_FileLevelBlock(t *testing.T) {
	src := `package main

// commandspec:custom:start file-level
var customVar = "hello"
// commandspec:custom:end
`
	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	assert.Equal(t, "file-level", blocks[0].Label)
	assert.Empty(t, blocks[0].Context, "file-level block should have empty Context")
}

// TestExtract_EmptyBlock verifies that an empty block (no content between markers)
// is extracted without error.
func TestExtract_EmptyBlock(t *testing.T) {
	src := `package main

func Foo() {
	// commandspec:custom:start empty-hook
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	assert.Equal(t, "empty-hook", blocks[0].Label)
	assert.Empty(t, strings.TrimSpace(blocks[0].Content),
		"empty block should have empty or whitespace-only Content")
}

// TestExtract_NoBlocks verifies that a source without any markers returns empty slice.
func TestExtract_NoBlocks(t *testing.T) {
	src := `package main

func main() {
	println("hello")
}
`
	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	assert.Empty(t, blocks)
}

// TestExtract_UnclosedBlockReturnsError verifies that an unclosed start marker
// returns an error.
func TestExtract_UnclosedBlockReturnsError(t *testing.T) {
	src := `package main

func Foo() {
	// commandspec:custom:start unclosed
	doSomething()
	// no end marker
}
`
	_, err := preserve.Extract(src)
	assert.Error(t, err, "unclosed custom block should return an error")
}

// TestExtract_NestedStartReturnsError verifies that a nested start marker
// (start inside an already-open block) returns an error.
func TestExtract_NestedStartReturnsError(t *testing.T) {
	src := `package main

func Foo() {
	// commandspec:custom:start outer
	// commandspec:custom:start inner
	doSomething()
	// commandspec:custom:end
	// commandspec:custom:end
}
`
	_, err := preserve.Extract(src)
	assert.Error(t, err, "nested custom blocks should return an error")
}

// TestExtract_EndWithoutStartReturnsError verifies that an end marker without
// a preceding start marker returns an error.
func TestExtract_EndWithoutStartReturnsError(t *testing.T) {
	src := `package main

func Foo() {
	doSomething()
	// commandspec:custom:end
}
`
	_, err := preserve.Extract(src)
	assert.Error(t, err, "end marker without start marker should return an error")
}

// TestExtract_LabelPreservesWhitespace verifies that whitespace in labels is
// handled correctly (trimmed).
func TestExtract_LabelTrimmed(t *testing.T) {
	src := `package main

func Foo() {
	// commandspec:custom:start   trimmed-label
	custom()
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	assert.Equal(t, "trimmed-label", blocks[0].Label,
		"label should be trimmed of surrounding whitespace")
}

// --- Merge ---

// TestMerge_RoundTrip verifies that extract → merge on identical source yields identical output.
func TestMerge_RoundTrip(t *testing.T) {
	src := `package main

func RunE() error {
	// commandspec:custom:start my-hook
	doSomethingCustom()
	// commandspec:custom:end
	return nil
}
`
	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	result, err := preserve.Merge(src, blocks)
	require.NoError(t, err)

	// The result should contain the custom code.
	assert.Contains(t, result, "doSomethingCustom()",
		"merged result should contain the preserved custom code")
	// The markers should still be present.
	assert.Contains(t, result, "commandspec:custom:start")
	assert.Contains(t, result, "commandspec:custom:end")
}

// TestMerge_InsertsBlockByLabel verifies that Merge re-inserts a block matched
// by label into freshly generated source.
func TestMerge_InsertsBlockByLabel(t *testing.T) {
	oldSrc := `package main

func RunE() error {
	// commandspec:custom:start my-hook
	myCustomCode()
	// commandspec:custom:end
	return nil
}
`
	blocks, err := preserve.Extract(oldSrc)
	require.NoError(t, err)

	// Fresh generation — has the marker placeholders but empty content.
	newSrc := `package main

func RunE() error {
	// commandspec:custom:start my-hook
	// commandspec:custom:end
	return nil
}
`

	result, err := preserve.Merge(newSrc, blocks)
	require.NoError(t, err)
	assert.Contains(t, result, "myCustomCode()",
		"merged result should contain the preserved custom code from the old source")
}

// TestMerge_InsertsBlockByContext verifies that an unlabeled block is matched
// to a destination by the enclosing function name (Context).
func TestMerge_InsertsBlockByContext(t *testing.T) {
	oldSrc := `package main

func Execute() {
	// commandspec:custom:start
	contextCustom()
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(oldSrc)
	require.NoError(t, err)

	newSrc := `package main

func Execute() {
	// commandspec:custom:start
	// commandspec:custom:end
}
`
	result, err := preserve.Merge(newSrc, blocks)
	require.NoError(t, err)
	assert.Contains(t, result, "contextCustom()",
		"unlabeled block should be merged by context (function name)")
}

// TestMerge_OrphanedBlockProducesWarningComment verifies that a block whose
// matching function has been removed from the new source produces a warning comment.
func TestMerge_OrphanedBlockProducesWarningComment(t *testing.T) {
	oldSrc := `package main

func RemovedFunc() {
	// commandspec:custom:start orphan-hook
	orphanedCode()
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(oldSrc)
	require.NoError(t, err)

	// New source doesn't have RemovedFunc at all.
	newSrc := `package main

func OtherFunc() {
	println("hello")
}
`
	result, err := preserve.Merge(newSrc, blocks)
	require.NoError(t, err)

	// The orphaned block content should still appear with a warning.
	assert.Contains(t, result, "orphanedCode()",
		"orphaned block content should be preserved in the output")
	lower := strings.ToLower(result)
	assert.True(t,
		strings.Contains(lower, "warn") ||
			strings.Contains(lower, "orphan") ||
			strings.Contains(lower, "preserved") ||
			strings.Contains(lower, "removed"),
		"orphaned block should produce a warning comment, got:\n%s", result)
}

// TestMerge_NoBlocksReturnsSameSource verifies that Merge with empty blocks
// returns the new source unchanged.
func TestMerge_NoBlocksReturnsSameSource(t *testing.T) {
	newSrc := `package main

func Foo() {
	println("no custom blocks")
}
`
	result, err := preserve.Merge(newSrc, nil)
	require.NoError(t, err)
	assert.Equal(t, newSrc, result,
		"Merge with no blocks should return the source unchanged")
}

// TestMerge_DuplicateLabels verifies the behavior when two custom blocks in the
// old source share the same label (e.g. "dupe-hook"), and the new source also
// contains two marker pairs with that same label.
//
// Current bug: the second block overwrites the first in the byLabel map during
// Extract; after Merge the first block's content is orphaned as a WARNING comment
// and the second marker in newSource gets empty content — so the first block's
// custom code is effectively lost from its intended location.
//
// Acceptable outcomes (either satisfies the contract):
//  1. Merge returns an error indicating duplicate labels are not allowed.
//  2. Merge succeeds and BOTH blocks' content appears in the merged output
//     (i.e. each destination marker gets the content of the block from the
//     corresponding source position).
//
// This test FAILS until the implementation handles duplicate labels without
// silently dropping one block's content.
func TestMerge_DuplicateLabels(t *testing.T) {
	// Old source: two blocks with the same label "dupe-hook" in different functions.
	oldSrc := `package main

func Alpha() {
	// commandspec:custom:start dupe-hook
	alphaCustomCode()
	// commandspec:custom:end
}

func Beta() {
	// commandspec:custom:start dupe-hook
	betaCustomCode()
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(oldSrc)
	require.NoError(t, err, "Extract should not error on duplicate labels — it just collects blocks")
	require.Len(t, blocks, 2, "should extract both blocks even when labels are duplicated")

	// New source: two empty marker pairs with the same label "dupe-hook".
	newSrc := `package main

func Alpha() {
	// commandspec:custom:start dupe-hook
	// commandspec:custom:end
}

func Beta() {
	// commandspec:custom:start dupe-hook
	// commandspec:custom:end
}
`

	result, mergeErr := preserve.Merge(newSrc, blocks)

	if mergeErr != nil {
		// Outcome 1: Merge rejects duplicate labels with an explicit error.
		// This is acceptable — the caller can warn the user to use unique labels.
		// The error message should mention "duplicate" or "label".
		lower := strings.ToLower(mergeErr.Error())
		assert.True(t,
			strings.Contains(lower, "duplicate") ||
				strings.Contains(lower, "label") ||
				strings.Contains(lower, "dupe-hook"),
			"error for duplicate labels should mention 'duplicate', 'label', or the label name, got: %v", mergeErr)
		return
	}

	// Outcome 2: Merge succeeds — both blocks' content must appear in output.
	// Neither alphaCustomCode nor betaCustomCode may be silently discarded.
	assert.Contains(t, result, "alphaCustomCode()",
		"alphaCustomCode() content must appear in merged output — duplicate label must not cause silent data loss")
	assert.Contains(t, result, "betaCustomCode()",
		"betaCustomCode() content must appear in merged output — duplicate label must not cause silent data loss")
}

// --- Bug regression tests ---

// TestExtract_CRLFSourceNormalizesContent documents Bug 1: strings.Split(source, "\n")
// on CRLF input leaves a trailing \r on every content line. The extracted
// CustomBlock.Content must NOT contain \r characters regardless of the source
// line-ending style.
//
// This test FAILS until the implementation normalises CRLF before splitting.
func TestExtract_CRLFSourceNormalizesContent(t *testing.T) {
	// Build the source string with Windows CRLF (\r\n) line endings explicitly.
	src := "package main\r\n" +
		"\r\n" +
		"func RunE() error {\r\n" +
		"\t// commandspec:custom:start crlf-hook\r\n" +
		"\tdoCRLFThing()\r\n" +
		"\t// commandspec:custom:end\r\n" +
		"\treturn nil\r\n" +
		"}\r\n"

	blocks, err := preserve.Extract(src)
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	// The extracted content must not carry \r — mixed line endings corrupt
	// any file the content is later injected into.
	assert.NotContains(t, blocks[0].Content, "\r",
		"extracted Content must not contain \\r characters when source uses CRLF line endings; "+
			"strings.Split on '\\n' leaves trailing \\r on each content line (Bug 1)")
}

// TestMerge_OrphanedBlockContentIsCommented documents Bug 2: when a custom block
// is orphaned (its enclosing function was removed from the new source), Merge
// currently appends the raw Go statement lines at package scope between the
// marker comments. That produces invalid Go — a non-declaration statement
// outside a function body.
//
// Acceptable fixes (any one satisfies the contract):
//
//	(a) All content lines are commented out with "//".
//	(b) Content is wrapped in an anonymous function / func literal.
//	(c) Content lines are omitted entirely; only warning/marker comments appear.
//
// The test asserts the raw uncommented function-body statement does NOT appear
// at file scope in the merged output.
//
// This test FAILS until the implementation guards orphaned content from being
// emitted as bare statements at package scope.
func TestMerge_OrphanedBlockContentIsCommented(t *testing.T) {
	oldSrc := `package main

func RemovedFunc() {
	// commandspec:custom:start orphan-compile-bug
	someGoStatement()
	x := computeSomething()
	_ = x
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(oldSrc)
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	// New source does NOT contain RemovedFunc — the block is orphaned.
	newSrc := `package main

func OtherFunc() {
	println("hello")
}
`
	result, err := preserve.Merge(newSrc, blocks)
	require.NoError(t, err,
		"Merge must not error on orphaned blocks — it should handle them gracefully")

	// The merged file must be valid Go at the syntactic level: bare function-call
	// statements (e.g. "someGoStatement()") must not appear at package scope.
	// They are only legal inside a function body.
	//
	// We verify this by checking that every line containing the raw statement is
	// either absent, or starts with "//" (commented out), ensuring the output
	// won't cause a compile error like "non-declaration statement outside function body".
	for _, line := range strings.Split(result, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "someGoStatement()" || trimmed == "x := computeSomething()" || trimmed == "_ = x" {
			t.Errorf("Bug 2: orphaned content line %q appears uncommented at package scope in merged output; "+
				"this produces a compile error. Lines must be commented out, wrapped, or omitted.\n\nFull output:\n%s",
				trimmed, result)
		}
	}
}

// TestMerge_MultipleBlocksAllPreserved verifies that all blocks are re-inserted.
func TestMerge_MultipleBlocksAllPreserved(t *testing.T) {
	oldSrc := `package main

func Alpha() {
	// commandspec:custom:start alpha-hook
	alphaCode()
	// commandspec:custom:end
}

func Beta() {
	// commandspec:custom:start beta-hook
	betaCode()
	// commandspec:custom:end
}
`
	blocks, err := preserve.Extract(oldSrc)
	require.NoError(t, err)
	require.Len(t, blocks, 2)

	// Fresh source with empty marker blocks.
	newSrc := `package main

func Alpha() {
	// commandspec:custom:start alpha-hook
	// commandspec:custom:end
}

func Beta() {
	// commandspec:custom:start beta-hook
	// commandspec:custom:end
}
`
	result, err := preserve.Merge(newSrc, blocks)
	require.NoError(t, err)
	assert.Contains(t, result, "alphaCode()", "alpha block should be preserved")
	assert.Contains(t, result, "betaCode()", "beta block should be preserved")
}
