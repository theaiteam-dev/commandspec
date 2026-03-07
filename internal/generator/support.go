package generator

import (
	"fmt"
	"strings"

	"github.com/queso/swagger-jack/internal/model"
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
const outputTemplate = `package internal

import (
	"encoding/json"
	"fmt"
	"os"
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
// variable prefix and the config-file directory path.
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
