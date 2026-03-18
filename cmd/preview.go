package cmd

import (
	"fmt"
	"time"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/theaiteam-dev/commandspec/internal/parser"
	"github.com/spf13/cobra"
)

// newPreviewCmd constructs the preview subcommand that dry-runs the full
// pipeline without writing any files to disk.
func newPreviewCmd() *cobra.Command {
	var schemaPath string
	var cliName string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Dry-run: show what files would be generated from an OpenAPI spec",
		Long:  "Preview runs the full parse → model → generate pipeline but does not write any files.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPreview(cmd, schemaPath, cliName, timeout)
		},
	}

	cmd.Flags().StringVar(&schemaPath, "schema", "", "Path to the OpenAPI spec file (required)")
	if err := cmd.MarkFlagRequired("schema"); err != nil {
		panic(fmt.Sprintf("failed to mark --schema as required: %v", err))
	}

	cmd.Flags().StringVar(&cliName, "name", "", "Name for the generated CLI (required)")
	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(fmt.Sprintf("failed to mark --name as required: %v", err))
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "HTTP timeout for fetching remote schemas")

	return cmd
}

// runPreview executes the preview pipeline and prints a summary to cmd's output.
func runPreview(cmd *cobra.Command, schemaPath, cliName string, timeout time.Duration) error {
	// Validate name with the same rules as generator.Generate().
	if err := generator.ValidateName(cliName); err != nil {
		return err
	}

	result, err := parser.LoadWithTimeout(schemaPath, timeout)
	if err != nil {
		return fmt.Errorf("failed to load spec: %w", err)
	}

	resources, err := model.Build(result)
	if err != nil {
		return fmt.Errorf("failed to build model: %w", err)
	}

	schemes, err := model.ExtractSecuritySchemes(result, cliName)
	if err != nil {
		return fmt.Errorf("failed to extract security schemes: %w", err)
	}

	spec := &model.APISpec{
		Title:           result.Spec.Title,
		Version:         result.Spec.Version,
		BaseURL:         result.Spec.BaseURL,
		Resources:       resources,
		SecuritySchemes: schemes,
	}

	files, err := generator.Preview(spec, cliName)
	if err != nil {
		return fmt.Errorf("failed to preview: %w", err)
	}

	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintf(out, "Preview: files that would be generated for %q\n\n", cliName)
	for _, f := range files {
		_, _ = fmt.Fprintf(out, "  %s\n", f)
	}

	totalCommands := countCommands(resources)
	_, _ = fmt.Fprintf(out, "\nResources: %d, Commands: %d\n", len(resources), totalCommands)

	if len(schemes) > 0 {
		_, _ = fmt.Fprintf(out, "\nSecurity schemes detected:\n")
		for name, scheme := range schemes {
			_, _ = fmt.Fprintf(out, "  %s (%s)\n", name, scheme.Type)
		}
	} else {
		_, _ = fmt.Fprintf(out, "\nSecurity schemes: none\n")
	}

	return nil
}
