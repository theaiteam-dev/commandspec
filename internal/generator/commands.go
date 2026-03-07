package generator

import (
	"fmt"
	"go/parser"
	"go/token"
	"strings"

	"github.com/queso/swagger-jack/internal/model"
)

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
// flags.
func GenerateVerbCmd(resource model.Resource, cmd model.Command) (string, error) {
	useField := buildUseField(cmd)
	argsExpr := buildArgsExpr(cmd.Args)
	varName := sanitizeIdentifier(resource.Name) + capitalise(sanitizeIdentifier(cmd.Name)) + "Cmd"
	resourceVarName := sanitizeIdentifier(resource.Name) + "Cmd"

	flagVars := buildFlagVarDeclarations(varName, cmd.Flags)
	flagInits := buildFlagInits(varName, cmd.Flags)
	requiredInits := buildRequiredFlagInits(varName, cmd.Flags)

	src := fmt.Sprintf(`package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

%s

var %s = &cobra.Command{
	Use: %q,
	Short: %q,
	Args: %s,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s %s\n")
	},
}

func init() {
	%s.AddCommand(%s)
%s%s}
`,
		flagVars,
		varName,
		useField,
		cmd.Description,
		argsExpr,
		cmd.HTTPMethod,
		cmd.Path,
		resourceVarName,
		varName,
		flagInits,
		requiredInits,
	)

	return validateGoSource(src)
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
func buildFlagVarDeclarations(cmdVarName string, flags []model.Flag) string {
	if len(flags) == 0 {
		return ""
	}

	var lines []string
	for _, flag := range flags {
		goType := flagGoType(flag.Type)
		varName := flagVarName(cmdVarName, flag.Name)
		lines = append(lines, fmt.Sprintf("\t%s %s", varName, goType))
	}

	return "var (\n" + strings.Join(lines, "\n") + "\n)\n"
}

// buildFlagInits returns the lines that register each flag with Cobra inside
// the init() function. Each line is indented by one tab.
func buildFlagInits(cmdVarName string, flags []model.Flag) string {
	if len(flags) == 0 {
		return ""
	}

	var lines []string
	for _, flag := range flags {
		varName := flagVarName(cmdVarName, flag.Name)
		line := buildFlagRegistration(cmdVarName, varName, flag)
		lines = append(lines, "\t"+line)
	}

	return strings.Join(lines, "\n") + "\n"
}

// buildFlagRegistration produces the single Flags().XxxVar(...) call for one flag.
func buildFlagRegistration(cmdVarName, varName string, flag model.Flag) string {
	switch flag.Type {
	case model.FlagTypeInt:
		return fmt.Sprintf(`%s.Flags().IntVar(&%s, %q, 0, %q)`,
			cmdVarName, varName, flag.Name, flag.Description)
	case model.FlagTypeBool:
		return fmt.Sprintf(`%s.Flags().BoolVar(&%s, %q, false, %q)`,
			cmdVarName, varName, flag.Name, flag.Description)
	case model.FlagTypeStringSlice:
		return fmt.Sprintf(`%s.Flags().StringArrayVar(&%s, %q, nil, %q)`,
			cmdVarName, varName, flag.Name, flag.Description)
	default: // FlagTypeString
		return fmt.Sprintf(`%s.Flags().StringVar(&%s, %q, "", %q)`,
			cmdVarName, varName, flag.Name, flag.Description)
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
