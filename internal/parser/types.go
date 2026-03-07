// Package parser provides types for the OpenAPI spec parsing pipeline.
package parser

import "github.com/queso/swagger-jack/internal/model"

// Result is the output of the loader/parser pipeline. It contains the
// normalized APISpec and the raw JSON bytes of the original spec, which
// may be useful for downstream tooling or debugging.
type Result struct {
	// Spec is the normalized internal model parsed from the OpenAPI spec.
	Spec *model.APISpec `json:"spec"`
	// RawJSON contains the raw JSON bytes of the original spec (after any
	// YAML-to-JSON conversion).
	RawJSON []byte `json:"raw_json,omitempty"`
}

// GetSpec returns the parsed APISpec, implementing model.SpecProvider.
func (r *Result) GetSpec() *model.APISpec {
	if r == nil {
		return nil
	}
	return r.Spec
}

// GetRawJSON returns the raw JSON bytes of the original spec.
func (r *Result) GetRawJSON() []byte {
	if r == nil {
		return nil
	}
	return r.RawJSON
}
