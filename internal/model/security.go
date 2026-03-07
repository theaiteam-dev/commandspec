package model

import (
	"fmt"
	"strings"
)

// SpecProvider is implemented by any type that exposes a parsed APISpec.
// This interface avoids an import cycle between the model and parser packages.
type SpecProvider interface {
	GetSpec() *APISpec
}

// ExtractSecuritySchemes reads the security schemes already parsed by the loader
// and enriches them with environment variable names derived from the CLI name.
// The result parameter must implement SpecProvider (e.g. *parser.Result).
func ExtractSecuritySchemes(result SpecProvider, cliName string) (map[string]SecurityScheme, error) {
	if cliName == "" {
		return nil, fmt.Errorf("cliName must not be empty")
	}

	spec := result.GetSpec()
	if spec == nil {
		return map[string]SecurityScheme{}, nil
	}

	if len(spec.SecuritySchemes) == 0 {
		return map[string]SecurityScheme{}, nil
	}

	envPrefix := strings.ToUpper(strings.ReplaceAll(cliName, "-", "_"))

	schemes := make(map[string]SecurityScheme, len(spec.SecuritySchemes))
	for name, s := range spec.SecuritySchemes {
		enriched := SecurityScheme{
			Type:       s.Type,
			HeaderName: s.HeaderName,
		}
		switch s.Type {
		case SecuritySchemeBearer, SecuritySchemeBasic:
			enriched.EnvVar = envPrefix + "_TOKEN"
		case SecuritySchemeAPIKey:
			enriched.EnvVar = envPrefix + "_API_KEY"
		default:
			// Skip unrecognized scheme types rather than including them with
			// an empty EnvVar, which would produce unusable entries.
			continue
		}
		schemes[name] = enriched
	}

	return schemes, nil
}
