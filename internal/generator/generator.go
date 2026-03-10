package generator

import (
	"fmt"
	"go/format"
	gotoken "go/token"
	"os"
	"path/filepath"

	"github.com/queso/swagger-jack/internal/model"
)

// Generate creates a buildable Go CLI project in outputDir for the given spec
// and CLI name. It produces:
//
//   - outputDir/cmd/                         (directory)
//   - outputDir/internal/                    (directory)
//   - outputDir/main.go                      (package main with cmd.Execute())
//   - outputDir/go.mod                       (module declaration + cobra dependency)
//   - outputDir/cmd/root.go                  (root cobra command + global flags)
//   - outputDir/cmd/<resource>.go            (one group command per resource)
//   - outputDir/cmd/<resource>_<verb>.go     (one leaf command per operation)
//   - outputDir/internal/client/client.go    (HTTP client)
//   - outputDir/internal/output/output.go    (output helpers)
//   - outputDir/internal/config.go           (config loader)
//   - outputDir/internal/errors.go           (error helpers)
func Generate(spec *model.APISpec, name string, outputDir string) error {
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}

	// Validate name contains only characters safe for shell environment variable
	// names and go.mod module paths: alphanumerics, hyphens, and dots.
	// Characters like @, (, ), ! would pass through cliNameToEnvPrefix unchanged
	// and produce invalid shell variable names (e.g. "my.api@v1" → "MY_API@V1"),
	// causing auth tokens to be silently unreadable.
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '.' {
			return fmt.Errorf("name %q contains invalid character %q: only alphanumerics, hyphens, and dots are allowed", name, r)
		}
	}

	// Reject reserved Go keywords — they cannot be used as identifiers.
	if gotoken.IsKeyword(name) {
		return fmt.Errorf("name %q is a reserved Go keyword", name)
	}

	if err := createDirectoryLayout(outputDir); err != nil {
		return err
	}

	if err := writeMainGo(outputDir, name); err != nil {
		return err
	}

	if err := writeGoMod(outputDir, name); err != nil {
		return err
	}

	// cmd/root.go
	rootSrc, err := GenerateRoot(spec, name)
	if err != nil {
		return fmt.Errorf("generating root command: %w", err)
	}
	if err := writeFile(filepath.Join(outputDir, "cmd", "root.go"), rootSrc); err != nil {
		return err
	}

	// cmd/completion.go
	completionSrcStr, err := GenerateCompletion(name)
	if err != nil {
		return fmt.Errorf("generating completion command: %w", err)
	}
	if err := writeFile(filepath.Join(outputDir, "cmd", "completion.go"), completionSrcStr); err != nil {
		return err
	}

	// cmd/<resource>.go and cmd/<resource>_<verb>.go for each resource
	for _, resource := range spec.Resources {
		resourceSrc, err := GenerateResourceCmd(resource)
		if err != nil {
			return fmt.Errorf("generating resource command for %q: %w", resource.Name, err)
		}
		if err := writeFile(filepath.Join(outputDir, "cmd", resource.Name+".go"), resourceSrc); err != nil {
			return err
		}

		for _, cmd := range resource.Commands {
			verbSrc, err := GenerateVerbCmd(resource, cmd, name)
			if err != nil {
				return fmt.Errorf("generating verb command %q for resource %q: %w", cmd.Name, resource.Name, err)
			}
			filename := resource.Name + "_" + cmd.Name + ".go"
			if err := writeFile(filepath.Join(outputDir, "cmd", filename), verbSrc); err != nil {
				return err
			}
		}
	}

	// internal/client/client.go — package client, imported as <name>/internal/client
	clientSrc, err := GenerateClient(spec)
	if err != nil {
		return fmt.Errorf("generating client: %w", err)
	}
	if err := writeFile(filepath.Join(outputDir, "internal", "client", "client.go"), clientSrc); err != nil {
		return err
	}

	// internal/config.go
	configSrc, err := GenerateConfig(spec, name)
	if err != nil {
		return fmt.Errorf("generating config: %w", err)
	}
	if err := writeFile(filepath.Join(outputDir, "internal", "config.go"), configSrc); err != nil {
		return err
	}

	// internal/output/output.go
	outputSrc, err := GenerateOutput()
	if err != nil {
		return fmt.Errorf("generating output helpers: %w", err)
	}
	if err := writeFile(filepath.Join(outputDir, "internal", "output", "output.go"), outputSrc); err != nil {
		return err
	}

	// internal/errors.go
	errorsSrc, err := GenerateErrors()
	if err != nil {
		return fmt.Errorf("generating error helpers: %w", err)
	}
	if err := writeFile(filepath.Join(outputDir, "internal", "errors.go"), errorsSrc); err != nil {
		return err
	}

	return nil
}

// writeFile creates or overwrites the file at path with the given content.
func writeFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// createDirectoryLayout creates the cmd/ and internal/ subdirectories under
// outputDir. It returns an error if any directory cannot be created.
func createDirectoryLayout(outputDir string) error {
	dirs := []string{
		filepath.Join(outputDir, "cmd"),
		filepath.Join(outputDir, "internal"),
		filepath.Join(outputDir, "internal", "client"),
		filepath.Join(outputDir, "internal", "output"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return nil
}

// writeMainGo writes a gofmt-formatted main.go into outputDir.
func writeMainGo(outputDir, name string) error {
	src := fmt.Sprintf(`package main

import "%s/cmd"

func main() {
	cmd.Execute()
}
`, name)

	formatted, err := format.Source([]byte(src))
	if err != nil {
		return fmt.Errorf("formatting main.go: %w", err)
	}

	mainGoPath := filepath.Join(outputDir, "main.go")
	if err := os.WriteFile(mainGoPath, formatted, 0o644); err != nil {
		return fmt.Errorf("writing main.go: %w", err)
	}

	return nil
}

// writeGoMod writes a go.mod file into outputDir declaring the module name,
// the Go version, and a require block with the cobra dependency.
func writeGoMod(outputDir, name string) error {
	content := fmt.Sprintf(`module %s

go 1.21

require (
	github.com/olekukonko/tablewriter v0.0.5
	github.com/spf13/cobra v1.8.0
)
`, name)

	goModPath := filepath.Join(outputDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	return nil
}
