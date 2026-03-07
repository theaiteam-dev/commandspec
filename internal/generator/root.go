package generator

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/queso/swagger-jack/internal/model"
)

// rootTemplate is the Go source template for the generated project's cmd/root.go.
// It wires up the root cobra.Command, registers all required global persistent
// flags, and exports the Execute() entry point called from main.
const rootTemplate = `package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   {{goString .Name}},
	Short: {{goString .Title}},
}

// Execute is the conventional cobra entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("json", false, "Output raw JSON")
	rootCmd.PersistentFlags().Bool("verbose", false, "Verbose output")
	rootCmd.PersistentFlags().String("config", "", "Config file path")
	rootCmd.PersistentFlags().String("base-url", {{goString .BaseURL}}, "API base URL")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")
}
`

// rootTemplateData holds the values interpolated into rootTemplate.
type rootTemplateData struct {
	Name    string
	Title   string
	BaseURL string
}

// GenerateRoot returns the Go source code for cmd/root.go in the generated
// project. It sets the root cobra.Command's Use field to name, its Short field
// to spec.Title, and registers all five required global persistent flags with
// spec.BaseURL as the default for --base-url.
func GenerateRoot(spec *model.APISpec, name string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("spec must not be nil")
	}

	funcMap := template.FuncMap{
		"goString": func(s string) string {
			return fmt.Sprintf("%q", s)
		},
	}
	tmpl, err := template.New("root").Funcs(funcMap).Parse(rootTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing root template: %w", err)
	}

	data := rootTemplateData{
		Name:    name,
		Title:   spec.Title,
		BaseURL: spec.BaseURL,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing root template: %w", err)
	}

	return strings.TrimLeft(buf.String(), "\n"), nil
}
