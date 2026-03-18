// Package cmd_test contains tests for the preview command.
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/theaiteam-dev/commandspec/cmd"
	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/theaiteam-dev/commandspec/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// previewFixtureDir returns the absolute path to the testdata directory.
func previewFixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(file)), "testdata")
}

// executePreview runs `cmdspec preview` with the given args and returns
// combined output and error.
func executePreview(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"preview"}, args...))
	err := root.Execute()
	return out.String(), err
}

// TestPreview_CommandExists verifies the "preview" subcommand is registered on root.
func TestPreview_CommandExists(t *testing.T) {
	root := cmd.NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "preview" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'preview' subcommand to be registered on root")
}

// TestPreview_RequiresSchemaFlag verifies that preview without --schema returns an error.
func TestPreview_RequiresSchemaFlag(t *testing.T) {
	_, err := executePreview(t, "--name", "myapi")
	assert.Error(t, err, "preview without --schema should return an error")
}

// TestPreview_RequiresNameFlag verifies that preview without --name returns an error.
func TestPreview_RequiresNameFlag(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")
	_, err := executePreview(t, "--schema", schema)
	assert.Error(t, err, "preview without --name should return an error")
}

// TestPreview_NoFilesWritten verifies that preview does NOT write any files to disk.
func TestPreview_NoFilesWritten(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	require.NoError(t, os.Chdir(tmpDir))

	out, err := executePreview(t, "--schema", schema, "--name", "testapi")
	require.NoError(t, err, "preview of valid spec should succeed")
	assert.NotEmpty(t, out, "preview should produce output")

	// No files should have been written under tmpDir.
	var written []string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		written = append(written, path)
		return nil
	})
	assert.Empty(t, written, "preview must not write any files to disk, found: %v", written)
}

// TestPreview_OutputListsMainGo verifies that main.go appears in preview output.
func TestPreview_OutputListsMainGo(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")
	out, err := executePreview(t, "--schema", schema, "--name", "testapi")
	require.NoError(t, err)
	assert.Contains(t, out, "main.go", "preview output should list main.go")
}

// TestPreview_OutputListsGoMod verifies that go.mod appears in preview output.
func TestPreview_OutputListsGoMod(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")
	out, err := executePreview(t, "--schema", schema, "--name", "testapi")
	require.NoError(t, err)
	assert.Contains(t, out, "go.mod", "preview output should list go.mod")
}

// TestPreview_OutputListsCmdDir verifies that cmd/ files appear in preview output.
func TestPreview_OutputListsCmdDir(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")
	out, err := executePreview(t, "--schema", schema, "--name", "testapi")
	require.NoError(t, err)
	assert.True(t, strings.Contains(out, "cmd/") || strings.Contains(out, "cmd\\"),
		"preview output should list cmd/ directory files, got:\n%s", out)
}

// TestPreview_OutputListsInternalDir verifies that internal/ files appear in preview output.
func TestPreview_OutputListsInternalDir(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")
	out, err := executePreview(t, "--schema", schema, "--name", "testapi")
	require.NoError(t, err)
	assert.True(t, strings.Contains(out, "internal/") || strings.Contains(out, "internal\\"),
		"preview output should list internal/ directory files, got:\n%s", out)
}

// TestPreview_OutputShowsResourceCount verifies that resource/command count appears in output.
func TestPreview_OutputShowsResourceCount(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "petstore.json")
	out, err := executePreview(t, "--schema", schema, "--name", "petstore")
	require.NoError(t, err)
	lower := strings.ToLower(out)
	hasCount := strings.Contains(lower, "resource") || strings.Contains(lower, "command")
	assert.True(t, hasCount,
		"preview output should mention resources or commands, got:\n%s", out)
}

// TestPreview_OutputShowsSecuritySchemeBearer verifies that bearer auth appears in output
// for a spec that defines bearerAuth.
func TestPreview_OutputShowsSecuritySchemeBearer(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "petstore.json")
	out, err := executePreview(t, "--schema", schema, "--name", "petstore")
	require.NoError(t, err)
	lower := strings.ToLower(out)
	hasSecurity := strings.Contains(lower, "bearer") ||
		strings.Contains(lower, "security") ||
		strings.Contains(lower, "auth")
	assert.True(t, hasSecurity,
		"preview output should mention security/auth for a spec with bearerAuth, got:\n%s", out)
}

// TestPreview_OutputShowsSecuritySchemeAPIKey verifies that API key auth appears
// in output for a spec that defines apiKey auth.
func TestPreview_OutputShowsSecuritySchemeAPIKey(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "apikey_auth.json")
	out, err := executePreview(t, "--schema", schema, "--name", "testapi")
	require.NoError(t, err)
	lower := strings.ToLower(out)
	hasSecurity := strings.Contains(lower, "api key") ||
		strings.Contains(lower, "apikey") ||
		strings.Contains(lower, "security") ||
		strings.Contains(lower, "auth")
	assert.True(t, hasSecurity,
		"preview output should mention security/auth for a spec with apiKey, got:\n%s", out)
}

// TestPreview_InvalidSchemaReturnsError verifies that an invalid schema causes an error.
func TestPreview_InvalidSchemaReturnsError(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "invalid.json")
	_, err := executePreview(t, "--schema", schema, "--name", "testapi")
	assert.Error(t, err, "preview with an invalid schema should return an error")
}

// TestPreview_NonExistentSchemaReturnsError verifies that a missing schema file causes an error.
func TestPreview_NonExistentSchemaReturnsError(t *testing.T) {
	_, err := executePreview(t, "--schema", "/does/not/exist/spec.json", "--name", "testapi")
	assert.Error(t, err, "preview with a missing schema file should return an error")
}

// TestPreview_AcceptsTimeoutFlag verifies that --timeout is a recognised flag.
func TestPreview_AcceptsTimeoutFlag(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")
	_, err := executePreview(t, "--schema", schema, "--name", "testapi", "--timeout", "30s")
	// We only care that "unknown flag: --timeout" is NOT the error.
	if err != nil {
		assert.NotContains(t, err.Error(), "unknown flag",
			"--timeout flag should be accepted by the preview command")
	}
}

// TestPreview_ExitCodeZeroOnSuccess verifies exit code 0 for a valid spec.
func TestPreview_ExitCodeZeroOnSuccess(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")
	_, err := executePreview(t, "--schema", schema, "--name", "testapi")
	assert.NoError(t, err, "preview of a valid spec should exit with code 0")
}

// --- Amy's bug findings ---

// TestPreview_ConcurrentTimeoutNoRace verifies that running preview concurrently
// with different --timeout values does not produce a data race on the package-level
// parser.httpTimeout global.
//
// This test FAILS under `go test -race` until the implementation passes timeout
// per-call (e.g. via context or a local http.Client) rather than storing it in a
// shared package-level variable via parser.SetHTTPTimeout.
func TestPreview_ConcurrentTimeoutNoRace(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")

	timeouts := []string{"5s", "10s", "15s", "20s", "25s"}

	var wg sync.WaitGroup
	for _, timeout := range timeouts {
		timeout := timeout // capture loop var
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine sets a different timeout and immediately loads the spec.
			// If SetHTTPTimeout writes to a shared global while another goroutine reads
			// it in loadFromURL, the race detector will fire.
			executePreview(t, "--schema", schema, "--name", "testapi", "--timeout", timeout) //nolint:errcheck
		}()
	}
	wg.Wait()
	// If we reach here under -race without the detector tripping, the fix is in place.
}

// TestPreview_InvalidName_SpecialChars verifies that --name containing characters
// that are invalid for a CLI project name (e.g. "@") returns a non-zero exit.
//
// This test FAILS until the preview command validates --name with the same rules
// as generator.Generate() — currently preview returns exit 0 for "my@api".
func TestPreview_InvalidName_SpecialChars(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")

	invalidNames := []string{
		"my@api",  // @ is not allowed
		"my api",  // space is not allowed
		"my!api",  // ! is not allowed
		"my/api",  // slash is not allowed
		"my(api)", // parens are not allowed
	}

	for _, name := range invalidNames {
		name := name
		t.Run(name, func(t *testing.T) {
			_, err := executePreview(t, "--schema", schema, "--name", name)
			assert.Error(t, err,
				"preview with invalid name %q should return a non-zero exit code", name)
		})
	}
}

// TestPreview_InvalidName_GoKeyword verifies that --name set to a reserved Go
// keyword (e.g. "go", "func", "type") returns a non-zero exit.
//
// This test FAILS until the preview command validates --name against Go keywords
// (currently preview returns exit 0 for these values even though generator.Generate
// would reject them).
func TestPreview_InvalidName_GoKeyword(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")

	keywords := []string{"go", "func", "type", "var", "import", "package"}

	for _, kw := range keywords {
		kw := kw
		t.Run(kw, func(t *testing.T) {
			_, err := executePreview(t, "--schema", schema, "--name", kw)
			assert.Error(t, err,
				"preview with Go keyword name %q should return a non-zero exit code", kw)
		})
	}
}

// TestPreview_FileListMatchesGenerate verifies that every file generator.Generate()
// would write is mentioned in the `preview` output — ensuring Preview() stays in
// sync with Generate() when new files are added to the generator.
//
// This test FAILS until Preview() derives its file list from the same code path
// as Generate() (e.g. a shared DryRun helper) rather than a hardcoded subset.
func TestPreview_FileListMatchesGenerate(t *testing.T) {
	schema := filepath.Join(previewFixtureDir(), "minimal.json")
	const name = "synctest"

	// Run preview and capture listed files from output.
	out, err := executePreview(t, "--schema", schema, "--name", name)
	require.NoError(t, err)

	// Load the spec and build the model — same steps as init/preview would do.
	result, loadErr := parser.Load(schema)
	require.NoError(t, loadErr)

	resources, buildErr := model.Build(result)
	require.NoError(t, buildErr)

	spec := result.GetSpec()
	spec.Resources = resources

	// Run Generate() into a temp dir to get the actual file list.
	tmpDir := t.TempDir()
	generateErr := generator.Generate(spec, name, tmpDir)
	require.NoError(t, generateErr, "generator.Generate() should succeed for the minimal fixture")

	// Collect all relative paths that Generate() actually wrote.
	var generatedFiles []string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(tmpDir, path)
		generatedFiles = append(generatedFiles, filepath.ToSlash(rel))
		return nil
	})
	require.NotEmpty(t, generatedFiles, "generator.Generate() should produce at least one file")

	// Every file that Generate() wrote must appear somewhere in the preview output.
	for _, f := range generatedFiles {
		base := filepath.Base(f)
		dir := filepath.ToSlash(filepath.Dir(f))
		inOutput := strings.Contains(out, f) ||
			strings.Contains(out, base) ||
			(dir != "." && strings.Contains(out, dir+"/"))
		assert.True(t, inOutput,
			"preview output should list %q (written by generator.Generate) but did not.\nPreview output:\n%s", f, out)
	}
}
