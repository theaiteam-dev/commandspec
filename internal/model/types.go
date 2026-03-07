// Package model defines the normalized internal model types used throughout
// the swagger-jack pipeline. These types represent the parsed OpenAPI spec
// as a CLI command tree.
package model

// FlagType represents the Go type for a CLI flag.
type FlagType string

const (
	// FlagTypeString is the string flag type.
	FlagTypeString FlagType = "string"
	// FlagTypeInt is the integer flag type.
	FlagTypeInt FlagType = "int"
	// FlagTypeBool is the boolean flag type.
	FlagTypeBool FlagType = "bool"
	// FlagTypeStringSlice is the string slice flag type.
	FlagTypeStringSlice FlagType = "[]string"
)

// FlagSource indicates where the flag value maps to in the HTTP request.
type FlagSource string

const (
	// FlagSourceQuery maps the flag to a URL query parameter.
	FlagSourceQuery FlagSource = "query"
	// FlagSourceBody maps the flag to the request body.
	FlagSourceBody FlagSource = "body"
	// FlagSourceHeader maps the flag to an HTTP request header.
	FlagSourceHeader FlagSource = "header"
)

// SecuritySchemeType identifies the auth mechanism.
type SecuritySchemeType string

const (
	// SecuritySchemeBearer is the Bearer token auth scheme.
	SecuritySchemeBearer SecuritySchemeType = "bearer"
	// SecuritySchemeAPIKey is the API key auth scheme.
	SecuritySchemeAPIKey SecuritySchemeType = "apikey"
	// SecuritySchemeBasic is the HTTP Basic auth scheme.
	SecuritySchemeBasic SecuritySchemeType = "basic"
)

// Arg represents a positional CLI argument derived from an OpenAPI path parameter.
type Arg struct {
	// Name is the argument name (e.g., "user-id" from "{userId}").
	Name string `json:"name"`
	// Description is the human-readable description of the argument.
	Description string `json:"description,omitempty"`
	// Required indicates whether the argument is required.
	Required bool `json:"required"`
}

// Flag represents a CLI flag derived from an OpenAPI query, body, or header parameter.
type Flag struct {
	// Name is the flag name (e.g., "limit", "address.city").
	Name string `json:"name"`
	// Type is the Go type for this flag.
	Type FlagType `json:"type"`
	// Required indicates whether the flag must be provided.
	Required bool `json:"required"`
	// Default is the default value for the flag (as a string).
	Default string `json:"default,omitempty"`
	// Description is the human-readable description of the flag.
	Description string `json:"description,omitempty"`
	// Source indicates where this flag maps to in the HTTP request.
	Source FlagSource `json:"source"`
}

// SchemaField represents a field in a flat request body schema.
type SchemaField struct {
	// Name is the field name.
	Name string `json:"name"`
	// Type is the Go type for this field.
	Type FlagType `json:"type"`
	// Required indicates whether the field is required.
	Required bool `json:"required"`
	// Description is the human-readable description of the field.
	Description string `json:"description,omitempty"`
}

// RequestBody represents the request body for a Command, supporting
// flat object schemas that map to individual flags.
type RequestBody struct {
	// ContentType is the MIME type of the request body (e.g., "application/json").
	ContentType string `json:"content_type"`
	// Required indicates whether a request body must be provided.
	Required bool `json:"required"`
	// Schema contains the flat-object field definitions for flag generation.
	// For nested objects, dot notation is used (e.g., "address.city").
	Schema []SchemaField `json:"schema,omitempty"`
}

// Response represents the expected response shape for a Command.
type Response struct {
	// ContentType is the MIME type of the response (e.g., "application/json").
	ContentType string `json:"content_type,omitempty"`
	// Description is the human-readable description of the response.
	Description string `json:"description,omitempty"`
}

// Command represents a single CLI command derived from an OpenAPI operation.
type Command struct {
	// Name is the CLI verb (e.g., "list", "get", "create", "update", "delete").
	Name string `json:"name"`
	// HTTPMethod is the HTTP method (e.g., "GET", "POST").
	HTTPMethod string `json:"http_method"`
	// Path is the URL path template (e.g., "/users/{userId}").
	Path string `json:"path"`
	// Description is the human-readable description of the command.
	Description string `json:"description,omitempty"`
	// Args are positional arguments derived from path parameters.
	Args []Arg `json:"args,omitempty"`
	// Flags are CLI flags derived from query, body, and header parameters.
	Flags []Flag `json:"flags,omitempty"`
	// RequestBody describes the request body, if any.
	RequestBody *RequestBody `json:"request_body,omitempty"`
	// Response describes the expected response.
	Response *Response `json:"response,omitempty"`
}

// Resource represents a group of related Commands under a single CLI subcommand.
type Resource struct {
	// Name is the resource name used as the CLI subcommand (e.g., "users").
	Name string `json:"name"`
	// Description is the human-readable description of the resource group.
	Description string `json:"description,omitempty"`
	// Commands are the operations available for this resource.
	Commands []Command `json:"commands,omitempty"`
}

// SecurityScheme describes an authentication mechanism from the OpenAPI spec.
type SecurityScheme struct {
	// Type is the auth mechanism type.
	Type SecuritySchemeType `json:"type"`
	// HeaderName is the HTTP header name for API key auth (e.g., "X-API-Key").
	HeaderName string `json:"header_name,omitempty"`
	// EnvVar is the environment variable name used to source the credential.
	EnvVar string `json:"env_var,omitempty"`
}

// APISpec is the top-level normalized representation of an OpenAPI spec,
// ready for use by the model builder and code generator.
type APISpec struct {
	// Title is the API title from the spec info object.
	Title string `json:"title"`
	// Version is the API version string from the spec info object.
	Version string `json:"version"`
	// Description is the API description from the spec info object.
	Description string `json:"description,omitempty"`
	// BaseURL is the resolved server URL for HTTP requests.
	BaseURL string `json:"base_url,omitempty"`
	// Resources is the list of resource groups derived from the API paths.
	Resources []Resource `json:"resources,omitempty"`
	// SecuritySchemes is the map of security scheme names to their definitions.
	SecuritySchemes map[string]SecurityScheme `json:"security_schemes,omitempty"`
}
