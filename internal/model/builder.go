package model

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// rawOperation is a minimal representation of an OpenAPI operation.
type rawOperation struct {
	OperationID string          `json:"operationId"`
	Summary     string          `json:"summary"`
	Parameters  json.RawMessage `json:"parameters"`
	RequestBody json.RawMessage `json:"requestBody"`
}

// RawJSONProvider is implemented by types that expose the raw spec JSON bytes.
type RawJSONProvider interface {
	GetRawJSON() []byte
}

// Build parses the raw OpenAPI spec from result and returns a fully populated
// slice of Resources with Commands, including operationId overrides and
// collision resolution.
func Build(result SpecProvider) ([]Resource, error) {
	rp, ok := result.(RawJSONProvider)
	if !ok {
		return nil, fmt.Errorf("result does not implement RawJSONProvider")
	}
	data := rp.GetRawJSON()
	if len(data) == 0 {
		return nil, fmt.Errorf("raw JSON is empty")
	}
	return buildFromRaw(data)
}

// buildFromRaw parses the OpenAPI spec JSON and builds Resources with full
// operationId support and collision resolution.
func buildFromRaw(data []byte) ([]Resource, error) {
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing spec JSON: %w", err)
	}

	namespaces := detectNamespacePrefixes(doc.Paths)

	// Collect per-resource command entries in insertion order.
	order := make([]string, 0)
	byName := make(map[string][]*cmdEntry)

	for path, methods := range doc.Paths {
		resourceName := pathResourceName(path, namespaces)
		if _, exists := byName[resourceName]; !exists {
			order = append(order, resourceName)
			byName[resourceName] = nil
		}

		for methodRaw, opBytes := range methods {
			method := strings.ToUpper(methodRaw)

			var op rawOperation
			if err := json.Unmarshal(opBytes, &op); err != nil {
				continue
			}

			verb := httpVerbForMethod(method, path)
			name := verb
			if op.OperationID != "" {
				name = sanitizeCommandName(op.OperationID)
			}

			cmd := Command{
				Name:        name,
				HTTPMethod:  method,
				Path:        path,
				Description: op.Summary,
			}

			// Extract path/query params.
			if len(op.Parameters) > 0 {
				var parameters []interface{}
				if err := json.Unmarshal(op.Parameters, &parameters); err == nil {
					args, queryFlags, err := ExtractParams(nil, parameters)
					if err != nil {
						log.Printf("ExtractParams for %s %s: %v", method, path, err)
					} else {
						cmd.Args = args
						cmd.Flags = append(cmd.Flags, queryFlags...)
					}
				}
			}

			// Extract body flags.
			if len(op.RequestBody) > 0 {
				var requestBody map[string]interface{}
				if err := json.Unmarshal(op.RequestBody, &requestBody); err == nil {
					bodyFlags, err := ExtractBodyFlags(requestBody)
					if err != nil {
						log.Printf("ExtractBodyFlags for %s %s: %v", method, path, err)
					} else {
						cmd.Flags = append(cmd.Flags, bodyFlags...)
					}
				}
			}

			byName[resourceName] = append(byName[resourceName], &cmdEntry{
				cmd:    cmd,
				verb:   verb,
				method: method,
			})
		}
	}

	resources := make([]Resource, 0, len(order))
	for _, name := range order {
		entries := byName[name]
		cmds := resolveCollisions(entries)
		resources = append(resources, Resource{Name: name, Commands: cmds})
	}
	return resources, nil
}

type cmdEntry struct {
	cmd    Command
	verb   string
	method string
}

// resolveCollisions detects any two commands in the same resource with the same
// Name and appends "-" + lowercase(method) to all colliding commands.
func resolveCollisions(entries []*cmdEntry) []Command {
	// Count occurrences of every command name, regardless of how it was derived.
	nameCount := make(map[string]int)
	for _, e := range entries {
		nameCount[e.cmd.Name]++
	}

	cmds := make([]Command, 0, len(entries))
	for _, e := range entries {
		cmd := e.cmd
		if nameCount[e.cmd.Name] > 1 {
			cmd.Name = e.cmd.Name + "-" + strings.ToLower(e.method)
		}
		cmds = append(cmds, cmd)
	}
	return cmds
}

// detectNamespacePrefixes identifies leading path segments that are API routing
// namespaces (e.g. "api", "v1") rather than resource names. A segment is a
// namespace when it appears as the first segment in ≥25% of all paths AND
// leads to more than 3 distinct non-parameter child segments.
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

// pathResourceName derives a resource name by skipping any detected namespace
// prefix and joining up to two meaningful (non-parameter) path segments with "-".
//
// Examples (assuming "api" is a detected namespace):
//
//	"/pets/{id}"              → "pets"
//	"/api/users/{id}"         → "users"
//	"/api/admin/broadcasts/"  → "admin-broadcasts"
func pathResourceName(path string, namespaces map[string]bool) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	var meaningful []string
	namespaceSkipped := false
	for _, p := range parts {
		if p == "" || strings.Contains(p, "{") {
			break
		}
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

// invalidCommandCharRE matches any character that is not an ASCII letter,
// digit, or hyphen — characters that would produce an invalid CLI command name.
var invalidCommandCharRE = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

// consecutiveHyphenRE matches two or more consecutive hyphens.
var consecutiveHyphenRE = regexp.MustCompile(`-{2,}`)

// sanitizeCommandName converts an operationId into a valid CLI command name by
// replacing any characters that are not alphanumeric or hyphen with "-",
// collapsing consecutive hyphens, and trimming leading/trailing hyphens.
// Case is preserved so that camelCase operationIds remain readable.
func sanitizeCommandName(operationID string) string {
	name := invalidCommandCharRE.ReplaceAllString(operationID, "-")
	name = consecutiveHyphenRE.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}

// httpVerbForMethod maps an HTTP method+path to a CLI verb.
func httpVerbForMethod(method, path string) string {
	hasParam := strings.Contains(path, "{")
	switch method {
	case "GET":
		if hasParam {
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
