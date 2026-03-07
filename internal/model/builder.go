package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// rawOperation is a minimal representation of an OpenAPI operation.
type rawOperation struct {
	OperationID string `json:"operationId"`
	Summary     string `json:"summary"`
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

	// Collect per-resource command entries in insertion order.
	order := make([]string, 0)
	byName := make(map[string][]*cmdEntry)

	for path, methods := range doc.Paths {
		resourceName := firstPathSegment(path)
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

// firstPathSegment extracts the resource name from the first path segment.
func firstPathSegment(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "root"
	}
	return parts[0]
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
