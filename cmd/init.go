package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/theaiteam-dev/commandspec/internal/parser"
	"github.com/spf13/cobra"
)

// newInitCmd constructs the init subcommand that runs the full pipeline:
// load spec → build model → generate project.
func newInitCmd() *cobra.Command {
	var schemaPath string
	var cliName string
	var outputDir string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a new CLI project from an OpenAPI spec",
		Long:  "Init reads an OpenAPI 3.x spec and generates a complete, buildable Go CLI project using Cobra.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, schemaPath, cliName, outputDir, timeout)
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

	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for the generated project (defaults to ./<name>)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "HTTP timeout for fetching remote schemas")

	return cmd
}

// runInit executes the full init pipeline: load, build model, extract security,
// generate project, and print the summary and next steps.
func runInit(cmd *cobra.Command, schemaPath, cliName, outputDir string, timeout time.Duration) error {
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

	resolvedOutputDir := outputDir
	if resolvedOutputDir == "" {
		resolvedOutputDir = filepath.Join(".", cliName)
	}

	if err := generator.Generate(spec, cliName, resolvedOutputDir); err != nil {
		return fmt.Errorf("failed to generate project: %w", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = resolvedOutputDir
	if tidyErr := tidyCmd.Run(); tidyErr != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "warning: go mod tidy failed (network may be unavailable): %v\n", tidyErr)
	}

	totalCommands := countCommands(resources)
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Generated %d resource(s) with %d command(s)\n", len(resources), totalCommands)
	_, _ = fmt.Fprintf(out, "\nNext steps:\n")
	_, _ = fmt.Fprintf(out, "  cd %s\n", resolvedOutputDir)
	_, _ = fmt.Fprintf(out, "  go build -o %s .\n", cliName)
	_, _ = fmt.Fprintf(out, "  ./%s --help\n", cliName)

	return nil
}
