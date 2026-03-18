package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/theaiteam-dev/commandspec/internal/generator"
	"github.com/theaiteam-dev/commandspec/internal/model"
	"github.com/theaiteam-dev/commandspec/internal/parser"
	"github.com/theaiteam-dev/commandspec/internal/preserve"
	"github.com/spf13/cobra"
)

// newUpdateCmd constructs the update subcommand that regenerates an existing
// CLI project in place, preserving custom code blocks and reporting diffs.
func newUpdateCmd() *cobra.Command {
	var schemaPath string
	var cliName string
	var outputDir string
	var dryRun bool
	var noDiff bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Regenerate an existing CLI project from an updated OpenAPI spec",
		Long:  "Update reruns the full parse → model → generate pipeline, preserving custom code blocks and reporting file-level diffs.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUpdate(cmd, schemaPath, cliName, outputDir, dryRun, noDiff, timeout)
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

	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory containing the existing generated project (defaults to ./<name>)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would change without writing any files")
	cmd.Flags().BoolVar(&noDiff, "no-diff", false, "Suppress unified diff output for modified files")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "HTTP timeout for fetching remote schemas")

	return cmd
}

// updateStats tracks counts for the final summary report.
type updateStats struct {
	added     int
	modified  int
	unchanged int
	warned    int
}

// runUpdate executes the update pipeline:
// load spec → build model → generate file map → diff/write each file → summary.
func runUpdate(cmd *cobra.Command, schemaPath, cliName, outputDir string, dryRun, noDiff bool, timeout time.Duration) error {
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

	resolvedOutputDir := outputDir
	if resolvedOutputDir == "" {
		resolvedOutputDir = filepath.Join(".", cliName)
	}

	// Generate all file contents in memory (relative path → content).
	fileMap, err := buildFileMap(spec, cliName)
	if err != nil {
		return fmt.Errorf("failed to generate file contents: %w", err)
	}

	out := cmd.OutOrStdout()
	var stats updateStats

	if dryRun {
		// Dry-run: report what would change without writing anything.
		for relPath := range fileMap {
			absPath := filepath.Join(resolvedOutputDir, relPath)
			existing, readErr := os.ReadFile(absPath)
			switch {
			case readErr != nil:
				_, _ = fmt.Fprintf(out, "  [would add]    %s\n", relPath)
				stats.added++
			case string(existing) != fileMap[relPath]:
				_, _ = fmt.Fprintf(out, "  [would modify] %s\n", relPath)
				stats.modified++
			default:
				_, _ = fmt.Fprintf(out, "  [unchanged]    %s\n", relPath)
				stats.unchanged++
			}
		}
		printSummary(out, stats)
		return nil
	}

	// Create directory layout before writing files.
	if err := generator.EnsureDirectories(resolvedOutputDir); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Process each file that would be generated.
	for relPath, newContent := range fileMap {
		absPath := filepath.Join(resolvedOutputDir, relPath)
		existing, readErr := os.ReadFile(absPath)
		if readErr != nil {
			// New file — create it.
			if writeErr := writeFileContent(absPath, newContent); writeErr != nil {
				return writeErr
			}
			_, _ = fmt.Fprintf(out, "  added:     %s\n", relPath)
			stats.added++
			continue
		}

		// Existing file — extract custom blocks, merge into new content.
		existingContent := string(existing)
		blocks, extractErr := preserve.Extract(existingContent)
		if extractErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warn: could not extract custom blocks from %s: %v\n", relPath, extractErr)
			blocks = nil
		}

		merged, mergeErr := preserve.Merge(newContent, blocks)
		if mergeErr != nil {
			merged = newContent
		}

		if merged == existingContent {
			_, _ = fmt.Fprintf(out, "  unchanged: %s\n", relPath)
			stats.unchanged++
			continue
		}

		// File changed — write merged content and optionally show diff.
		if !noDiff {
			diff := computeUnifiedDiff(relPath, existingContent, merged)
			if diff != "" {
				_, _ = fmt.Fprint(out, diff)
			}
		}

		if writeErr := writeFileContent(absPath, merged); writeErr != nil {
			return writeErr
		}
		_, _ = fmt.Fprintf(out, "  modified:  %s\n", relPath)
		stats.modified++
	}

	// Warn about files on disk that have no corresponding generated file.
	existingFiles, walkErr := collectGeneratedFiles(resolvedOutputDir)
	if walkErr == nil {
		for _, absPath := range existingFiles {
			relPath, relErr := filepath.Rel(resolvedOutputDir, absPath)
			if relErr != nil {
				continue
			}
			if _, inMap := fileMap[relPath]; !inMap {
				_, _ = fmt.Fprintf(out, "  warn: file not in current spec (would be removed if cleaned): %s\n", relPath)
				stats.warned++
			}
		}
	}

	printSummary(out, stats)
	return nil
}

// buildFileMap generates all file contents in memory and returns a map from
// relative path to file content string.
func buildFileMap(spec *model.APISpec, name string) (map[string]string, error) {
	files := make(map[string]string)

	mainSrc, err := generator.GenerateMain(name)
	if err != nil {
		return nil, fmt.Errorf("generating main.go: %w", err)
	}
	files["main.go"] = mainSrc

	goModSrc := generator.GenerateGoMod(name)
	files["go.mod"] = goModSrc

	rootSrc, err := generator.GenerateRoot(spec, name)
	if err != nil {
		return nil, fmt.Errorf("generating root command: %w", err)
	}
	files[filepath.Join("cmd", "root.go")] = rootSrc

	completionSrc, err := generator.GenerateCompletion(name)
	if err != nil {
		return nil, fmt.Errorf("generating completion command: %w", err)
	}
	files[filepath.Join("cmd", "completion.go")] = completionSrc

	reservedCmdNames := map[string]bool{"root": true, "completion": true}
	for _, resource := range spec.Resources {
		if reservedCmdNames[resource.Name] {
			continue
		}
		resourceSrc, err := generator.GenerateResourceCmd(resource)
		if err != nil {
			return nil, fmt.Errorf("generating resource command for %q: %w", resource.Name, err)
		}
		files[filepath.Join("cmd", resource.Name+".go")] = resourceSrc

		for _, cmd := range resource.Commands {
			verbSrc, err := generator.GenerateVerbCmd(resource, cmd, name)
			if err != nil {
				return nil, fmt.Errorf("generating verb command %q for resource %q: %w", cmd.Name, resource.Name, err)
			}
			filename := resource.Name + "_" + cmd.Name + ".go"
			files[filepath.Join("cmd", filename)] = verbSrc
		}
	}

	clientSrc, err := generator.GenerateClient(spec)
	if err != nil {
		return nil, fmt.Errorf("generating client: %w", err)
	}
	files[filepath.Join("internal", "client", "client.go")] = clientSrc

	paginationSrc, err := generator.GeneratePagination(name)
	if err != nil {
		return nil, fmt.Errorf("generating pagination helper: %w", err)
	}
	files[filepath.Join("internal", "client", "pagination.go")] = paginationSrc

	configSrc, err := generator.GenerateConfig(spec, name)
	if err != nil {
		return nil, fmt.Errorf("generating config: %w", err)
	}
	files[filepath.Join("internal", "config.go")] = configSrc

	outputSrc, err := generator.GenerateOutput()
	if err != nil {
		return nil, fmt.Errorf("generating output helpers: %w", err)
	}
	files[filepath.Join("internal", "output", "output.go")] = outputSrc

	errorsSrc, err := generator.GenerateErrors()
	if err != nil {
		return nil, fmt.Errorf("generating error helpers: %w", err)
	}
	files[filepath.Join("internal", "errors.go")] = errorsSrc

	validateSrc, err := generator.GenerateValidate()
	if err != nil {
		return nil, fmt.Errorf("generating validate helpers: %w", err)
	}
	files[filepath.Join("internal", "validate", "validate.go")] = validateSrc

	return files, nil
}

// writeFileContent writes content to absPath, creating parent directories as needed.
func writeFileContent(absPath, content string) error {
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", absPath, err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", absPath, err)
	}
	return nil
}

// computeUnifiedDiff produces a unified diff string between oldContent and newContent.
func computeUnifiedDiff(filename, oldContent, newContent string) string {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: "a/" + filename,
		ToFile:   "b/" + filename,
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return ""
	}
	return text
}

// collectGeneratedFiles walks outputDir and returns absolute paths to all
// generated Go source files. Only .go files are included because the generator
// exclusively produces Go source; non-generated files like go.sum, .gitignore,
// and README.md must not be mistaken for orphaned generated files.
func collectGeneratedFiles(outputDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

// printSummary writes the final change summary to out.
func printSummary(out interface{ Write([]byte) (int, error) }, stats updateStats) {
	parts := []string{
		fmt.Sprintf("%d added", stats.added),
		fmt.Sprintf("%d modified", stats.modified),
		fmt.Sprintf("%d unchanged", stats.unchanged),
	}
	if stats.warned > 0 {
		parts = append(parts, fmt.Sprintf("%d warned (not in spec)", stats.warned))
	}
	_, _ = fmt.Fprintf(out, "\nSummary: %s\n", strings.Join(parts, ", "))
}
