package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/theaiteam-dev/commandspec/internal/model"
	"gopkg.in/yaml.v3"
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

// detectFormat returns "yaml" for .yaml/.yml files, "json" otherwise.
func detectFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		return "yaml"
	}
	return "json"
}

// yamlToJSON converts YAML bytes to JSON bytes.
func yamlToJSON(data []byte) ([]byte, error) {
	var v interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}
	return json.Marshal(v)
}

// isURL returns true if the path looks like an HTTP or HTTPS URL.
func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// detectFormatFromContentType returns "yaml" or "json" based on a Content-Type header value.
func detectFormatFromContentType(ct string) string {
	ct = strings.ToLower(ct)
	// Strip parameters like charset
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	switch ct {
	case "application/x-yaml", "text/yaml", "application/yaml":
		return "yaml"
	case "application/json", "text/json":
		return "json"
	default:
		return ""
	}
}

// defaultHTTPTimeout is the fallback HTTP timeout when none is specified.
const defaultHTTPTimeout = 30 * time.Second

// SetHTTPTimeout is kept for backward compatibility but is now a no-op.
// Pass the timeout via LoadWithTimeout instead.
//
// Deprecated: Use LoadWithTimeout to avoid data races under concurrent use.
func SetHTTPTimeout(_ time.Duration) {}

// loadFromURL fetches a spec from an HTTP/HTTPS URL and returns its bytes and detected format.
func loadFromURL(rawURL string, timeout time.Duration) ([]byte, string, error) {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	httpClient := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request for %q: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "commandspec/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching spec from %q: %w", rawURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetching spec from %q: HTTP %d", rawURL, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading response body from %q: %w", rawURL, err)
	}

	// Detect format: Content-Type header first, then URL path extension.
	format := detectFormatFromContentType(resp.Header.Get("Content-Type"))
	if format == "" {
		// Parse just the path portion of the URL for extension detection.
		urlPath := rawURL
		if idx := strings.Index(urlPath, "?"); idx >= 0 {
			urlPath = urlPath[:idx]
		}
		format = detectFormat(urlPath)
	}

	return data, format, nil
}

// Load reads an OpenAPI 3.0 spec (JSON or YAML) from path (file or URL), resolves $ref references inline,
// and returns a Result containing the normalized APISpec and raw JSON bytes.
// For URL-based specs, a default 30s HTTP timeout is used. Use LoadWithTimeout
// to specify a custom timeout per-call without data races.
func Load(path string) (*Result, error) {
	return LoadWithTimeout(path, defaultHTTPTimeout)
}

// LoadWithTimeout is like Load but uses the given timeout for HTTP requests.
// It is safe to call concurrently with different timeouts.
func LoadWithTimeout(path string, timeout time.Duration) (*Result, error) {
	var data []byte
	var format string
	var err error

	if isURL(path) {
		data, format, err = loadFromURL(path, timeout)
		if err != nil {
			return nil, err
		}
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("loading spec %q: %w", path, err)
		}
		format = detectFormat(path)
	}

	if format == "yaml" {
		data, err = yamlToJSON(data)
		if err != nil {
			return nil, fmt.Errorf("converting yaml spec %q: %w", path, err)
		}
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

	return &Result{Spec: spec, RawJSON: resolved}, nil
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

// buildResources groups paths by resource name to produce Resources.
// It first detects "namespace prefix" segments (e.g. "api", "v1") that are
// shared across many paths and carry no resource-level meaning, then uses
// up to two meaningful path segments after stripping those prefixes as the
// resource name. Each HTTP method on a path becomes a Command.
func buildResources(paths map[string]map[string]json.RawMessage) []model.Resource {
	namespaces := detectNamespacePrefixes(paths)

	// Use an ordered slice to keep deterministic output.
	order := make([]string, 0)
	byName := make(map[string]*model.Resource)

	for path, methods := range paths {
		resourceName := pathToResourceName(path, namespaces)
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

// detectNamespacePrefixes identifies leading path segments that are API routing
// namespaces rather than resource names. A segment qualifies as a namespace
// prefix when it appears as the first segment in ≥25% of all paths AND leads
// to more than 3 distinct non-parameter child segments — meaning it is a
// mount-point that routes to many different resources rather than being a
// resource itself.
//
// Example: an API where every path starts with "/api/..." will detect "api" as
// a namespace and strip it, so "/api/users/{id}" becomes resource "users".
// A spec where all paths start with "/pets/..." will NOT strip "pets" because
// it only leads to a single child ("{petId}"), so it is the resource.
func detectNamespacePrefixes(paths map[string]map[string]json.RawMessage) map[string]bool {
	total := len(paths)
	if total == 0 {
		return nil
	}

	pathCount := make(map[string]int)
	nextSegs := make(map[string]map[string]bool)

	for path := range paths {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) == 0 || parts[0] == "" || strings.Contains(parts[0], "{") {
			continue
		}
		seg := parts[0]
		pathCount[seg]++
		if nextSegs[seg] == nil {
			nextSegs[seg] = make(map[string]bool)
		}
		if len(parts) > 1 && parts[1] != "" && !strings.Contains(parts[1], "{") {
			nextSegs[seg][parts[1]] = true
		}
	}

	prefixes := make(map[string]bool)
	for seg, count := range pathCount {
		if float64(count)/float64(total) >= 0.25 && len(nextSegs[seg]) > 3 {
			prefixes[seg] = true
		}
	}
	return prefixes
}

// pathToResourceName derives a resource name from a path by skipping any
// detected namespace prefixes and joining up to two meaningful (non-parameter)
// segments with a hyphen.
//
// Examples (assuming "api" is a detected namespace prefix):
//
//	"/pets/{petId}"              → "pets"          (no prefix)
//	"/api/users/{id}"            → "users"         (prefix stripped)
//	"/api/admin/broadcasts/"     → "admin-broadcasts" (prefix stripped, 2 segments)
func pathToResourceName(path string, namespaces map[string]bool) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	var meaningful []string
	namespaceSkipped := false
	for _, p := range parts {
		if p == "" || strings.Contains(p, "{") {
			break
		}
		// Skip at most one leading namespace prefix.
		if !namespaceSkipped && namespaces[p] {
			namespaceSkipped = true
			continue
		}
		meaningful = append(meaningful, p)
		if len(meaningful) == 2 {
			break
		}
	}

	if len(meaningful) == 0 {
		return "root"
	}
	return strings.Join(meaningful, "-")
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

	resolved, err := walkResolve(rootMap, rootMap, make(map[string]bool))
	if err != nil {
		return nil, err
	}

	return json.Marshal(resolved)
}

// walkResolve recursively walks the node, replacing any {"$ref": "#/..."} objects
// with the referenced value looked up in root. visiting tracks refs currently on
// the call stack to break circular references.
func walkResolve(node interface{}, root map[string]interface{}, visiting map[string]bool) (interface{}, error) {
	switch v := node.(type) {
	case map[string]interface{}:
		if ref, ok := v["$ref"]; ok {
			refStr, ok := ref.(string)
			if !ok {
				return node, nil
			}
			// Break circular references: return a placeholder instead of recursing.
			if visiting[refStr] {
				return map[string]interface{}{"$ref": refStr}, nil
			}
			resolved, err := resolveRef(refStr, root)
			if err != nil {
				return nil, err
			}
			// Recursively resolve refs within the resolved value.
			visiting[refStr] = true
			result, err := walkResolve(resolved, root, visiting)
			delete(visiting, refStr)
			return result, err
		}
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			resolved, err := walkResolve(val, root, visiting)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			resolved, err := walkResolve(item, root, visiting)
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
