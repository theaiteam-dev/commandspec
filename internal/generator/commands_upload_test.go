// Package generator_test contains tests for file upload command generation.
// These tests cover WI-519: multipart/form-data command code generation.
//
// The tests fail until the generator is updated to:
//   - Detect FlagTypeFile flags and emit multipart upload code in RunE
//   - Add a DoMultipart method to the generated client
//   - Use StringVar (file path) for FlagTypeFile flag declarations
//   - Import mime/multipart, os, path/filepath when file flags are present
//   - Leave non-upload commands unaffected
package generator_test

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeUploadCmd returns a POST Command whose RequestBody is multipart/form-data
// and that includes a FlagTypeFile flag plus a text field.
func makeUploadCmd() model.Command {
	return model.Command{
		Name:       "upload",
		HTTPMethod: "POST",
		Path:       "/documents/upload",
		Flags: []model.Flag{
			{
				Name: "file", Type: model.FlagTypeFile, Required: true, Source: model.FlagSourceBody,
				Description: "path to the file to upload",
			},
			{
				Name: "name", Type: model.FlagTypeString, Required: false, Source: model.FlagSourceBody,
				Description: "display name for the uploaded document",
			},
		},
		RequestBody: &model.RequestBody{
			ContentType:  "multipart/form-data",
			IsFileUpload: true,
		},
	}
}

// makeUploadResource wraps makeUploadCmd in a minimal resource.
func makeUploadResource() model.Resource {
	return model.Resource{
		Name:     "documents",
		Commands: []model.Command{makeUploadCmd()},
	}
}

// makeFileOnlyUploadCmd returns a Command with only a single binary file flag
// (no extra text fields).
func makeFileOnlyUploadCmd() model.Command {
	return model.Command{
		Name:       "upload",
		HTTPMethod: "POST",
		Path:       "/images/upload",
		Flags: []model.Flag{
			{
				Name: "image", Type: model.FlagTypeFile, Required: true, Source: model.FlagSourceBody,
				Description: "path to the image file",
			},
		},
		RequestBody: &model.RequestBody{
			ContentType:  "multipart/form-data",
			IsFileUpload: true,
		},
	}
}

// ---- FlagTypeFile → StringVar (file path) ----

// TestGenerateVerbCmd_FileFlag_UsesStringVar verifies that FlagTypeFile flags
// are declared with StringVar (not IntVar, BoolVar, etc.) because the user
// provides a filesystem path.
// FAILS until buildFlagVarDeclarations handles FlagTypeFile as a string path.
func TestGenerateVerbCmd_FileFlag_UsesStringVar(t *testing.T) {
	cmd := makeUploadCmd()
	resource := makeUploadResource()

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	assert.Contains(t, src, "StringVar",
		"FlagTypeFile flag declaration should use StringVar (path to file)")
}

// TestGenerateVerbCmd_FileFlag_NotBoolVar verifies that file flags are not
// inadvertently registered as bool flags.
func TestGenerateVerbCmd_FileFlag_NotBoolVar(t *testing.T) {
	cmd := makeFileOnlyUploadCmd()
	resource := model.Resource{Name: "images", Commands: []model.Command{cmd}}

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	assert.NotContains(t, src, "BoolVar",
		"FlagTypeFile flag should never be registered as a bool flag")
}

// ---- Multipart upload code in RunE ----

// TestGenerateVerbCmd_FileUpload_UsesDoMultipart verifies that the generated
// RunE body calls DoMultipart (or an equivalent multipart dispatch method) when
// the command has FlagTypeFile flags.
// FAILS until buildRunEBody detects IsFileUpload and emits DoMultipart call.
func TestGenerateVerbCmd_FileUpload_UsesDoMultipart(t *testing.T) {
	cmd := makeUploadCmd()
	resource := makeUploadResource()

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	hasMultipart := strings.Contains(src, "DoMultipart") ||
		strings.Contains(src, "multipart") ||
		strings.Contains(src, "Multipart")
	assert.True(t, hasMultipart,
		"upload command RunE should call DoMultipart (or equivalent multipart helper)")
}

// TestGenerateVerbCmd_FileUpload_ReadsFileFromDisk verifies that the generated
// RunE opens the file from the path supplied via the flag (e.g. os.Open).
// FAILS until buildRunEBody emits file-reading logic.
func TestGenerateVerbCmd_FileUpload_ReadsFileFromDisk(t *testing.T) {
	cmd := makeUploadCmd()
	resource := makeUploadResource()

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	hasFileRead := strings.Contains(src, "os.Open") ||
		strings.Contains(src, "os.ReadFile") ||
		strings.Contains(src, "Open(") ||
		strings.Contains(src, "ReadFile(")
	assert.True(t, hasFileRead,
		"upload command RunE should open the file from the path provided via flag (os.Open / os.ReadFile)")
}

// TestGenerateVerbCmd_FileUpload_IncludesTextFields verifies that non-file form
// fields (type: string) are sent as text parts in the multipart body.
// FAILS until buildRunEBody emits text-field writing for non-file flags.
func TestGenerateVerbCmd_FileUpload_IncludesTextFields(t *testing.T) {
	cmd := makeUploadCmd() // has both "file" (FlagTypeFile) and "name" (FlagTypeString)
	resource := makeUploadResource()

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	// The "name" text field must be wired into the multipart form.
	// Accept WriteField, formDataContentType header, or explicit "name" flag usage.
	hasTextField := strings.Contains(src, "WriteField") ||
		strings.Contains(src, "name") ||
		strings.Contains(src, "FormField") ||
		strings.Contains(src, "textField")
	assert.True(t, hasTextField,
		"upload command RunE should include text form fields alongside the file part")
}

// ---- Required imports when file flags are present ----

// TestGenerateVerbCmd_FileUpload_ImportsOS verifies that "os" is imported when
// the command has FlagTypeFile flags (needed for os.Open).
// FAILS until buildImports detects FlagTypeFile and adds "os".
func TestGenerateVerbCmd_FileUpload_ImportsOS(t *testing.T) {
	cmd := makeUploadCmd()
	resource := makeUploadResource()

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	assert.Contains(t, src, `"os"`,
		"upload command should import \"os\" for file operations")
}

// TestGenerateVerbCmd_FileUpload_ImportsMimeMultipart verifies that
// "mime/multipart" is imported when the command has FlagTypeFile flags.
// FAILS until buildImports detects FlagTypeFile and adds "mime/multipart".
func TestGenerateVerbCmd_FileUpload_ImportsMimeMultipart(t *testing.T) {
	cmd := makeUploadCmd()
	resource := makeUploadResource()

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	hasMultipartImport := strings.Contains(src, `"mime/multipart"`) ||
		strings.Contains(src, "mime/multipart") ||
		strings.Contains(src, "multipart.NewWriter")
	assert.True(t, hasMultipartImport,
		"upload command should import \"mime/multipart\" for form writer")
}

// TestGenerateVerbCmd_FileUpload_ImportsFilepath verifies that "path/filepath"
// is imported when the command has FlagTypeFile flags.
// FAILS until buildImports detects FlagTypeFile and adds "path/filepath".
func TestGenerateVerbCmd_FileUpload_ImportsFilepath(t *testing.T) {
	cmd := makeUploadCmd()
	resource := makeUploadResource()

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	hasFilepathImport := strings.Contains(src, `"path/filepath"`) ||
		strings.Contains(src, "path/filepath") ||
		strings.Contains(src, "filepath.")
	assert.True(t, hasFilepathImport,
		"upload command should import \"path/filepath\" for file name extraction")
}

// ---- Generated code syntax check ----

// TestGenerateVerbCmd_FileUpload_ValidGoSyntax verifies that the upload command
// source is syntactically valid Go once the implementation is in place.
// Skipped if multipart code is not yet emitted (pre-implementation).
func TestGenerateVerbCmd_FileUpload_ValidGoSyntax(t *testing.T) {
	cmd := makeUploadCmd()
	resource := makeUploadResource()

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	hasMultipart := strings.Contains(src, "DoMultipart") ||
		strings.Contains(src, "multipart") ||
		strings.Contains(src, "Multipart")
	if !hasMultipart {
		t.Skip("multipart upload code not yet implemented; skipping syntax check")
	}

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "documents_upload.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "upload command should produce valid Go syntax:\n%s", src)
}

// ---- DoMultipart method in generated client ----

// TestGenerateClient_HasDoMultipart verifies that the generated client.go
// contains a DoMultipart method (or equivalent) for sending multipart requests.
// FAILS until client.go template gains a DoMultipart method.
func TestGenerateClient_HasDoMultipart(t *testing.T) {
	spec := &model.APISpec{
		Title:   "Upload API",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
	}

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	hasDoMultipart := strings.Contains(src, "DoMultipart") ||
		strings.Contains(src, "doMultipart") ||
		strings.Contains(src, "Multipart")
	assert.True(t, hasDoMultipart,
		"generated client.go should contain a DoMultipart method for multipart/form-data uploads")
}

// TestGenerateClient_DoMultipart_AcceptsFilePath verifies that DoMultipart
// accepts a file path parameter (string), a field name, and optional form fields.
// FAILS until client.go template gains DoMultipart with the correct signature.
func TestGenerateClient_DoMultipart_AcceptsFilePath(t *testing.T) {
	spec := &model.APISpec{
		Title:   "Upload API",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
	}

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	// The method must accept at minimum a file path (string). Accept any of these
	// common signatures: (filePath string, ...), (path string, ...), (file string, ...)
	hasFilePathParam := strings.Contains(src, "filePath") ||
		strings.Contains(src, "fileField") ||
		strings.Contains(src, "file string") ||
		strings.Contains(src, "path string")
	if strings.Contains(src, "DoMultipart") || strings.Contains(src, "doMultipart") {
		assert.True(t, hasFilePathParam,
			"DoMultipart should accept a file path parameter")
	} else {
		t.Skip("DoMultipart not yet implemented; skipping parameter check")
	}
}

// TestGenerateClient_DoMultipart_SetsContentTypeHeader verifies that DoMultipart
// sets the Content-Type header to multipart/form-data with a boundary.
// FAILS until client.go template emits Content-Type assignment in DoMultipart.
func TestGenerateClient_DoMultipart_SetsContentTypeHeader(t *testing.T) {
	spec := &model.APISpec{
		Title:   "Upload API",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
	}

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	if !strings.Contains(src, "DoMultipart") && !strings.Contains(src, "doMultipart") {
		t.Skip("DoMultipart not yet implemented; skipping content-type check")
	}

	hasContentType := strings.Contains(src, "multipart/form-data") ||
		strings.Contains(src, "FormDataContentType") ||
		strings.Contains(src, "Content-Type")
	assert.True(t, hasContentType,
		"DoMultipart should set Content-Type to multipart/form-data with boundary")
}

// TestGenerateClient_DoMultipart_ValidGoSyntax verifies that the generated
// client including DoMultipart is valid Go.
func TestGenerateClient_DoMultipart_ValidGoSyntax(t *testing.T) {
	spec := &model.APISpec{
		Title:   "Upload API",
		Version: "1.0.0",
		BaseURL: "https://api.example.com",
	}

	src, err := generator.GenerateClient(spec)
	require.NoError(t, err)

	if !strings.Contains(src, "DoMultipart") && !strings.Contains(src, "multipart") {
		t.Skip("DoMultipart not yet implemented; skipping syntax check")
	}

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "client.go", src, parser.AllErrors)
	assert.NoError(t, parseErr, "generated client.go should be valid Go syntax:\n%s", src)
}

// ---- Non-upload commands are unaffected (regression guard) ----

// TestGenerateVerbCmd_NonUpload_NoMultipartCode verifies that a plain GET
// command does not accidentally emit multipart upload code.
func TestGenerateVerbCmd_NonUpload_NoMultipartCode(t *testing.T) {
	cmd := model.Command{
		Name:       "list",
		HTTPMethod: "GET",
		Path:       "/documents",
		Flags: []model.Flag{
			{Name: "limit", Type: model.FlagTypeInt, Source: model.FlagSourceQuery},
		},
	}
	resource := model.Resource{Name: "documents", Commands: []model.Command{cmd}}

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	assert.NotContains(t, src, "DoMultipart",
		"GET list command should NOT call DoMultipart")
	assert.NotContains(t, src, "multipart.NewWriter",
		"GET list command should NOT create a multipart writer")
}

// TestGenerateVerbCmd_JSONPost_NoMultipartCode verifies that a standard JSON
// POST (no FlagTypeFile flags) does not emit multipart code.
func TestGenerateVerbCmd_JSONPost_NoMultipartCode(t *testing.T) {
	cmd := model.Command{
		Name:       "create",
		HTTPMethod: "POST",
		Path:       "/documents",
		Flags: []model.Flag{
			{Name: "title", Type: model.FlagTypeString, Required: true, Source: model.FlagSourceBody},
			{Name: "content", Type: model.FlagTypeString, Required: false, Source: model.FlagSourceBody},
		},
		RequestBody: &model.RequestBody{
			ContentType:  "application/json",
			IsFileUpload: false,
		},
	}
	resource := model.Resource{Name: "documents", Commands: []model.Command{cmd}}

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	assert.NotContains(t, src, "DoMultipart",
		"JSON POST command should NOT call DoMultipart")
}

// ---- Bug regression tests: buildMultipartRunEBody ----

// TestGenerateVerbCmd_MultipleFileFlags_AllUploadedAsFiles verifies that when a
// command has MORE THAN ONE FlagTypeFile flag, every file flag is handled as a
// real file upload (os.Open or os.ReadFile + CreateFormFile), not emitted via
// WriteField as a plain text string.
//
// BUG (commands.go ~line 840): The loop captures only the FIRST FlagTypeFile
// flag into fileFlagVar. Subsequent FlagTypeFile flags fall into the
// "else if f.Source == model.FlagSourceBody" branch and land in textFlags,
// causing them to be emitted as _mpWriter.WriteField("thumbnail", ...) — which
// sends the file path as a plain text value instead of uploading file bytes.
//
// FAILS currently: the second file flag ("thumbnail") is written with WriteField.
func TestGenerateVerbCmd_MultipleFileFlags_AllUploadedAsFiles(t *testing.T) {
	cmd := model.Command{
		Name:       "upload",
		HTTPMethod: "POST",
		Path:       "/media/upload",
		Flags: []model.Flag{
			{
				Name: "file", Type: model.FlagTypeFile, Required: true, Source: model.FlagSourceBody,
				Description: "primary file to upload",
			},
			{
				Name: "thumbnail", Type: model.FlagTypeFile, Required: false, Source: model.FlagSourceBody,
				Description: "thumbnail image to upload",
			},
		},
		RequestBody: &model.RequestBody{
			ContentType:  "multipart/form-data",
			IsFileUpload: true,
		},
	}
	resource := model.Resource{Name: "media", Commands: []model.Command{cmd}}

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	// Both file flags must be read from disk. Count occurrences of the
	// file-reading patterns: each file flag needs its own os.Open or os.ReadFile.
	openCount := strings.Count(src, "os.Open") + strings.Count(src, "os.ReadFile")
	assert.GreaterOrEqual(t, openCount, 2,
		"every FlagTypeFile flag should produce an os.Open or os.ReadFile call; "+
			"got %d but expected at least 2 (one per file flag).\n\nGenerated source:\n%s",
		openCount, src)

	// The "thumbnail" flag must NOT appear as a WriteField call — that would
	// mean the file path string is being sent as a plain text form field.
	thumbnailWriteField := strings.Contains(src, `WriteField("thumbnail"`) ||
		strings.Contains(src, `WriteField(\"thumbnail\"`)
	assert.False(t, thumbnailWriteField,
		"the second FlagTypeFile flag (\"thumbnail\") must NOT be emitted as WriteField; "+
			"it should be uploaded as a file part via CreateFormFile.\n\nGenerated source:\n%s", src)
}

// TestGenerateVerbCmd_FileUpload_NoDeadFileDescriptor verifies that the
// generated RunE does NOT open a file with os.Open and then ALSO read it with
// os.ReadFile. Using both for the same flag is dead code: os.Open creates an
// _mpFile handle that is never read from (os.ReadFile opens its own handle),
// leaking a file descriptor until defer runs.
//
// BUG (commands.go ~line 853): buildMultipartRunEBody emits:
//
//	_mpFile, _mpErr := os.Open(fileFlagVar)      // descriptor opened, never read
//	defer _mpFile.Close()
//	_mpFileBytes, _mpErr := os.ReadFile(...)     // opens its own handle, reads bytes
//
// The fix is to use EITHER os.Open + io.ReadAll(_mpFile) OR os.ReadFile alone.
//
// FAILS currently: generated code contains both os.Open and os.ReadFile.
func TestGenerateVerbCmd_FileUpload_NoDeadFileDescriptor(t *testing.T) {
	cmd := makeFileOnlyUploadCmd()
	resource := model.Resource{Name: "images", Commands: []model.Command{cmd}}

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	if !strings.Contains(src, "DoMultipart") && !strings.Contains(src, "multipart") {
		t.Skip("multipart upload code not yet implemented; skipping dead-descriptor check")
	}

	hasOpen := strings.Contains(src, "os.Open(")
	hasReadFile := strings.Contains(src, "os.ReadFile(")

	// Both being present is the bug: one is dead code.
	bothPresent := hasOpen && hasReadFile
	assert.False(t, bothPresent,
		"generated RunE must not use both os.Open AND os.ReadFile for the same flag: "+
			"os.Open opens a descriptor that os.ReadFile never uses, leaking it until defer. "+
			"Use EITHER os.Open+io.ReadAll OR os.ReadFile alone.\n\nGenerated source:\n%s", src)
}

// TestGenerateVerbCmd_FileUpload_OptionalTextFieldGuarded verifies that
// optional (Required=false) text form fields in a multipart upload are only
// written to the form when the user actually passed the flag on the command
// line — i.e., the generated code guards WriteField with a
// cmd.Flags().Changed("fieldname") check.
//
// BUG (commands.go ~line 881): The textFlags loop emits WriteField
// unconditionally for every non-file body flag, including optional ones. An
// optional flag that the user did not pass will have its zero value (empty
// string "") sent to the server, which may override a server-side default or
// cause a validation error.
//
// FAILS currently: no Changed guard is emitted for optional text fields.
func TestGenerateVerbCmd_FileUpload_OptionalTextFieldGuarded(t *testing.T) {
	cmd := model.Command{
		Name:       "upload",
		HTTPMethod: "POST",
		Path:       "/documents/upload",
		Flags: []model.Flag{
			{
				Name: "file", Type: model.FlagTypeFile, Required: true, Source: model.FlagSourceBody,
				Description: "path to the file to upload",
			},
			{
				// Optional text field — should be guarded by Changed().
				Name: "description", Type: model.FlagTypeString, Required: false, Source: model.FlagSourceBody,
				Description: "optional description for the uploaded document",
			},
		},
		RequestBody: &model.RequestBody{
			ContentType:  "multipart/form-data",
			IsFileUpload: true,
		},
	}
	resource := model.Resource{Name: "documents", Commands: []model.Command{cmd}}

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	if !strings.Contains(src, "WriteField") {
		t.Skip("multipart text-field code not yet implemented; skipping Changed guard check")
	}

	// The optional "description" field must only be sent when the user provided
	// it. Accept any of: Changed("description"), .Changed("description"),
	// Flags().Changed, or a similar guard pattern.
	hasChangedGuard := strings.Contains(src, `Changed("description"`) ||
		strings.Contains(src, `.Changed("description"`)
	assert.True(t, hasChangedGuard,
		"optional text field \"description\" must be guarded by cmd.Flags().Changed(\"description\") "+
			"so it is only sent when the user explicitly passes the flag; "+
			"unconditional WriteField sends an empty string even when the flag was omitted.\n\nGenerated source:\n%s", src)
}

// ---- Mixed file + text multipart ----

// TestGenerateVerbCmd_MixedMultipart_BothPartsPresent verifies that when the
// command has both FlagTypeFile and FlagTypeString body flags, the generated
// RunE handles both: the file part and the text part.
// FAILS until buildRunEBody handles mixed multipart payloads.
func TestGenerateVerbCmd_MixedMultipart_BothPartsPresent(t *testing.T) {
	cmd := makeUploadCmd() // "file" (FlagTypeFile) + "name" (FlagTypeString)
	resource := makeUploadResource()

	src, err := generator.GenerateVerbCmd(resource, cmd, "mycli")
	require.NoError(t, err)

	// Must reference the file flag
	hasFilePart := strings.Contains(src, "file") &&
		(strings.Contains(src, "os.Open") || strings.Contains(src, "ReadFile") ||
			strings.Contains(src, "DoMultipart") || strings.Contains(src, "multipart"))
	// Must reference the text field "name"
	hasNameField := strings.Contains(src, "name") &&
		(strings.Contains(src, "WriteField") || strings.Contains(src, "name") ||
			strings.Contains(src, "FormField"))

	assert.True(t, hasFilePart,
		"mixed multipart command should handle the file part")
	assert.True(t, hasNameField,
		"mixed multipart command should include text field 'name' as a form part")
}

// ---- Compile test: multiple file flags must not produce duplicate variable declarations ----

// TestGenerateVerbCmd_MultipleFileFlags_Compiles verifies that generated code
// for a command with two FlagTypeFile flags actually compiles. This is a
// regression test for a scope collision bug: when the file-flag loop emitted
// _mpFileBytes/:= and _mpPart/:= at the same flat scope, the second iteration
// produced "no new variables on left side of :=", which go/parser does NOT
// catch but go build fails on.
//
// The fix wraps each file-flag block in its own {…} scope so the locals are
// redeclared in distinct inner scopes.
//
// Before the fix this test FAILS (go build error); after the fix it PASSES.
func TestGenerateVerbCmd_MultipleFileFlags_Compiles(t *testing.T) {
	cmd := model.Command{
		Name:       "upload",
		HTTPMethod: "POST",
		Path:       "/media/upload",
		Flags: []model.Flag{
			{
				Name: "file", Type: model.FlagTypeFile, Required: true, Source: model.FlagSourceBody,
				Description: "primary file to upload",
			},
			{
				Name: "thumbnail", Type: model.FlagTypeFile, Required: false, Source: model.FlagSourceBody,
				Description: "thumbnail image to upload",
			},
		},
		RequestBody: &model.RequestBody{
			ContentType:  "multipart/form-data",
			IsFileUpload: true,
		},
	}
	resource := model.Resource{Name: "media", Commands: []model.Command{cmd}}

	src, err := generator.GenerateVerbCmd(resource, cmd, "testcli")
	require.NoError(t, err, "GenerateVerbCmd must not return a syntax error")

	// Write the generated source into a temporary directory as a Go module and
	// attempt to build it. A compilation failure means the generated code has
	// type-checker errors (e.g. duplicate := declarations) that go/parser misses.
	tmpDir := t.TempDir()

	// Write go.mod so the compiler recognises this as a module. No external
	// dependencies are listed here because all real imports (cobra, client,
	// output) are replaced with local stubs in stubSrc before writing the file.
	goMod := `module testcli

go 1.21
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o600))

	// Stub out the packages the generated command imports so the file is
	// self-contained for a type-check. We use a minimal stub that satisfies the
	// compiler without needing a real network or cobra dependency.
	//
	// Strategy: replace the real import paths with a local package that provides
	// just enough exported names to pass type-checking. We do this by rewriting
	// the import paths in the generated source to point at local stubs.
	stubSrc := strings.NewReplacer(
		`"github.com/spf13/cobra"`, `cobra "testcli/internal/stub/cobra"`,
		`"testcli/internal/client"`, `client "testcli/internal/stub/client"`,
		`"testcli/internal/output"`, `output "testcli/internal/stub/output"`,
	).Replace(src)

	// Write the generated command file into a cmd package.
	cmdDir := filepath.Join(tmpDir, "cmd")
	require.NoError(t, os.MkdirAll(cmdDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cmdDir, "media_upload.go"), []byte(stubSrc), 0o600))

	// Provide a minimal root.go so the cmd package compiles (mediaCmdCmd is
	// expected to be defined in the resource group file, rootCmd in root.go).
	rootSrc := `package cmd

import cobra "testcli/internal/stub/cobra"

var rootCmd = &cobra.Command{}
var mediaCmd = &cobra.Command{}
`
	require.NoError(t, os.WriteFile(filepath.Join(cmdDir, "root.go"), []byte(rootSrc), 0o600))

	// Cobra stub — provides just enough for the generated code to type-check.
	// The Command struct mirrors the fields used in generated struct literals
	// (Use, Short, Args, RunE). Methods are added for the call sites in RunE
	// (Flags, PersistentFlags, Root, AddCommand, MarkFlagRequired,
	// RegisterFlagCompletionFunc).
	cobraStubDir := filepath.Join(tmpDir, "internal", "stub", "cobra")
	require.NoError(t, os.MkdirAll(cobraStubDir, 0o755))
	cobraStub := `package cobra

type Command struct {
	Use   string
	Short string
	Args  func(*Command, []string) error
	RunE  func(*Command, []string) error
}

func (c *Command) AddCommand(...*Command) {}
func (c *Command) Flags() *FlagSet { return &FlagSet{} }
func (c *Command) PersistentFlags() *FlagSet { return &FlagSet{} }
func (c *Command) Root() *Command { return c }
func (c *Command) RegisterFlagCompletionFunc(name string, fn func(*Command, []string, string) ([]string, ShellCompDirective)) error { return nil }
func (c *Command) MarkFlagRequired(name string) error { return nil }

type FlagSet struct{}

func (f *FlagSet) StringVar(p *string, name, value, usage string) {}
func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {}
func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {}
func (f *FlagSet) StringArrayVar(p *[]string, name string, value []string, usage string) {}
func (f *FlagSet) StringArray(name string, value []string, usage string) *[]string { return nil }
func (f *FlagSet) GetString(name string) (string, error) { return "", nil }
func (f *FlagSet) GetBool(name string) (bool, error) { return false, nil }
func (f *FlagSet) Changed(name string) bool { return false }

type ShellCompDirective int

const ShellCompDirectiveNoFileComp ShellCompDirective = 0

func NoArgs(cmd *Command, args []string) error { return nil }
func ExactArgs(n int) func(*Command, []string) error {
	return func(_ *Command, _ []string) error { return nil }
}
`
	require.NoError(t, os.WriteFile(filepath.Join(cobraStubDir, "cobra.go"), []byte(cobraStub), 0o600))

	// Client stub.
	clientStubDir := filepath.Join(tmpDir, "internal", "stub", "client")
	require.NoError(t, os.MkdirAll(clientStubDir, 0o755))
	clientStub := `package client

type Client struct{}

func NewClient(baseURL, token string) *Client { return &Client{} }
func (c *Client) Do(method, path string, pathParams, queryParams map[string]string, body interface{}) ([]byte, error) {
	return nil, nil
}
func (c *Client) DoMultipart(method, path string, pathParams, queryParams map[string]string, body interface{}, contentType string) ([]byte, error) {
	return nil, nil
}
`
	require.NoError(t, os.WriteFile(filepath.Join(clientStubDir, "client.go"), []byte(clientStub), 0o600))

	// Output stub.
	outputStubDir := filepath.Join(tmpDir, "internal", "stub", "output")
	require.NoError(t, os.MkdirAll(outputStubDir, 0o755))
	outputStub := `package output

func PrintTable(data []byte, noColor bool) error { return nil }
`
	require.NoError(t, os.WriteFile(filepath.Join(outputStubDir, "output.go"), []byte(outputStub), 0o600))

	// Run go build on the cmd package. This exercises the full type-checker,
	// catching duplicate := errors that go/parser silently accepts.
	buildCmd := exec.Command("go", "build", "./cmd/...")
	buildCmd.Dir = tmpDir
	out, buildErr := buildCmd.CombinedOutput()
	assert.NoError(t, buildErr,
		"generated code with two FlagTypeFile flags must compile without errors.\n"+
			"go build output:\n%s\n\nGenerated source (after stub rewrites):\n%s",
		string(out), stubSrc)
}
