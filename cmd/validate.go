package cmd

import (
	"fmt"
	"strings"

	"github.com/queso/swagger-jack/internal/model"
	"github.com/queso/swagger-jack/internal/parser"
	"github.com/spf13/cobra"
)

// newValidateCmd constructs the validate subcommand.
func newValidateCmd() *cobra.Command {
	var schemaPath string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate an OpenAPI spec file",
		Long:  "Validate reads an OpenAPI 3.x spec and reports the title, version, resource count, and command count.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runValidate(cmd, schemaPath)
		},
	}

	cmd.Flags().StringVar(&schemaPath, "schema", "", "Path to the OpenAPI spec file (required)")
	if err := cmd.MarkFlagRequired("schema"); err != nil {
		panic(fmt.Sprintf("failed to mark --schema as required: %v", err))
	}

	return cmd
}

// runValidate loads the spec, builds the model, and prints the summary.
func runValidate(cmd *cobra.Command, schemaPath string) error {
	out := cmd.OutOrStdout()

	result, err := parser.Load(schemaPath)
	if err != nil {
		_, _ = fmt.Fprintf(out, "Error: %v\n", err)
		return fmt.Errorf("failed to load spec: %w", err)
	}

	resources, err := model.Build(result)
	if err != nil {
		_, _ = fmt.Fprintf(out, "Error: %v\n", err)
		return fmt.Errorf("failed to build model: %w", err)
	}

	totalCommands := countCommands(resources)

	_, _ = fmt.Fprintf(out, "Spec: %s (%s)\n", result.Spec.Title, result.Spec.Version)
	_, _ = fmt.Fprintf(out, "%d resources\n", len(resources))
	_, _ = fmt.Fprintf(out, "%d commands\n", totalCommands)
	_, _ = fmt.Fprintf(out, "Auth: %s\n", detectAuthDescription(result.Spec))

	return nil
}

// detectAuthDescription returns a human-readable description of detected auth schemes.
func detectAuthDescription(spec *model.APISpec) string {
	if len(spec.SecuritySchemes) == 0 {
		return "None detected"
	}

	var parts []string
	for _, scheme := range spec.SecuritySchemes {
		switch scheme.Type {
		case model.SecuritySchemeBearer:
			parts = append(parts, "Bearer token")
		case model.SecuritySchemeAPIKey:
			if scheme.HeaderName != "" {
				parts = append(parts, fmt.Sprintf("API key (%s)", scheme.HeaderName))
			} else {
				parts = append(parts, "API key")
			}
		case model.SecuritySchemeBasic:
			parts = append(parts, "Basic")
		default:
			parts = append(parts, string(scheme.Type))
		}
	}

	if len(parts) == 0 {
		return "None detected"
	}
	return strings.Join(parts, ", ")
}

// countCommands sums the number of commands across all resources.
func countCommands(resources []model.Resource) int {
	total := 0
	for _, r := range resources {
		total += len(r.Commands)
	}
	return total
}
