package generator

import (
	"fmt"
	"go/parser"
	"go/token"
	"strings"

	"github.com/queso/swagger-jack/internal/model"
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
	flagVars := buildFlagVarDeclarations(varName, cmd.Flags, writeOp)
	flagInits := buildFlagInits(varName, cmd.Flags, writeOp)
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
	hasIntOrBoolQueryFlag := false
	hasStringSliceQueryFlag := false
	hasDotNotationBodyFlag := false
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
	}

	var imports []string
	if cliName != "" {
		imports = append(imports, `"encoding/json"`)
	}
	imports = append(imports, `"fmt"`, `"os"`)
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
	}
	return "import (\n\t" + strings.Join(imports, "\n\t") + "\n)"
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

	// Runtime validation for enum flags.
	for _, f := range cmd.Flags {
		if len(f.Enum) == 0 {
			continue
		}
		fVar := flagVarName(varName, f.Name)
		enumLiterals := make([]string, len(f.Enum))
		for i, v := range f.Enum {
			enumLiterals[i] = fmt.Sprintf("%q", v)
		}
		allowedExpr := "{" + strings.Join(enumLiterals, ", ") + "}"
		if f.Required {
			// Always validate required enum flags.
			w.linef("_allowedFor%s := []string%s", sanitizeIdentifier(f.Name), allowedExpr)
			w.linef("_validFor%s := false", sanitizeIdentifier(f.Name))
			w.linef("for _, _v := range _allowedFor%s { if _v == %s { _validFor%s = true; break } }", sanitizeIdentifier(f.Name), fVar, sanitizeIdentifier(f.Name))
			w.linef("if !_validFor%s {", sanitizeIdentifier(f.Name))
			w.indent++
			w.linef(`return fmt.Errorf("invalid value %%q for --%s: must be one of: %s", %s)`, f.Name, strings.Join(f.Enum, ", "), fVar)
			w.indent--
			w.line("}")
		} else {
			// Only validate optional flags when explicitly set.
			w.linef(`if cmd.Flags().Changed(%q) {`, f.Name)
			w.indent++
			w.linef("_allowedFor%s := []string%s", sanitizeIdentifier(f.Name), allowedExpr)
			w.linef("_validFor%s := false", sanitizeIdentifier(f.Name))
			w.linef("for _, _v := range _allowedFor%s { if _v == %s { _validFor%s = true; break } }", sanitizeIdentifier(f.Name), fVar, sanitizeIdentifier(f.Name))
			w.linef("if !_validFor%s {", sanitizeIdentifier(f.Name))
			w.indent++
			w.linef(`return fmt.Errorf("invalid value %%q for --%s: must be one of: %s", %s)`, f.Name, strings.Join(f.Enum, ", "), fVar)
			w.indent--
			w.line("}")
			w.indent--
			w.line("}")
		}
	}

	// Build body — support --body/--body-file overrides for write operations.
	if writeOp {
		// --body-file: read JSON from file, set body string
		w.line(`if bodyFile != "" {`)
		w.indent++
		w.line("fileData, err := os.ReadFile(bodyFile)")
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
		w.line("body = string(fileData)")
		w.indent--
		w.line("}")
		// Decide body source: --body/--body-file override vs individual flags
		w.line(`if body != "" {`)
		w.indent++
		w.line("if !json.Valid([]byte(body)) {")
		w.indent++
		w.line(`return fmt.Errorf("--body does not contain valid JSON")`)
		w.indent--
		w.line("}")
		w.line("var bodyObj interface{}")
		w.line("_ = json.Unmarshal([]byte(body), &bodyObj)")
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

	// Call client.Do.
	bodyArg := "nil"
	if hasBodyFlags {
		bodyArg = "bodyMap"
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
func buildFlagVarDeclarations(cmdVarName string, flags []model.Flag, writeOp bool) string {
	var lines []string
	if writeOp {
		lines = append(lines, "\tbody string", "\tbodyFile string")
	}
	for _, flag := range flags {
		if flag.Type == model.FlagTypeStringSlice && flag.Source == model.FlagSourceQuery {
			continue // read inline in RunE, no var needed
		}
		if flag.Source == model.FlagSourceBody && strings.Count(flag.Name, ".") >= 3 {
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
func buildFlagInits(cmdVarName string, flags []model.Flag, writeOp bool) string {
	var lines []string
	if writeOp {
		lines = append(lines,
			fmt.Sprintf("\t%s.Flags().StringVar(&body, \"body\", \"\", \"Raw JSON body (overrides individual flags)\")", cmdVarName),
			fmt.Sprintf("\t%s.Flags().StringVar(&bodyFile, \"body-file\", \"\", \"Path to JSON file to use as request body\")", cmdVarName),
		)
	}
	for _, flag := range flags {
		if flag.Source == model.FlagSourceBody && strings.Count(flag.Name, ".") >= 3 {
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
