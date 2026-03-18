// Package generator produces Go source files for the generated CLI project.
package generator

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/theaiteam-dev/commandspec/internal/model"
)

// clientTemplate is the Go source template for the generated HTTP client.
const clientTemplate = `package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is the default API base URL embedded from the OpenAPI spec.
const DefaultBaseURL = {{goString .BaseURL}}
{{range .APIKeySchemes}}
// {{.EnvVar}} is the API key credential for the {{.HeaderName}} header.
var {{envVarToIdent .EnvVar}} = os.Getenv({{goString .EnvVar}})
{{end}}

// Client holds the configuration for making authenticated HTTP requests.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient constructs a Client with the given baseURL and token.
// When baseURL is empty, DefaultBaseURL is used.
func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// execute injects authentication headers into req, sends it, reads the response
// body, and returns an error for non-2xx status codes. Both Do and DoMultipart
// delegate to this method so auth injection is defined exactly once.
func (c *Client) execute(req *http.Request) ([]byte, error) {
	// Inject authentication credentials based on security schemes.
{{- if and .BearerSchemes .BasicSchemes}}
	// Bearer takes precedence over Basic when both schemes are present.
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
{{- else if .BearerSchemes}}
	// Bearer token auth.
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
{{- else if .BasicSchemes}}
	// Basic auth (base64-encoded credentials).
	if c.Token != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(c.Token))
		req.Header.Set("Authorization", "Basic "+encoded)
	}
{{- end}}
{{- range .APIKeySchemes}}
	// API key auth via {{.HeaderName}}.
	if apiKey := {{envVarToIdent .EnvVar}}; apiKey != "" {
		req.Header.Set({{goString .HeaderName}}, apiKey)
	}
{{- end}}
{{- if .NoSchemes}}
	// No security schemes defined; inject Bearer token if provided.
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
{{- end}}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// Return a descriptive error for non-2xx responses.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyStr := string(responseBody)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "... (truncated)"
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, bodyStr)
	}

	return responseBody, nil
}

// Do executes an HTTP request against the API.
//
// method is the HTTP verb (GET, POST, etc.).
// path is the URL path template (e.g., "/users/{userId}").
// pathParams maps placeholder names to their runtime values for path interpolation.
// queryParams maps query parameter names to values appended to the URL.
// body is an optional request body; pass nil for requests without a body.
//
// Path parameter substitution uses strings.NewReplacer to replace {param}
// placeholders with the corresponding values from pathParams.
func (c *Client) Do(method, path string, pathParams map[string]string, queryParams map[string]string, body interface{}) ([]byte, error) {
	// Interpolate {param} placeholders in the path template.
	pairs := make([]string, 0, len(pathParams)*2)
	for key, value := range pathParams {
		pairs = append(pairs, "{"+key+"}", value)
	}
	interpolatedPath := strings.NewReplacer(pairs...).Replace(path)

	requestURL := strings.TrimRight(c.BaseURL, "/") + interpolatedPath

	// Append query parameters.
	if len(queryParams) > 0 {
		separator := "?"
		for key, value := range queryParams {
			requestURL += separator + key + "=" + url.QueryEscape(value)
			separator = "&"
		}
	}

	// Encode the request body as JSON when provided.
	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.execute(req)
}

// DoMultipart executes a multipart/form-data HTTP request against the API.
//
// method is the HTTP verb (typically POST or PUT).
// path is the URL path template (e.g., "/documents/upload").
// pathParams maps placeholder names to their runtime values for path interpolation.
// queryParams maps query parameter names to values appended to the URL.
// body is an io.Reader containing the pre-built multipart body (built by the caller
// using mime/multipart.Writer).
// contentType is the full Content-Type header value including the boundary parameter,
// obtained from multipart.Writer.FormDataContentType().
func (c *Client) DoMultipart(method, path string, pathParams map[string]string, queryParams map[string]string, body io.Reader, contentType string) ([]byte, error) {
	// Interpolate {param} placeholders in the path template.
	pairs := make([]string, 0, len(pathParams)*2)
	for key, value := range pathParams {
		pairs = append(pairs, "{"+key+"}", value)
	}
	interpolatedPath := strings.NewReplacer(pairs...).Replace(path)

	requestURL := strings.TrimRight(c.BaseURL, "/") + interpolatedPath

	// Append query parameters.
	if len(queryParams) > 0 {
		separator := "?"
		for key, value := range queryParams {
			requestURL += separator + key + "=" + url.QueryEscape(value)
			separator = "&"
		}
	}

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set Content-Type to multipart/form-data with the writer's boundary.
	req.Header.Set("Content-Type", contentType)

	return c.execute(req)
}
`

// apiKeySchemeData holds data for one API key scheme in the template.
type apiKeySchemeData struct {
	HeaderName string
	EnvVar     string
}

// clientTemplateData holds the values interpolated into clientTemplate.
type clientTemplateData struct {
	BaseURL       string
	BearerSchemes []model.SecurityScheme
	BasicSchemes  []model.SecurityScheme
	APIKeySchemes []apiKeySchemeData
	NoSchemes     bool
}

// envVarToIdent converts an env var name like "MYAPI_API_KEY" to a Go identifier
// safe for use as a variable name. We use a lowercase camelCase conversion.
func envVarToIdent(envVar string) string {
	// Base64-encode the env var reference as a var name: just use the raw call
	// to os.Getenv in the template instead; this helper is for embedding calls.
	// We keep it simple: lowercase the env var name and prepend "cred".
	parts := strings.Split(envVar, "_")
	if len(parts) == 0 {
		return "credToken"
	}
	result := strings.ToLower(parts[0])
	for _, p := range parts[1:] {
		if len(p) == 0 {
			continue
		}
		result += strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return result
}

// GenerateClient returns the Go source code for the generated project's HTTP
// client (internal/client/client.go). It embeds spec.BaseURL as DefaultBaseURL
// and generates auth injection for all security schemes in the spec.
func GenerateClient(spec *model.APISpec) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("spec must not be nil")
	}

	data, err := buildClientTemplateData(spec)
	if err != nil {
		return "", err
	}

	funcMap := template.FuncMap{
		"goString": func(s string) string {
			return fmt.Sprintf("%q", s)
		},
		"envVarToIdent": envVarToIdent,
	}
	tmpl, err := template.New("client").Funcs(funcMap).Parse(clientTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing client template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing client template: %w", err)
	}

	src := strings.TrimLeft(buf.String(), "\n")

	// Remove the unused base64 import if no Basic auth schemes are present.
	// The template always imports it; strip the import if unused to keep the
	// generated file compilable.
	if len(data.BasicSchemes) == 0 {
		src = removeUnusedBase64Import(src)
	}

	// Remove the unused os import if no API key schemes are present.
	// os.Getenv is only emitted in API key scheme var declarations.
	if len(data.APIKeySchemes) == 0 {
		src = removeUnusedOsImport(src)
	}

	return src, nil
}

// buildClientTemplateData classifies the spec's security schemes into bearer,
// basic, and API key buckets for the template. Returns an error if any API key
// scheme has an empty EnvVar or empty HeaderName.
func buildClientTemplateData(spec *model.APISpec) (clientTemplateData, error) {
	data := clientTemplateData{
		BaseURL: spec.BaseURL,
	}

	if len(spec.SecuritySchemes) == 0 {
		data.NoSchemes = true
		return data, nil
	}

	// Sort scheme names for deterministic output.
	names := make([]string, 0, len(spec.SecuritySchemes))
	for name := range spec.SecuritySchemes {
		names = append(names, name)
	}
	sort.Strings(names)

	// Track emitted Go identifiers to deduplicate API key var declarations.
	seenAPIKeyIdents := map[string]bool{}

	for _, name := range names {
		scheme := spec.SecuritySchemes[name]
		switch scheme.Type {
		case model.SecuritySchemeBearer:
			data.BearerSchemes = append(data.BearerSchemes, scheme)
		case model.SecuritySchemeBasic:
			data.BasicSchemes = append(data.BasicSchemes, scheme)
		case model.SecuritySchemeAPIKey:
			if scheme.EnvVar == "" {
				return clientTemplateData{}, fmt.Errorf("security scheme %q has empty EnvVar: cannot generate auth code", name)
			}
			if scheme.HeaderName == "" {
				return clientTemplateData{}, fmt.Errorf("security scheme %q has empty HeaderName: cannot generate auth code", name)
			}
			ident := envVarToIdent(scheme.EnvVar)
			if seenAPIKeyIdents[ident] {
				// Skip duplicate — same Go var already declared for this EnvVar.
				continue
			}
			seenAPIKeyIdents[ident] = true
			data.APIKeySchemes = append(data.APIKeySchemes, apiKeySchemeData{
				HeaderName: scheme.HeaderName,
				EnvVar:     scheme.EnvVar,
			})
		}
	}

	return data, nil
}

// removeUnusedBase64Import removes the "encoding/base64" import line from
// generated source when no Basic auth schemes require it.
func removeUnusedBase64Import(src string) string {
	// Remove standalone import line.
	src = strings.ReplaceAll(src, "\t\"encoding/base64\"\n", "")
	return src
}

// removeUnusedOsImport removes the "os" import line from generated source when
// no API key schemes are present (os.Getenv is only used in API key var declarations).
func removeUnusedOsImport(src string) string {
	src = strings.ReplaceAll(src, "\t\"os\"\n", "")
	return src
}

// Ensure base64 package is referenced so the import isn't flagged by the
// Go compiler during generator compilation (it's used in the template string,
// not in this file's Go code directly).
var _ = base64.StdEncoding
