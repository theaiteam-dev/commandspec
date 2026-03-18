package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/theaiteam-dev/commandspec/internal/model"
)

// configTemplate is the Go source template for the generated project's config loader.
const configTemplate = `package internal

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds runtime configuration for the CLI.
type Config struct {
	Token   string ` + "`yaml:\"token\"`" + `
	BaseURL string ` + "`yaml:\"base_url\"`" + `
}

// Load reads configuration with the following precedence (highest to lowest):
//  1. Environment variable {{.EnvPrefix}}_TOKEN
//  2. Config file at ~/.config/<cliName>/config.yaml
func Load(cliName string) (*Config, error) {
	cfg := &Config{}

	// Attempt to load from the config file first (lowest precedence).
	configDir, err := os.UserConfigDir()
	if err == nil {
		configPath := filepath.Join(configDir, cliName, "config.yaml")
		data, readErr := os.ReadFile(configPath)
		if readErr == nil {
			_ = yaml.Unmarshal(data, cfg)
		}
	}

	// Environment variable overrides the config file.
	envKey := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(cliName)) + "_TOKEN"
	if token := os.Getenv(envKey); token != "" {
		cfg.Token = token
	}

	return cfg, nil
}
`

// outputTemplate is the Go source template for the generated project's output helpers.
const outputTemplate = `package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/olekukonko/tablewriter"
)

// Print writes data to stdout. When jsonMode is true the raw JSON is printed
// compactly; otherwise it is pretty-printed with indentation.
func Print(data interface{}, jsonMode bool) error {
	var encoded []byte
	var err error

	if jsonMode {
		encoded, err = json.Marshal(data)
	} else {
		encoded, err = json.MarshalIndent(data, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("marshalling output: %w", err)
	}

	_, err = fmt.Fprintln(os.Stdout, string(encoded))
	return err
}

// PrintTable renders JSON data as a human-readable table.
// Arrays of objects are rendered as columnar tables with one row per object.
// Single objects are rendered as two-column key-value tables.
// Nested objects within rows and arrays of non-objects fall back to
// pretty-printed JSON output.
func PrintTable(data []byte, noColor bool) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		// Fallback: print raw
		_, err = fmt.Fprintln(os.Stdout, string(data))
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	if noColor {
		table.SetBorder(true)
	}

	switch v := raw.(type) {
	case []interface{}:
		if len(v) == 0 {
			fmt.Fprintln(os.Stdout, "(no results)")
			return nil
		}
		// Collect headers from the first element.
		first, ok := v[0].(map[string]interface{})
		if !ok {
			// Not an array of objects; fall back to JSON.
			encoded, _ := json.MarshalIndent(raw, "", "  ")
			fmt.Fprintln(os.Stdout, string(encoded))
			return nil
		}
		headers := make([]string, 0, len(first))
		for k := range first {
			headers = append(headers, k)
		}
		sort.Strings(headers)
		table.SetHeader(headers)
		for _, item := range v {
			row, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			cols := make([]string, len(headers))
			for i, h := range headers {
				cols[i] = fmt.Sprintf("%v", row[h])
			}
			table.Append(cols)
		}
		table.Render()

	case map[string]interface{}:
		table.SetHeader([]string{"Key", "Value"})
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			table.Append([]string{k, fmt.Sprintf("%v", v[k])})
		}
		table.Render()

	default:
		fmt.Fprintln(os.Stdout, string(data))
	}

	return nil
}
`

// errorsTemplate is the Go source template for the generated project's error helpers.
const errorsTemplate = `package internal

import "fmt"

// HTTPError represents an unexpected HTTP response from the API.
type HTTPError struct {
	StatusCode int
	Body       string
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// FormatHTTPError returns an error that includes the HTTP StatusCode and body.
func FormatHTTPError(statusCode int, body string) error {
	return fmt.Errorf("HTTP %d: %s", statusCode, body)
}
`

// GenerateConfig returns Go source code for the generated project's
// internal/config.go file. The CLI name is used to derive the environment
// variable prefix and the config-file directory path. All security scheme
// env vars are documented in the generated file.
func GenerateConfig(spec *model.APISpec, name string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("spec must not be nil")
	}
	if name == "" {
		return "", fmt.Errorf("name must not be empty")
	}

	// Substitute the template placeholder comment with the actual env prefix so
	// the generated file is self-documenting.
	envPrefix := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(name))
	src := strings.ReplaceAll(configTemplate, "{{.EnvPrefix}}", envPrefix)
	src = strings.TrimLeft(src, "\n")

	// Append documentation comments for all detected security scheme env vars
	// so developers can easily discover the available auth environment variables.
	if spec != nil && len(spec.SecuritySchemes) > 0 {
		// Sort for deterministic output.
		schemeNames := make([]string, 0, len(spec.SecuritySchemes))
		for sn := range spec.SecuritySchemes {
			schemeNames = append(schemeNames, sn)
		}
		sort.Strings(schemeNames)

		var envVars []string
		seen := map[string]bool{}
		for _, sn := range schemeNames {
			scheme := spec.SecuritySchemes[sn]
			if scheme.EnvVar != "" && !seen[scheme.EnvVar] {
				seen[scheme.EnvVar] = true
				envVars = append(envVars, scheme.EnvVar)
			}
		}

		if len(envVars) > 0 {
			var comment strings.Builder
			comment.WriteString("\n// Auth environment variables detected from the OpenAPI spec:\n")
			for _, ev := range envVars {
				comment.WriteString("//   " + ev + "\n")
			}
			src += comment.String()
		}
	}

	return src, nil
}

// GenerateOutput returns Go source code for the generated project's
// internal/output.go file.
func GenerateOutput() (string, error) {
	return strings.TrimLeft(outputTemplate, "\n"), nil
}

// GenerateErrors returns Go source code for the generated project's
// internal/errors.go file.
func GenerateErrors() (string, error) {
	return strings.TrimLeft(errorsTemplate, "\n"), nil
}
