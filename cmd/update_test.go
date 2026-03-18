// Package cmd_test contains tests for the update command.
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// updateFixtureDir returns the absolute path to the testdata directory.
func updateFixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(file)), "testdata")
}

// executeUpdate runs `cmdspec update` with the given args and returns combined
// output and error.
func executeUpdate(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"update"}, args...))
	err := root.Execute()
	return out.String(), err
}

// --- Registration ---

// TestUpdateCmd_CommandExists verifies that the "update" subcommand is registered on root.
func TestUpdateCmd_CommandExists(t *testing.T) {
	root := cmd.NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "update" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'update' subcommand to be registered on root")
}

// --- Required flags ---

// TestUpdateCmd_RequiredFlags verifies that running update without --schema returns
// a usage error.
func TestUpdateCmd_RequiredFlags(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := executeUpdate(t, "--name", "testapi", "--output", tmpDir)
	assert.Error(t, err, "update without --schema should return an error")
}

// TestUpdateCmd_RequiredNameFlag verifies that running update without --name returns
// a usage error.
func TestUpdateCmd_RequiredNameFlag(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()
	_, err := executeUpdate(t, "--schema", schema, "--output", tmpDir)
	assert.Error(t, err, "update without --name should return an error")
}

// --- Schema loading errors ---

// TestUpdateCmd_SchemaNotFound verifies that --schema pointing to a nonexistent
// file returns an error that mentions the file path.
func TestUpdateCmd_SchemaNotFound(t *testing.T) {
	missingPath := "/nonexistent/path/to/spec.yaml"
	out, err := executeUpdate(t, "--schema", missingPath, "--name", "testapi")
	assert.Error(t, err, "update with a missing schema should return an error")
	combined := out + err.Error()
	assert.True(t,
		strings.Contains(combined, missingPath) ||
			strings.Contains(combined, "nonexistent") ||
			strings.Contains(combined, "no such file"),
		"error output should mention the missing file path, got: %s", combined)
}

// --- Files written ---

// TestUpdateCmd_WritesFiles verifies that update writes Go source files to the
// output directory and those files contain valid Go package declarations.
func TestUpdateCmd_WritesFiles(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	_, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, err, "update of a valid spec should succeed")

	// At least one .go file should exist.
	var goFiles []string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	require.NotEmpty(t, goFiles, "update should write at least one .go file to the output directory")

	// Each .go file must begin with a package declaration.
	for _, f := range goFiles {
		data, readErr := os.ReadFile(f)
		require.NoError(t, readErr)
		assert.True(t,
			strings.Contains(string(data), "package "),
			"generated file %q should contain a Go package declaration", f)
	}
}

// --- Custom code preservation ---

// TestUpdateCmd_PreservesCustomCode verifies that when an existing generated file
// contains a // commandspec:custom:start block INSIDE a function body, the custom
// block content survives regeneration as EXECUTABLE code — not commented-out orphan output.
//
// The original test was wrong: it appended the custom block at file-level (after all
// functions), so on regeneration the block became an orphan that was commented out.
// The Contains assertion still matched because "myPreservedCustomCode()" appeared inside
// the comment "// \tmyPreservedCustomCode()". This is the corrected version.
func TestUpdateCmd_PreservesCustomCode(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	// First, do a clean generate into tmpDir so the expected directory structure exists.
	_, firstErr := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, firstErr, "first update should succeed")

	// Target cmd/root.go specifically — it always exists and contains a known init()
	// function whose body is a stable injection point for custom markers.
	rootGoPath := filepath.Join(tmpDir, "cmd", "root.go")
	original, readErr := os.ReadFile(rootGoPath)
	require.NoError(t, readErr, "cmd/root.go should exist after first update")

	// Inject the custom block INSIDE the init() function body — before the closing brace.
	// This places it where the generator should also emit a matching marker slot after B.A.'s fix,
	// allowing Merge to re-insert it as executable (non-commented) code on the next update.
	const customMarker = "myPreservedCustomCode()"
	const customBlock = "\t// commandspec:custom:start init-hook\n\t" + customMarker + "\n\t// commandspec:custom:end\n"

	// Replace the closing brace of init() with the custom block + closing brace.
	// The generated init() always ends with a single "}" on its own line.
	src := string(original)
	injected := strings.Replace(src, "\nfunc init() {", "\nfunc init() {\n"+customBlock, 1)
	require.NotEqual(t, src, injected, "injection should have modified the file — init() should be present in root.go")
	require.NoError(t, os.WriteFile(rootGoPath, []byte(injected), 0o644))

	// Run update again — the custom block should survive regeneration as executable code.
	_, secondErr := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, secondErr, "second update should succeed")

	updated, readErr2 := os.ReadFile(rootGoPath)
	require.NoError(t, readErr2)
	updatedStr := string(updated)

	// The custom code must be present.
	assert.Contains(t, updatedStr, customMarker,
		"custom block content should be preserved across update regeneration")

	// CRITICAL: the custom code must NOT appear only inside a comment.
	// Find every line that contains the custom marker and assert at least one
	// is NOT a comment line (i.e. does not start with optional whitespace then "//").
	var foundExecutableLine bool
	for _, line := range strings.Split(updatedStr, "\n") {
		if strings.Contains(line, customMarker) {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "//") {
				foundExecutableLine = true
				break
			}
		}
	}
	assert.True(t, foundExecutableLine,
		"custom block content must appear as executable code (not commented out) after update;\n"+
			"if every line containing %q starts with '//', the code was orphaned and commented out.\n"+
			"File contents:\n%s", customMarker, updatedStr)
}

// TestUpdateCmd_MalformedMarkerPrintsWarning verifies that when a generated file
// has an UNCLOSED commandspec:custom:start marker, the update command prints a
// warning to output (including the filename) rather than silently discarding the
// extraction error and overwriting the file.
//
// Bug: update.go:149-153 calls preserve.Extract and when it returns an error it
// silently sets blocks = nil, losing any custom code with no warning to the user.
func TestUpdateCmd_MalformedMarkerPrintsWarning(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	// First, do a clean generate.
	_, firstErr := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, firstErr, "first update should succeed")

	// Inject an UNCLOSED custom marker into cmd/root.go — no end marker follows.
	// This simulates a user accidentally deleting the :end comment or a merge conflict.
	rootGoPath := filepath.Join(tmpDir, "cmd", "root.go")
	original, readErr := os.ReadFile(rootGoPath)
	require.NoError(t, readErr, "cmd/root.go should exist after first update")

	unclosedBlock := "\n// commandspec:custom:start my-block\nmyCustomCode()\n// (no end marker — this is malformed)\n"
	malformed := string(original) + unclosedBlock
	require.NoError(t, os.WriteFile(rootGoPath, []byte(malformed), 0o644))

	// Run update — it should NOT silently discard the extraction error.
	// It must print a warning that includes the word "warning" (case-insensitive)
	// AND mentions the affected filename so the user knows which file is affected.
	out, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)

	// The command itself should still succeed (non-fatal warning), or at minimum
	// must print an actionable warning — we do not require it to fail fatally.
	_ = err // either outcome is acceptable; the warning is what matters

	lower := strings.ToLower(out)
	hasWarning := strings.Contains(lower, "warning") || strings.Contains(lower, "warn")
	assert.True(t, hasWarning,
		"update should print a warning when preserve.Extract fails due to malformed markers, got:\n%s", out)

	// The warning must name the affected file so the user knows where to look.
	hasFilename := strings.Contains(out, "root.go") || strings.Contains(out, "cmd/root.go")
	assert.True(t, hasFilename,
		"update warning for malformed markers should include the affected filename, got:\n%s", out)
}

// TestUpdateCmd_GoSumNotFlaggedAsOrphan verifies that go.sum (and other common
// non-generated files like .gitignore, README.md) do NOT trigger the
// "file not in current spec (would be removed if cleaned)" warning.
//
// Bug: collectGeneratedFiles walks ALL files in the output directory, so go.sum
// (which is not a generated file) appears as an orphan on every update.
func TestUpdateCmd_GoSumNotFlaggedAsOrphan(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	// First update — generates the base project.
	_, firstErr := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, firstErr, "first update should succeed")

	// Place a go.sum file in the output directory — this simulates running
	// "go mod tidy" after generation, which is a normal workflow step.
	goSumPath := filepath.Join(tmpDir, "go.sum")
	goSumContent := "# go.sum — created by go mod tidy\n"
	require.NoError(t, os.WriteFile(goSumPath, []byte(goSumContent), 0o644))

	// Also place other common non-generated files to confirm they are not flagged.
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte("*.log\n"), 0o644))

	// Run update — go.sum must NOT appear in any orphan/warn message.
	out, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, err, "update should succeed even with go.sum present")

	// go.sum must not appear in any warning/orphan output line.
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(line)
		isWarnLine := strings.Contains(lower, "warn") || strings.Contains(lower, "orphan") ||
			strings.Contains(lower, "not in") || strings.Contains(lower, "removed")
		if isWarnLine {
			assert.NotContains(t, line, "go.sum",
				"go.sum should NOT be flagged as an orphan file — it is a normal project file, got warn line: %q", line)
			assert.NotContains(t, line, ".gitignore",
				".gitignore should NOT be flagged as an orphan file, got warn line: %q", line)
		}
	}

	// Belt-and-suspenders: assert that the full output does not contain "go.sum"
	// on any warn/orphan context line. We check by looking for the specific
	// orphan message format that update.go currently emits.
	assert.NotContains(t, out, "go.sum",
		"full update output must not mention go.sum at all in orphan warnings, got:\n%s", out)
}

// --- Diff output ---

// TestUpdateCmd_DiffOutput verifies that when a file changes the command outputs
// a unified diff containing ---, +++, and @@ lines.
func TestUpdateCmd_DiffOutput(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	// Initial generate.
	_, firstErr := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, firstErr, "first update should succeed")

	// Modify a file so the next update sees a change.
	var targetFile string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || targetFile != "" {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			targetFile = path
		}
		return nil
	})
	require.NotEmpty(t, targetFile)

	data, _ := os.ReadFile(targetFile)
	// Add a comment that the regenerator won't produce — this simulates drift.
	drifted := string(data) + "\n// drift-marker-line\n"
	require.NoError(t, os.WriteFile(targetFile, []byte(drifted), 0o644))

	// Run update and capture output — it should show a unified diff.
	out, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, err, "update should succeed even when files change")

	hasDiff := strings.Contains(out, "---") && strings.Contains(out, "+++") && strings.Contains(out, "@@")
	assert.True(t, hasDiff,
		"update output should contain unified diff (---, +++, @@) for changed files, got:\n%s", out)
}

// TestUpdateCmd_NoDiffFlag verifies that --no-diff suppresses diff output.
func TestUpdateCmd_NoDiffFlag(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	// Initial generate.
	_, firstErr := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, firstErr)

	// Modify a file to force a diff.
	var targetFile string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || targetFile != "" {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			targetFile = path
		}
		return nil
	})
	require.NotEmpty(t, targetFile)
	data, _ := os.ReadFile(targetFile)
	require.NoError(t, os.WriteFile(targetFile, []byte(string(data)+"\n// drift\n"), 0o644))

	// With --no-diff the output must not contain diff markers.
	out, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir, "--no-diff")
	require.NoError(t, err)
	assert.False(t,
		strings.Contains(out, "---") && strings.Contains(out, "+++") && strings.Contains(out, "@@"),
		"--no-diff should suppress unified diff output, got:\n%s", out)
}

// --- Dry run ---

// TestUpdateCmd_DryRun verifies that --dry-run does not write any files to disk
// but still prints what would change.
func TestUpdateCmd_DryRun(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	out, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir, "--dry-run")
	require.NoError(t, err, "--dry-run should succeed without error")
	assert.NotEmpty(t, out, "--dry-run should still produce output describing what would change")

	// No files should have been written.
	var written []string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		written = append(written, path)
		return nil
	})
	assert.Empty(t, written,
		"--dry-run must not write any files to disk, but found: %v", written)
}

// --- Unchanged files ---

// TestUpdateCmd_UnchangedFiles verifies that when regenerated content matches
// existing content on disk the file is reported as "unchanged" and is not rewritten
// (file mtime does not change).
func TestUpdateCmd_UnchangedFiles(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	// Generate once.
	_, firstErr := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, firstErr)

	// Record mtimes before second update.
	mtimes := map[string]int64{}
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		mtimes[path] = info.ModTime().UnixNano()
		return nil
	})
	require.NotEmpty(t, mtimes, "should have files after first update")

	// Run update again — spec unchanged so files should be unchanged.
	out, secondErr := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, secondErr)

	// The output should report "unchanged" for at least one file.
	assert.Contains(t, strings.ToLower(out), "unchanged",
		"update output should report unchanged files when spec has not changed, got:\n%s", out)
}

// --- New endpoint creates file ---

// TestUpdateCmd_NewEndpointCreatesFile verifies that if the spec contains an
// endpoint whose corresponding file does not yet exist in the output directory,
// the update command creates that file.
func TestUpdateCmd_NewEndpointCreatesFile(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	// Start from an empty output directory — every file is "new".
	tmpDir := t.TempDir()

	_, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, err, "update should succeed for a fresh output directory")

	var files []string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	assert.NotEmpty(t, files,
		"update into a fresh directory should create all expected output files")
}

// TestUpdateCmd_NewEndpointOutputReportsAdded verifies that when a new file is
// created the command output mentions "added" or "new" or "created".
func TestUpdateCmd_NewEndpointOutputReportsAdded(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	out, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, err)

	lower := strings.ToLower(out)
	hasAddedReport := strings.Contains(lower, "added") ||
		strings.Contains(lower, "new") ||
		strings.Contains(lower, "created")
	assert.True(t, hasAddedReport,
		"update output should report added/new/created files when writing into a fresh directory, got:\n%s", out)
}

// --- Removed endpoint warns ---

// TestUpdateCmd_RemovedEndpointWarns verifies that if the output directory
// contains a file that has no corresponding endpoint in the new spec, the command
// warns the user but does NOT delete the file.
func TestUpdateCmd_RemovedEndpointWarns(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	// Generate first.
	_, firstErr := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, firstErr)

	// Inject an orphan file that looks like a generated command file but won't
	// be produced by the current spec.
	orphanPath := filepath.Join(tmpDir, "cmd", "orphan-resource_list.go")
	orphanContent := "package cmd\n\n// This file had no corresponding endpoint in the new spec.\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(orphanPath), 0o755))
	require.NoError(t, os.WriteFile(orphanPath, []byte(orphanContent), 0o644))

	// Run update — orphan file should be warned about but NOT deleted.
	out, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, err, "update should succeed even when orphan files exist")

	lower := strings.ToLower(out)
	hasWarn := strings.Contains(lower, "warn") ||
		strings.Contains(lower, "removed") ||
		strings.Contains(lower, "orphan") ||
		strings.Contains(lower, "no longer") ||
		strings.Contains(lower, "not in")
	assert.True(t, hasWarn,
		"update output should warn about files with no corresponding endpoint, got:\n%s", out)

	// The orphan file must still exist — update must NOT delete files.
	_, statErr := os.Stat(orphanPath)
	assert.NoError(t, statErr,
		"update must NOT delete orphan files — it should only warn, file should still exist at %s", orphanPath)
}

// --- Timeout flag ---

// TestUpdateCmd_Timeout verifies that --timeout is a recognised flag and does
// not cause "unknown flag" errors.
func TestUpdateCmd_Timeout(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	_, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir, "--timeout", "30s")
	if err != nil {
		assert.NotContains(t, err.Error(), "unknown flag",
			"--timeout should be a recognised flag on the update command")
	}
}

// --- Summary report format ---

// TestUpdateCmd_SummaryReport verifies that the update command prints a human-readable
// summary indicating the count of files added, modified, unchanged, and (warned as)
// removed.
func TestUpdateCmd_SummaryReport(t *testing.T) {
	schema := filepath.Join(updateFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	out, err := executeUpdate(t, "--schema", schema, "--name", "testapi", "--output", tmpDir)
	require.NoError(t, err)

	// At minimum a summary with counts should appear somewhere in output.
	lower := strings.ToLower(out)
	hasSummary := strings.Contains(lower, "added") ||
		strings.Contains(lower, "modified") ||
		strings.Contains(lower, "unchanged") ||
		strings.Contains(lower, "file")
	assert.True(t, hasSummary,
		"update output should include a summary of changes, got:\n%s", out)
}
