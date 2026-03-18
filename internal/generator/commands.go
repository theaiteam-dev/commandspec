package generator

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/theaiteam-dev/commandspec/internal/model"
)

// codeWriter is a small helper for building indented Go source code.
// indent controls the number of leading tabs prepended by line().
type codeWriter struct {
	sb     strings.Builder
	indent int
}

// line writes a literal line to the builder, prefixed with w.indent tabs.
// The string s is written as-is with no format expansion.
func (w *codeWriter) line(s string) {
	w.sb.WriteString(strings.Repeat("\t", w.indent))
	w.sb.WriteString(s)
	w.sb.WriteString("\n")
}

// linef writes a formatted line to the builder, prefixed with w.indent tabs.
// The format string is expanded before writing.
func (w *codeWriter) linef(format string, args ...interface{}) {
	w.sb.WriteString(strings.Repeat("\t", w.indent))
	fmt.Fprintf(&w.sb, format, args...)
	w.sb.WriteString("\n")
}

// String returns the accumulated source text.
func (w *codeWriter) String() string {
	return w.sb.String()
}

// GenerateResourceCmd produces the source for cmd/<resource>.go, which
// declares a Cobra group command for the given resource. All operations on
// the resource are added as sub-commands of this group.
func GenerateResourceCmd(resource model.Resource) (string, error) {
	short := resource.Description
	if short == "" {
		short = resource.Name
	}

	varName := sanitizeIdentifier(resource.Name) + "Cmd"

	src := fmt.Sprintf(`package cmd

import "github.com/spf13/cobra"

var %s = &cobra.Command{
	Use: %q,
	Short: %q,
}

func init() {
	rootCmd.AddCommand(%s)
}
`, varName, resource.Name, short, varName)

	return validateGoSource(src)
}

// GenerateVerbCmd produces the source for cmd/<resource>_<verb>.go, which
// declares the individual Cobra command for a specific HTTP operation.
// Path arguments become positional args; query/body/header parameters become
// flags. cliName is used to derive the token env var and the import path for
// the internal client package.
func GenerateVerbCmd(resource model.Resource, cmd model.Command, cliName string) (string, error) {
	if cliName == "" {
		return "", fmt.Errorf("cliName must not be empty: needed for client import path and auth env var generation")
	}

	useField := buildUseField(cmd)
	argsExpr := buildArgsExpr(cmd.Args)
	varName := sanitizeIdentifier(resource.Name) + capitalise(sanitizeIdentifier(cmd.Name)) + "Cmd"
	resourceVarName := sanitizeIdentifier(resource.Name) + "Cmd"

	writeOp := isWriteOperation(cmd.HTTPMethod)
	// File upload commands handle their own body construction via multipart;
	// suppress the generic --body/--body-file flags to avoid unused var errors.
	writeOpForFlags := writeOp && !hasFileUpload(cmd)
	flagVars := buildFlagVarDeclarations(varName, cmd.Flags, writeOpForFlags, cmd.Pagination)
	flagInits := buildFlagInits(varName, cmd.Flags, writeOpForFlags, cmd.Pagination)
	requiredInits := buildRequiredFlagInits(varName, cmd.Flags)

	runEBody := buildRunEBody(cmd, varName, cliName, writeOp)

	// Determine which imports are needed.
	imports := buildImports(cmd, cliName)

	src := fmt.Sprintf(`package cmd

%s

%s

var %s = &cobra.Command{
	Use: %q,
	Short: %q,
	Args: %s,
	RunE: func(cmd *cobra.Command, args []string) error {
%s	},
}

func init() {
	%s.AddCommand(%s)
%s%s}
`,
		imports,
		flagVars,
		varName,
		useField,
		cmd.Description,
		argsExpr,
		runEBody,
		resourceVarName,
		varName,
		flagInits,
		requiredInits,
	)

	// Clean up double blank lines that result from empty flagVars.
	for strings.Contains(src, "\n\n\n") {
		src = strings.ReplaceAll(src, "\n\n\n", "\n\n")
	}

	return validateGoSource(src)
}

// GenerateVerbCmdWithAuth is an auth-aware variant of GenerateVerbCmd. It
// accepts a SecuritySchemes map so the generated RunE body reads the correct
// env vars for each scheme type rather than always falling back to _TOKEN.
//
// When schemes is nil or empty, the function falls back to Bearer token
// behaviour (same as GenerateVerbCmd).
func GenerateVerbCmdWithAuth(resource model.Resource, cmd model.Command, cliName string, schemes map[string]model.SecurityScheme) (string, error) {
	if cliName == "" {
		return "", fmt.Errorf("cliName must not be empty: needed for client import path and auth env var generation")
	}

	useField := buildUseField(cmd)
	argsExpr := buildArgsExpr(cmd.Args)
	varName := sanitizeIdentifier(resource.Name) + capitalise(sanitizeIdentifier(cmd.Name)) + "Cmd"
	resourceVarName := sanitizeIdentifier(resource.Name) + "Cmd"

	writeOp := isWriteOperation(cmd.HTTPMethod)
	writeOpForFlags := writeOp && !hasFileUpload(cmd)
	flagVars := buildFlagVarDeclarations(varName, cmd.Flags, writeOpForFlags, cmd.Pagination)
	flagInits := buildFlagInits(varName, cmd.Flags, writeOpForFlags, cmd.Pagination)
	requiredInits := buildRequiredFlagInits(varName, cmd.Flags)

	runEBody := buildRunEBodyWithAuth(cmd, varName, cliName, writeOp, schemes)
	imports := buildImports(cmd, cliName)

	src := fmt.Sprintf(`package cmd

%s

%s

var %s = &cobra.Command{
	Use: %q,
	Short: %q,
	Args: %s,
	RunE: func(cmd *cobra.Command, args []string) error {
%s	},
}

func init() {
	%s.AddCommand(%s)
%s%s}
`,
		imports,
		flagVars,
		varName,
		useField,
		cmd.Description,
		argsExpr,
		runEBody,
		resourceVarName,
		varName,
		flagInits,
		requiredInits,
	)

	for strings.Contains(src, "\n\n\n") {
		src = strings.ReplaceAll(src, "\n\n\n", "\n\n")
	}

	return validateGoSource(src)
}

// buildRunEBodyWithAuth is like buildRunEBody but emits auth env var lookups
// based on the provided SecuritySchemes map. When schemes is nil or empty, it
// falls back to the default Bearer token behaviour.
func buildRunEBodyWithAuth(cmd model.Command, varName, cliName string, writeOp bool, schemes map[string]model.SecurityScheme) string {
	w := &codeWriter{indent: 2}

	w.line(`baseURL, _ := cmd.Root().PersistentFlags().GetString("base-url")`)

	// Classify schemes.
	hasBearerOrBasic := false
	apiKeyEnvVars := []string{}

	if len(schemes) == 0 {
		// No schemes — default to bearer token.
		hasBearerOrBasic = true
	} else {
		seenAPIKeyEnvVars := map[string]bool{}
		for _, s := range schemes {
			switch s.Type {
			case model.SecuritySchemeBearer, model.SecuritySchemeBasic:
				hasBearerOrBasic = true
			case model.SecuritySchemeAPIKey:
				if !seenAPIKeyEnvVars[s.EnvVar] && s.EnvVar != "" {
					seenAPIKeyEnvVars[s.EnvVar] = true
					apiKeyEnvVars = append(apiKeyEnvVars, s.EnvVar)
				}
			}
		}
	}

	// Emit token lookup for Bearer/Basic schemes.
	tokenEnvVar := cliNameToEnvPrefix(cliName) + "_TOKEN"
	if hasBearerOrBasic {
		w.linef("token := os.Getenv(%q)", tokenEnvVar)
	} else {
		// Emit a blank token so client.NewClient still compiles.
		w.line(`token := ""`)
	}

	// Emit API key env var lookups.
	// Assign to the blank identifier so there is no named local variable that
	// could collide with the "token" variable emitted for Bearer/Basic schemes
	// (e.g. when EnvVar=="TOKEN", envVarToIdent returns "token"). The client
	// package already handles API key injection at package scope, so these
	// reads serve only to surface the env var name in the generated source.
	for _, envVar := range apiKeyEnvVars {
		w.linef("_ = os.Getenv(%q)", envVar)
	}

	w.line("c := client.NewClient(baseURL, token)")

	// Delegate the rest of the body to the existing helper, which already
	// handles path/query params, body building, and output.
	// We can't call buildRunEBody directly (it would re-emit the base-url/token
	// lines), so we use a thin delegation: write the lines we need differently
	// above, then copy the remaining logic from buildRunEBody inline.
	// For simplicity, delegate to a shared tail helper.
	tail := buildRunEBodyTail(cmd, varName, cliName, writeOp)
	return w.String() + tail
}

// buildRunEBodyTail returns the portion of the RunE body that comes after the
// client construction line: path params, query params, body, and c.Do call.
// This is separated so that buildRunEBodyWithAuth can emit its own preamble
// and then append the common tail.
func buildRunEBodyTail(cmd model.Command, varName, _ string, writeOp bool) string {
	w := &codeWriter{indent: 2}

	w.line("pathParams := map[string]string{}")
	for i, arg := range cmd.Args {
		w.linef("pathParams[%q] = args[%d]", argToPathKey(arg.Name), i)
	}

	hasQueryFlags := false
	for _, f := range cmd.Flags {
		if f.Source == model.FlagSourceQuery {
			hasQueryFlags = true
			break
		}
	}

	hasBodyFlags := false
	for _, f := range cmd.Flags {
		if f.Source == model.FlagSourceBody {
			hasBodyFlags = true
			break
		}
	}

	if hasQueryFlags {
		w.line("queryParams := map[string]string{}")
		for _, f := range cmd.Flags {
			if f.Source != model.FlagSourceQuery {
				continue
			}
			fVar := flagVarName(varName, f.Name)
			switch f.Type {
			case model.FlagTypeInt:
				w.linef("queryParams[%q] = strconv.Itoa(%s)", f.Name, fVar)
			case model.FlagTypeBool:
				w.linef("queryParams[%q] = strconv.FormatBool(%s)", f.Name, fVar)
			case model.FlagTypeStringSlice:
				w.linef("%s_vals, _ := cmd.Flags().GetStringArray(%q)", fVar, f.Name)
				w.linef("queryParams[%q] = strings.Join(%s_vals, \",\")", f.Name, fVar)
			default:
				w.linef("queryParams[%q] = %s", f.Name, fVar)
			}
		}
	} else {
		w.line("queryParams := map[string]string{}")
	}

	for _, f := range cmd.Flags {
		if len(f.Enum) == 0 {
			continue
		}
		fVar := flagVarName(varName, f.Name)
		enumLiterals := make([]string, len(f.Enum))
		for i, v := range f.Enum {
			enumLiterals[i] = fmt.Sprintf("%q", v)
		}
		allowedExpr := "[]string{" + strings.Join(enumLiterals, ", ") + "}"
		if f.Required {
			w.linef(`if err := validate.Enum(%q, %s, %s); err != nil { return err }`, f.Name, fVar, allowedExpr)
		} else {
			w.linef(`if cmd.Flags().Changed(%q) { if err := validate.Enum(%q, %s, %s); err != nil { return err } }`, f.Name, f.Name, fVar, allowedExpr)
		}
	}

	bodyVar := varName + "Body"
	bodyFileVar := varName + "BodyFile"
	if writeOp {
		w.linef(`if %s != "" {`, bodyFileVar)
		w.indent++
		w.linef("fileData, err := os.ReadFile(%s)", bodyFileVar)
		w.line("if err != nil {")
		w.indent++
		w.linef(`return fmt.Errorf("reading body-file: %%w", err)`)
		w.indent--
		w.line("}")
		w.line("if !json.Valid(fileData) {")
		w.indent++
		w.line(`return fmt.Errorf("body-file does not contain valid JSON")`)
		w.indent--
		w.line("}")
		w.linef("%s = string(fileData)", bodyVar)
		w.indent--
		w.line("}")
		w.linef(`if %s != "" {`, bodyVar)
		w.indent++
		w.linef("if !json.Valid([]byte(%s)) {", bodyVar)
		w.indent++
		w.line(`return fmt.Errorf("--body does not contain valid JSON")`)
		w.indent--
		w.line("}")
		w.line("var bodyObj interface{}")
		w.linef("_ = json.Unmarshal([]byte(%s), &bodyObj)", bodyVar)
		w.linef(`resp, err := c.Do(%q, %q, pathParams, queryParams, bodyObj)`, cmd.HTTPMethod, cmd.Path)
		w.line("if err != nil {")
		w.indent++
		w.line("return err")
		w.indent--
		w.line("}")
		w.line(`jsonMode, _ := cmd.Root().PersistentFlags().GetBool("json")`)
		w.line(`noColor, _ := cmd.Root().PersistentFlags().GetBool("no-color")`)
		w.line("if jsonMode {")
		w.indent++
		w.line(`fmt.Printf("%s\n", string(resp))`)
		w.indent--
		w.line("} else {")
		w.indent++
		w.line("if err := output.PrintTable(resp, noColor); err != nil {")
		w.indent++
		w.line(`fmt.Println(string(resp))`)
		w.indent--
		w.line("}")
		w.indent--
		w.line("}")
		w.line("return nil")
		w.indent--
		w.line("}")
	}

	if hasBodyFlags {
		w.line("bodyMap := map[string]interface{}{}")
		for _, f := range cmd.Flags {
			if f.Source != model.FlagSourceBody {
				continue
			}
			fVar := flagVarName(varName, f.Name)
			parts := strings.Split(f.Name, ".")
			if len(parts) == 1 {
				w.linef("bodyMap[%q] = %s", f.Name, fVar)
			} else if len(parts) <= 3 {
				w.linef("{")
				w.indent++
				w.linef("_parts := strings.Split(%q, \".\")", f.Name)
				w.line("_cur := bodyMap")
				w.line("for _, _p := range _parts[:len(_parts)-1] {")
				w.indent++
				w.line("if _, ok := _cur[_p]; !ok {")
				w.indent++
				w.line("_cur[_p] = map[string]interface{}{}")
				w.indent--
				w.line("}")
				w.line("_cur = _cur[_p].(map[string]interface{})")
				w.indent--
				w.line("}")
				w.linef("_cur[_parts[len(_parts)-1]] = %s", fVar)
				w.indent--
				w.line("}")
			}
		}
	}

	bodyArg := "nil"
	if hasBodyFlags {
		bodyArg = "bodyMap"
	}

	allVar := varName + "All"

	if cmd.Pagination != nil {
		pType := string(cmd.Pagination.Type)
		pageParam := cmd.Pagination.PageParam
		sizeParam := cmd.Pagination.SizeParam
		cursorParam := cmd.Pagination.CursorParam

		w.linef("if %s {", allVar)
		w.indent++
		w.linef("_cfg := client.PaginationConfig{")
		w.indent++
		w.linef("Type: client.PaginationType(%q),", pType)
		w.linef("PageParam: %q,", pageParam)
		w.linef("SizeParam: %q,", sizeParam)
		w.linef("CursorParam: %q,", cursorParam)
		w.indent--
		w.line("}")
		w.linef("_out, _err := client.FetchAll(c, %q, %q, pathParams, queryParams, _cfg)", cmd.HTTPMethod, cmd.Path)
		w.line("if _err != nil { return _err }")
		w.line(`jsonMode, _ := cmd.Root().PersistentFlags().GetBool("json")`)
		w.line(`noColor, _ := cmd.Root().PersistentFlags().GetBool("no-color")`)
		w.line("if jsonMode {")
		w.indent++
		w.line(`fmt.Printf("%s\n", string(_out))`)
		w.indent--
		w.line("} else {")
		w.indent++
		w.line("if err := output.PrintTable(_out, noColor); err != nil {")
		w.indent++
		w.line(`fmt.Println(string(_out))`)
		w.indent--
		w.line("}")
		w.indent--
		w.line("}")
		w.line("return nil")
		w.indent--
		w.line("}")
	}

	w.linef("resp, err := c.Do(%q, %q, pathParams, queryParams, %s)",
		cmd.HTTPMethod, cmd.Path, bodyArg)
	w.line("if err != nil {")
	w.indent++
	w.line("return err")
	w.indent--
	w.line("}")

	w.line(`jsonMode, _ := cmd.Root().PersistentFlags().GetBool("json")`)
	w.line(`noColor, _ := cmd.Root().PersistentFlags().GetBool("no-color")`)
	w.line("if jsonMode {")
	w.indent++
	w.line(`fmt.Printf("%s\n", string(resp))`)
	w.indent--
	w.line("} else {")
	w.indent++
	w.line("if err := output.PrintTable(resp, noColor); err != nil {")
	w.indent++
	w.line(`fmt.Println(string(resp))`)
	w.indent--
	w.line("}")
	w.indent--
	w.line("}")
	w.line("return nil")

	return w.String()
}

// isWriteOperation returns true for HTTP methods that carry a request body.
func isWriteOperation(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PUT", "PATCH":
		return true
	}
	return false
}

// buildImports constructs the import block for the verb command file.
func buildImports(cmd model.Command, cliName string) string {
	writeOp := isWriteOperation(cmd.HTTPMethod)
	hasIntOrBoolQueryFlag := false
	hasStringSliceQueryFlag := false
	hasDotNotationBodyFlag := false
	hasEnumFlag := false
	hasFileFlag := false
	for _, f := range cmd.Flags {
		if f.Source == model.FlagSourceQuery {
			switch f.Type {
			case model.FlagTypeInt, model.FlagTypeBool:
				hasIntOrBoolQueryFlag = true
			case model.FlagTypeStringSlice:
				hasStringSliceQueryFlag = true
			}
		}
		if f.Source == model.FlagSourceBody && strings.Contains(f.Name, ".") {
			parts := strings.Split(f.Name, ".")
			if len(parts) <= 3 {
				hasDotNotationBodyFlag = true
			}
		}
		if len(f.Enum) > 0 {
			hasEnumFlag = true
		}
		if f.Type == model.FlagTypeFile {
			hasFileFlag = true
		}
	}

	var imports []string
	if cliName != "" && writeOp && !hasFileFlag {
		imports = append(imports, `"encoding/json"`)
	}
	if hasFileFlag {
		imports = append(imports, `"bytes"`)
		imports = append(imports, `"io"`)
		imports = append(imports, `"mime/multipart"`)
	}
	imports = append(imports, `"fmt"`, `"os"`)
	if hasFileFlag {
		imports = append(imports, `"path/filepath"`)
	}
	if hasStringSliceQueryFlag || hasDotNotationBodyFlag {
		imports = append(imports, `"strings"`)
	}
	if hasIntOrBoolQueryFlag {
		imports = append(imports, `"strconv"`)
	}
	imports = append(imports, `"github.com/spf13/cobra"`)
	if cliName != "" {
		imports = append(imports, fmt.Sprintf(`%q`, cliName+"/internal/client"))
		imports = append(imports, fmt.Sprintf(`%q`, cliName+"/internal/output"))
		if hasEnumFlag {
			imports = append(imports, fmt.Sprintf(`%q`, cliName+"/internal/validate"))
		}
	}
	return "import (\n\t" + strings.Join(imports, "\n\t") + "\n)"
}

// hasFileUpload returns true when the command has at least one FlagTypeFile flag,
// indicating a multipart/form-data upload request.
func hasFileUpload(cmd model.Command) bool {
	for _, f := range cmd.Flags {
		if f.Type == model.FlagTypeFile {
			return true
		}
	}
	return false
}

// buildRunEBody generates the RunE function body for the given command.
func buildRunEBody(cmd model.Command, varName, cliName string, writeOp bool) string {
	w := &codeWriter{indent: 2}

	// Read base URL from root persistent flags.
	w.line(`baseURL, _ := cmd.Root().PersistentFlags().GetString("base-url")`)

	// Read token from env var.
	tokenEnvVar := cliNameToEnvPrefix(cliName) + "_TOKEN"
	w.linef("token := os.Getenv(%q)", tokenEnvVar)

	// Create client.
	w.line("c := client.NewClient(baseURL, token)")

	// Build pathParams map.
	w.line("pathParams := map[string]string{}")
	for i, arg := range cmd.Args {
		// The path param key is the original camelCase name from the path template.
		// We derive it from the arg name by reversing kebab->camelCase is complex;
		// instead we use the arg name directly as the key since that's what client.Do needs.
		w.linef("pathParams[%q] = args[%d]", argToPathKey(arg.Name), i)
	}

	// Build queryParams map — only if there are query flags.
	hasQueryFlags := false
	for _, f := range cmd.Flags {
		if f.Source == model.FlagSourceQuery {
			hasQueryFlags = true
			break
		}
	}

	hasBodyFlags := false
	for _, f := range cmd.Flags {
		if f.Source == model.FlagSourceBody {
			hasBodyFlags = true
			break
		}
	}

	if hasQueryFlags {
		w.line("queryParams := map[string]string{}")
		for _, f := range cmd.Flags {
			if f.Source != model.FlagSourceQuery {
				continue
			}
			fVar := flagVarName(varName, f.Name)
			switch f.Type {
			case model.FlagTypeInt:
				w.linef("queryParams[%q] = strconv.Itoa(%s)", f.Name, fVar)
			case model.FlagTypeBool:
				w.linef("queryParams[%q] = strconv.FormatBool(%s)", f.Name, fVar)
			case model.FlagTypeStringSlice:
				// Read inline to avoid []string assignment into map[string]string.
				w.linef("%s_vals, _ := cmd.Flags().GetStringArray(%q)", fVar, f.Name)
				w.linef("queryParams[%q] = strings.Join(%s_vals, \",\")", f.Name, fVar)
			default:
				w.linef("queryParams[%q] = %s", f.Name, fVar)
			}
		}
	} else {
		w.line("queryParams := map[string]string{}")
	}

	// Runtime validation for enum flags using validate.Enum helper.
	for _, f := range cmd.Flags {
		if len(f.Enum) == 0 {
			continue
		}
		fVar := flagVarName(varName, f.Name)
		enumLiterals := make([]string, len(f.Enum))
		for i, v := range f.Enum {
			enumLiterals[i] = fmt.Sprintf("%q", v)
		}
		allowedExpr := "[]string{" + strings.Join(enumLiterals, ", ") + "}"
		if f.Required {
			// Always validate required enum flags.
			w.linef(`if err := validate.Enum(%q, %s, %s); err != nil { return err }`, f.Name, fVar, allowedExpr)
		} else {
			// Only validate optional flags when explicitly set.
			w.linef(`if cmd.Flags().Changed(%q) { if err := validate.Enum(%q, %s, %s); err != nil { return err } }`, f.Name, f.Name, fVar, allowedExpr)
		}
	}

	// Multipart file upload path — emit DoMultipart call and return early.
	if hasFileUpload(cmd) {
		return buildMultipartRunEBody(w, cmd, varName)
	}

	// Build body — support --body/--body-file overrides for write operations.
	bodyVar := varName + "Body"
	bodyFileVar := varName + "BodyFile"
	if writeOp {
		// --body-file: read JSON from file, set body string
		w.linef(`if %s != "" {`, bodyFileVar)
		w.indent++
		w.linef("fileData, err := os.ReadFile(%s)", bodyFileVar)
		w.line("if err != nil {")
		w.indent++
		w.linef(`return fmt.Errorf("reading body-file: %%w", err)`)
		w.indent--
		w.line("}")
		w.line("if !json.Valid(fileData) {")
		w.indent++
		w.line(`return fmt.Errorf("body-file does not contain valid JSON")`)
		w.indent--
		w.line("}")
		w.linef("%s = string(fileData)", bodyVar)
		w.indent--
		w.line("}")
		// Decide body source: --body/--body-file override vs individual flags
		w.linef(`if %s != "" {`, bodyVar)
		w.indent++
		w.linef("if !json.Valid([]byte(%s)) {", bodyVar)
		w.indent++
		w.line(`return fmt.Errorf("--body does not contain valid JSON")`)
		w.indent--
		w.line("}")
		w.line("var bodyObj interface{}")
		w.linef("_ = json.Unmarshal([]byte(%s), &bodyObj)", bodyVar)
		w.linef(`resp, err := c.Do(%q, %q, pathParams, queryParams, bodyObj)`, cmd.HTTPMethod, cmd.Path)
		w.line("if err != nil {")
		w.indent++
		w.line("return err")
		w.indent--
		w.line("}")
		w.line(`jsonMode, _ := cmd.Root().PersistentFlags().GetBool("json")`)
		w.line(`noColor, _ := cmd.Root().PersistentFlags().GetBool("no-color")`)
		w.line("if jsonMode {")
		w.indent++
		w.line(`fmt.Printf("%s\n", string(resp))`)
		w.indent--
		w.line("} else {")
		w.indent++
		w.line("if err := output.PrintTable(resp, noColor); err != nil {")
		w.indent++
		w.line(`fmt.Println(string(resp))`)
		w.indent--
		w.line("}")
		w.indent--
		w.line("}")
		w.line("return nil")
		w.indent--
		w.line("}")
	}

	// Build body from individual flags (fallback for write ops when --body/--body-file not set,
	// or for non-write ops that have body flags).
	if hasBodyFlags {
		w.line("bodyMap := map[string]interface{}{}")
		for _, f := range cmd.Flags {
			if f.Source != model.FlagSourceBody {
				continue
			}
			fVar := flagVarName(varName, f.Name)
			parts := strings.Split(f.Name, ".")
			if len(parts) == 1 {
				// Flat flag — direct assignment.
				w.linef("bodyMap[%q] = %s", f.Name, fVar)
			} else if len(parts) <= 3 {
				// Dot-notation: build nested maps using strings.Split path.
				w.linef("{")
				w.indent++
				w.linef("_parts := strings.Split(%q, \".\")", f.Name)
				w.line("_cur := bodyMap")
				w.line("for _, _p := range _parts[:len(_parts)-1] {")
				w.indent++
				w.line("if _, ok := _cur[_p]; !ok {")
				w.indent++
				w.line("_cur[_p] = map[string]interface{}{}")
				w.indent--
				w.line("}")
				// The type assertion below is safe: extractFlagsFromSchema only
				// emits dot-notation flags when a schema property is of object
				// type, so any intermediate key in the path was inserted as
				// map[string]interface{} above. A flat flag and a dot-notation
				// parent sharing the same name cannot coexist from a valid spec.
				w.line("_cur = _cur[_p].(map[string]interface{})")
				w.indent--
				w.line("}")
				w.linef("_cur[_parts[len(_parts)-1]] = %s", fVar)
				w.indent--
				w.line("}")
			}
			// Flags with >3 parts (4+ segments) are skipped per depth limit.
		}
	}

	// Call client.Do — with optional auto-pagination when --all is set.
	bodyArg := "nil"
	if hasBodyFlags {
		bodyArg = "bodyMap"
	}

	allVar := varName + "All"

	if cmd.Pagination != nil {
		// Emit auto-pagination block gated on --all flag.
		// Delegates all page iteration to client.FetchAll which is defined in
		// the generated project's internal/client/pagination.go.
		pType := string(cmd.Pagination.Type)
		pageParam := cmd.Pagination.PageParam
		sizeParam := cmd.Pagination.SizeParam
		cursorParam := cmd.Pagination.CursorParam

		w.linef("if %s {", allVar)
		w.indent++
		w.linef("_cfg := client.PaginationConfig{")
		w.indent++
		w.linef("Type: client.PaginationType(%q),", pType)
		w.linef("PageParam: %q,", pageParam)
		w.linef("SizeParam: %q,", sizeParam)
		w.linef("CursorParam: %q,", cursorParam)
		w.indent--
		w.line("}")
		w.linef("_out, _err := client.FetchAll(c, %q, %q, pathParams, queryParams, _cfg)", cmd.HTTPMethod, cmd.Path)
		w.line("if _err != nil { return _err }")
		w.line(`jsonMode, _ := cmd.Root().PersistentFlags().GetBool("json")`)
		w.line(`noColor, _ := cmd.Root().PersistentFlags().GetBool("no-color")`)
		w.line("if jsonMode {")
		w.indent++
		w.line(`fmt.Printf("%s\n", string(_out))`)
		w.indent--
		w.line("} else {")
		w.indent++
		w.line("if err := output.PrintTable(_out, noColor); err != nil {")
		w.indent++
		w.line(`fmt.Println(string(_out))`)
		w.indent--
		w.line("}")
		w.indent--
		w.line("}")
		w.line("return nil")
		w.indent--
		w.line("}")
	}

	w.linef("resp, err := c.Do(%q, %q, pathParams, queryParams, %s)",
		cmd.HTTPMethod, cmd.Path, bodyArg)
	w.line("if err != nil {")
	w.indent++
	w.line("return err")
	w.indent--
	w.line("}")

	// Read --json flag and output.
	w.line(`jsonMode, _ := cmd.Root().PersistentFlags().GetBool("json")`)
	w.line(`noColor, _ := cmd.Root().PersistentFlags().GetBool("no-color")`)
	w.line("if jsonMode {")
	w.indent++
	w.line(`fmt.Printf("%s\n", string(resp))`)
	w.indent--
	w.line("} else {")
	w.indent++
	w.line("if err := output.PrintTable(resp, noColor); err != nil {")
	w.indent++
	w.line(`fmt.Println(string(resp))`)
	w.indent--
	w.line("}")
	w.indent--
	w.line("}")
	w.line("return nil")

	return w.String()
}

// buildMultipartRunEBody appends multipart/form-data upload code to w and
// returns the complete RunE body string. It is called when the command has at
// least one FlagTypeFile flag. The generated code builds the multipart form
// inline and calls c.DoMultipart with the pre-built body.
func buildMultipartRunEBody(w *codeWriter, cmd model.Command, varName string) string {
	// Collect all file flags and non-file body text flags separately.
	// Every FlagTypeFile flag must produce its own file-reading + CreateFormFile
	// block — not fall through to WriteField as a plain text value.
	fileFlags := []model.Flag{}
	textFlags := []model.Flag{}
	for _, f := range cmd.Flags {
		if f.Type == model.FlagTypeFile {
			fileFlags = append(fileFlags, f)
		} else if f.Source == model.FlagSourceBody {
			textFlags = append(textFlags, f)
		}
	}

	// Build the multipart body using mime/multipart.
	w.line("var _mpBuf bytes.Buffer")
	w.line("_mpWriter := multipart.NewWriter(&_mpBuf)")

	// Declare _mpErr once in the outer scope so it is visible to the
	// _mpWriter.Close() call below and to the plain "=" assignments inside each
	// file-flag block. Using a single outer declaration avoids the "no new
	// variables on left side of :=" compile error that arises when each
	// iteration redeclares the same variable names with := in the same scope.
	w.line("var _mpErr error")

	// For each file flag: read bytes with os.ReadFile, then write as form file.
	// Each file flag's block is wrapped in its own {…} scope so the local
	// variables _mpFileBytes and _mpPart are scoped per-iteration. _mpErr is
	// assigned (not declared) from the outer scope so Close() can check it.
	// os.Open is intentionally omitted — os.ReadFile opens its own handle, so
	// calling os.Open first would create a descriptor that is never read from.
	for _, f := range fileFlags {
		fv := flagVarName(varName, f.Name)
		w.line("{")
		w.indent++
		w.line("var _mpFileBytes []byte")
		w.linef("_mpFileBytes, _mpErr = os.ReadFile(filepath.Clean(%s))", fv)
		w.line("if _mpErr != nil {")
		w.indent++
		w.linef(`return fmt.Errorf("reading file: %%w", _mpErr)`)
		w.indent--
		w.line("}")
		w.line("var _mpPart io.Writer")
		w.linef("_mpPart, _mpErr = _mpWriter.CreateFormFile(%q, filepath.Base(%s))", f.Name, fv)
		w.line("if _mpErr != nil {")
		w.indent++
		w.linef(`return fmt.Errorf("creating form file: %%w", _mpErr)`)
		w.indent--
		w.line("}")
		w.line("if _, _mpErr = _mpPart.Write(_mpFileBytes); _mpErr != nil {")
		w.indent++
		w.linef(`return fmt.Errorf("writing file content: %%w", _mpErr)`)
		w.indent--
		w.line("}")
		w.indent--
		w.line("}")
	}

	// Write text fields. Optional fields are guarded by cmd.Flags().Changed so
	// that the server does not receive an empty string for flags the user omitted.
	for _, f := range textFlags {
		fv := flagVarName(varName, f.Name)
		if f.Required {
			// Required flags are always set; emit WriteField unconditionally.
			w.linef("if _mpErr = _mpWriter.WriteField(%q, %s); _mpErr != nil {", f.Name, fv)
			w.indent++
			w.linef(`return fmt.Errorf("writing form field %s: %%w", _mpErr)`, f.Name)
			w.indent--
			w.line("}")
		} else {
			// Optional flags: only send when the user explicitly provided them.
			w.linef("if cmd.Flags().Changed(%q) {", f.Name)
			w.indent++
			w.linef("if _mpErr = _mpWriter.WriteField(%q, %s); _mpErr != nil {", f.Name, fv)
			w.indent++
			w.linef(`return fmt.Errorf("writing form field %s: %%w", _mpErr)`, f.Name)
			w.indent--
			w.line("}")
			w.indent--
			w.line("}")
		}
	}

	w.line("if _mpErr = _mpWriter.Close(); _mpErr != nil {")
	w.indent++
	w.linef(`return fmt.Errorf("closing multipart writer: %%w", _mpErr)`)
	w.indent--
	w.line("}")

	// Call DoMultipart with the pre-built body.
	w.linef(`resp, err := c.DoMultipart(%q, %q, pathParams, queryParams, &_mpBuf, _mpWriter.FormDataContentType())`,
		cmd.HTTPMethod, cmd.Path)
	w.line("if err != nil {")
	w.indent++
	w.line("return err")
	w.indent--
	w.line("}")
	w.line(`jsonMode, _ := cmd.Root().PersistentFlags().GetBool("json")`)
	w.line(`noColor, _ := cmd.Root().PersistentFlags().GetBool("no-color")`)
	w.line("if jsonMode {")
	w.indent++
	w.line(`fmt.Printf("%s\n", string(resp))`)
	w.indent--
	w.line("} else {")
	w.indent++
	w.line("if err := output.PrintTable(resp, noColor); err != nil {")
	w.indent++
	w.line(`fmt.Println(string(resp))`)
	w.indent--
	w.line("}")
	w.indent--
	w.line("}")
	w.line("return nil")

	return w.String()
}

// cliNameToEnvPrefix converts a cliName (e.g. "my-cli") to an env var prefix
// (e.g. "MY_CLI") by uppercasing and replacing hyphens/dots with underscores.
func cliNameToEnvPrefix(cliName string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(cliName))
}

// argToPathKey converts a kebab-case arg name back to the camelCase path
// parameter key used in the OpenAPI URL template. The model builder converts
// OpenAPI path params like {userId} to kebab-case arg names like "user-id".
// This function reverses that transformation so the generated RunE can look
// up the correct path placeholder when calling client.Do().
// e.g. "user-id" -> "userId", "pet-id" -> "petId"
func argToPathKey(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) == 1 {
		return name
	}
	var sb strings.Builder
	sb.WriteString(parts[0])
	for _, p := range parts[1:] {
		if len(p) > 0 {
			sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return sb.String()
}

// buildUseField constructs the Use string for a Cobra command: the command
// name followed by angle-bracket placeholders for each positional argument.
func buildUseField(cmd model.Command) string {
	parts := []string{cmd.Name}
	for _, arg := range cmd.Args {
		parts = append(parts, "<"+arg.Name+">")
	}
	return strings.Join(parts, " ")
}

// buildArgsExpr returns the cobra.ExactArgs(N) or cobra.NoArgs expression
// for the number of positional arguments the command expects.
func buildArgsExpr(args []model.Arg) string {
	if len(args) == 0 {
		return "cobra.NoArgs"
	}
	return fmt.Sprintf("cobra.ExactArgs(%d)", len(args))
}

// buildFlagVarDeclarations returns a var block declaring one variable per
// flag so that StringVar / IntVar / BoolVar / StringArrayVar can reference
// them. Returns an empty string when there are no flags.
// StringSlice query flags are excluded because they are read inline in RunE
// via cmd.Flags().GetStringArray().
// For write operations, body and bodyFile vars are added.
// For paginated commands, an --all bool var is added.
func buildFlagVarDeclarations(cmdVarName string, flags []model.Flag, writeOp bool, pagination *model.Pagination) string {
	var lines []string
	if writeOp {
		lines = append(lines,
			fmt.Sprintf("\t%sBody string", cmdVarName),
			fmt.Sprintf("\t%sBodyFile string", cmdVarName),
		)
	}
	if pagination != nil {
		lines = append(lines, fmt.Sprintf("\t%sAll bool", cmdVarName))
	}
	for _, flag := range flags {
		if flag.Type == model.FlagTypeStringSlice && flag.Source == model.FlagSourceQuery {
			continue // read inline in RunE, no var needed
		}
		if flag.Source == model.FlagSourceBody && strings.Count(flag.Name, ".") >= 3 {
			fmt.Fprintf(os.Stderr, "warning: flag %q exceeds max nesting depth (3 levels), skipping\n", flag.Name)
			continue // 4+ part dot-notation flags exceed depth limit, skipped in RunE
		}
		goType := flagGoType(flag.Type)
		varName := flagVarName(cmdVarName, flag.Name)
		lines = append(lines, fmt.Sprintf("\t%s %s", varName, goType))
	}
	if len(lines) == 0 {
		return ""
	}
	return "var (\n" + strings.Join(lines, "\n") + "\n)\n"
}

// buildFlagInits returns the lines that register each flag with Cobra inside
// the init() function. Each line is indented by one tab.
// For write operations, --body and --body-file flag registrations are prepended.
// For paginated commands, an --all flag registration is prepended.
func buildFlagInits(cmdVarName string, flags []model.Flag, writeOp bool, pagination *model.Pagination) string {
	var lines []string
	if writeOp {
		lines = append(lines,
			fmt.Sprintf("\t%s.Flags().StringVar(&%sBody, \"body\", \"\", \"Raw JSON body (overrides individual flags)\")", cmdVarName, cmdVarName),
			fmt.Sprintf("\t%s.Flags().StringVar(&%sBodyFile, \"body-file\", \"\", \"Path to JSON file to use as request body\")", cmdVarName, cmdVarName),
		)
	}
	if pagination != nil {
		lines = append(lines,
			fmt.Sprintf("\t%s.Flags().BoolVar(&%sAll, \"all\", false, \"Auto-paginate through all pages\")", cmdVarName, cmdVarName),
		)
	}
	for _, flag := range flags {
		if flag.Source == model.FlagSourceBody && strings.Count(flag.Name, ".") >= 3 {
			fmt.Fprintf(os.Stderr, "warning: flag %q exceeds max nesting depth (3 levels), skipping\n", flag.Name)
			continue // 4+ part dot-notation flags exceed depth limit, not registered
		}
		var line string
		if flag.Type == model.FlagTypeStringSlice && flag.Source == model.FlagSourceQuery {
			// Read inline in RunE — register without a pre-declared var.
			desc := buildFlagDescription(flag)
			line = fmt.Sprintf(`%s.Flags().StringArray(%q, nil, %q)`,
				cmdVarName, flag.Name, desc)
		} else {
			varName := flagVarName(cmdVarName, flag.Name)
			line = buildFlagRegistration(cmdVarName, varName, flag)
		}
		lines = append(lines, "\t"+line)

		// Register completion function for enum flags.
		if len(flag.Enum) > 0 {
			enumLiterals := make([]string, len(flag.Enum))
			for i, v := range flag.Enum {
				enumLiterals[i] = fmt.Sprintf("%q", v)
			}
			completionLine := fmt.Sprintf(
				"\t%s.RegisterFlagCompletionFunc(%q, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {\n\t\treturn []string{%s}, cobra.ShellCompDirectiveNoFileComp\n\t})",
				cmdVarName, flag.Name, strings.Join(enumLiterals, ", "),
			)
			lines = append(lines, completionLine)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// buildFlagDescription returns the flag description, appending enum values if present.
func buildFlagDescription(flag model.Flag) string {
	desc := flag.Description
	if len(flag.Enum) > 0 {
		if desc != "" {
			desc += " (" + strings.Join(flag.Enum, "|") + ")"
		} else {
			desc = "(" + strings.Join(flag.Enum, "|") + ")"
		}
	}
	return desc
}

// buildFlagRegistration produces the single Flags().XxxVar(...) call for one flag.
func buildFlagRegistration(cmdVarName, varName string, flag model.Flag) string {
	desc := buildFlagDescription(flag)
	switch flag.Type {
	case model.FlagTypeInt:
		return fmt.Sprintf(`%s.Flags().IntVar(&%s, %q, 0, %q)`,
			cmdVarName, varName, flag.Name, desc)
	case model.FlagTypeBool:
		return fmt.Sprintf(`%s.Flags().BoolVar(&%s, %q, false, %q)`,
			cmdVarName, varName, flag.Name, desc)
	case model.FlagTypeStringSlice:
		return fmt.Sprintf(`%s.Flags().StringArrayVar(&%s, %q, nil, %q)`,
			cmdVarName, varName, flag.Name, desc)
	default: // FlagTypeString
		return fmt.Sprintf(`%s.Flags().StringVar(&%s, %q, "", %q)`,
			cmdVarName, varName, flag.Name, desc)
	}
}

// buildRequiredFlagInits returns the MarkFlagRequired lines for flags that
// are marked Required: true. Each line is indented by one tab.
func buildRequiredFlagInits(cmdVarName string, flags []model.Flag) string {
	var lines []string
	for _, flag := range flags {
		if flag.Required {
			lines = append(lines, fmt.Sprintf("\t%s.MarkFlagRequired(%q)", cmdVarName, flag.Name))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// flagGoType maps a model.FlagType to its Go type string for variable
// declarations.
func flagGoType(t model.FlagType) string {
	switch t {
	case model.FlagTypeInt:
		return "int"
	case model.FlagTypeBool:
		return "bool"
	case model.FlagTypeStringSlice:
		return "[]string"
	default:
		return "string"
	}
}

// flagVarName produces a valid Go identifier for a flag variable by
// combining the command var name with a sanitized version of the flag name.
func flagVarName(cmdVarName, flagName string) string {
	return cmdVarName + "_" + sanitizeIdentifier(flagName)
}

// capitalise returns the string with its first rune uppercased.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// sanitizeIdentifier converts a kebab-case or dot-notation name into a valid
// Go identifier by replacing hyphens and dots with underscores and
// capitalising each segment after the first.
func sanitizeIdentifier(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '.' || r == '_'
	})
	if len(parts) == 0 {
		return "_"
	}
	var sb strings.Builder
	sb.WriteString(parts[0])
	for _, p := range parts[1:] {
		if len(p) > 0 {
			sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return sb.String()
}

// validateGoSource parses the provided source string to confirm it is
// syntactically valid Go and returns it unchanged. An error is returned when
// the source cannot be parsed, with the raw source appended for debugging.
func validateGoSource(src string) (string, error) {
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "", src, parser.AllErrors); err != nil {
		return "", fmt.Errorf("generated Go source has syntax errors: %w\n---\n%s", err, src)
	}
	return src, nil
}
