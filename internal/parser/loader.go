package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/queso/swagger-jack/internal/model"
)

// rawSpec is the intermediate representation of an OpenAPI 3.0 spec used during loading.
type rawSpec struct {
	OpenAPI string `json:"openapi"`
	Info    struct {
		Title       string `json:"title"`
		Version     string `json:"version"`
		Description string `json:"description"`
	} `json:"info"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Components struct {
		Schemas         map[string]json.RawMessage   `json:"schemas"`
		SecuritySchemes map[string]rawSecurityScheme `json:"securitySchemes"`
	} `json:"components"`
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

type rawSecurityScheme struct {
	Type   string `json:"type"`
	Scheme string `json:"scheme"`
	Name   string `json:"name"`
	In     string `json:"in"`
}

// Load reads an OpenAPI 3.0 JSON spec from path, resolves $ref references inline,
// and returns a Result containing the normalized APISpec and raw JSON bytes.
func Load(path string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading spec %q: %w", path, err)
	}

	// Resolve $refs before unmarshalling into typed structs.
	resolved, err := resolveRefs(data)
	if err != nil {
		return nil, fmt.Errorf("resolving $refs in %q: %w", path, err)
	}

	var raw rawSpec
	if err := json.Unmarshal(resolved, &raw); err != nil {
		return nil, fmt.Errorf("parsing spec %q: %w", path, err)
	}

	spec := &model.APISpec{
		Title:       raw.Info.Title,
		Version:     raw.Info.Version,
		Description: raw.Info.Description,
	}

	if len(raw.Servers) > 0 {
		spec.BaseURL = raw.Servers[0].URL
	}

	// Build security schemes.
	if len(raw.Components.SecuritySchemes) > 0 {
		spec.SecuritySchemes = make(map[string]model.SecurityScheme, len(raw.Components.SecuritySchemes))
		for name, ss := range raw.Components.SecuritySchemes {
			scheme := resolveSecurityScheme(ss)
			spec.SecuritySchemes[name] = scheme
		}
	}

	// Build resources from paths (group by first path segment).
	spec.Resources = buildResources(raw.Paths)

	return &Result{Spec: spec, RawJSON: data}, nil
}

// resolveSecurityScheme maps a raw OpenAPI security scheme to model.SecurityScheme.
func resolveSecurityScheme(ss rawSecurityScheme) model.SecurityScheme {
	t := strings.ToLower(ss.Type)
	switch t {
	case "http":
		if strings.ToLower(ss.Scheme) == "bearer" {
			return model.SecurityScheme{Type: model.SecuritySchemeBearer}
		}
		return model.SecurityScheme{Type: model.SecuritySchemeBasic}
	case "apikey":
		return model.SecurityScheme{
			Type:       model.SecuritySchemeAPIKey,
			HeaderName: ss.Name,
		}
	default:
		return model.SecurityScheme{Type: model.SecuritySchemeType(t)}
	}
}

// buildResources groups paths by their first segment to produce Resources.
// Each unique first segment becomes a resource; each HTTP method becomes a Command.
func buildResources(paths map[string]map[string]json.RawMessage) []model.Resource {
	// Use an ordered slice to keep deterministic output.
	order := make([]string, 0)
	byName := make(map[string]*model.Resource)

	for path, methods := range paths {
		resourceName := pathToResourceName(path)
		if _, exists := byName[resourceName]; !exists {
			byName[resourceName] = &model.Resource{Name: resourceName}
			order = append(order, resourceName)
		}
		res := byName[resourceName]

		for method := range methods {
			method = strings.ToUpper(method)
			cmdName := httpMethodToVerb(method, path)
			cmd := model.Command{
				Name:       cmdName,
				HTTPMethod: method,
				Path:       path,
			}
			res.Commands = append(res.Commands, cmd)
		}
	}

	resources := make([]model.Resource, 0, len(order))
	for _, name := range order {
		resources = append(resources, *byName[name])
	}
	return resources
}

// pathToResourceName extracts the resource name from the first path segment.
// "/pets/{petId}" → "pets"
func pathToResourceName(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "root"
	}
	return parts[0]
}

// httpMethodToVerb maps an HTTP method to a CLI verb.
func httpMethodToVerb(method, path string) string {
	hasPathParam := strings.Contains(path, "{")
	switch method {
	case "GET":
		if hasPathParam {
			return "get"
		}
		return "list"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return strings.ToLower(method)
	}
}

// resolveRefs takes raw JSON bytes and resolves all "#/..." $ref pointers inline.
// It performs a simple text-level replacement by first unmarshalling to a generic
// map, then walking and replacing $ref nodes.
func resolveRefs(data []byte) ([]byte, error) {
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	rootMap, ok := root.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("spec root must be a JSON object")
	}

	resolved, err := walkResolve(rootMap, rootMap)
	if err != nil {
		return nil, err
	}

	return json.Marshal(resolved)
}

// walkResolve recursively walks the node, replacing any {"$ref": "#/..."} objects
// with the referenced value looked up in root.
func walkResolve(node interface{}, root map[string]interface{}) (interface{}, error) {
	switch v := node.(type) {
	case map[string]interface{}:
		if ref, ok := v["$ref"]; ok {
			refStr, ok := ref.(string)
			if !ok {
				return node, nil
			}
			resolved, err := resolveRef(refStr, root)
			if err != nil {
				return nil, err
			}
			// Recursively resolve refs within the resolved value.
			return walkResolve(resolved, root)
		}
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			resolved, err := walkResolve(val, root)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			resolved, err := walkResolve(item, root)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	default:
		return node, nil
	}
}

// resolveRef looks up a JSON pointer like "#/components/schemas/Pet" in root.
func resolveRef(ref string, root map[string]interface{}) (interface{}, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("unsupported $ref format: %q (only local #/ refs supported)", ref)
	}
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	var current interface{} = root
	for _, part := range parts {
		// Unescape JSON Pointer per RFC 6901.
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("$ref %q: expected object at segment %q", ref, part)
		}
		val, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("$ref %q: key %q not found", ref, part)
		}
		current = val
	}
	return current, nil
}
