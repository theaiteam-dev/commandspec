package model

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ExtractParams parses an OpenAPI operation's parameter list into positional
// args (path params) and flags (query params). The operation map is accepted
// for future use but is not required — pass nil if you have no operation object.
func ExtractParams(_ map[string]interface{}, allParams []interface{}) ([]Arg, []Flag, error) {
	args := []Arg{}
	flags := []Flag{}

	for _, raw := range allParams {
		param, ok := raw.(map[string]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("parameter is not a map: %T", raw)
		}

		location, _ := param["in"].(string)

		switch location {
		case "path":
			arg, err := buildArg(param)
			if err != nil {
				return nil, nil, err
			}
			args = append(args, arg)

		case "query":
			flag, err := buildFlag(param)
			if err != nil {
				return nil, nil, err
			}
			flags = append(flags, flag)
		}
	}

	return args, flags, nil
}

// buildArg converts an OpenAPI path parameter map into an Arg.
func buildArg(param map[string]interface{}) (Arg, error) {
	name, _ := param["name"].(string)
	description, _ := param["description"].(string)

	return Arg{
		Name:        camelToKebab(name),
		Description: description,
		Required:    true,
	}, nil
}

// buildFlag converts an OpenAPI query parameter map into a Flag.
func buildFlag(param map[string]interface{}) (Flag, error) {
	name, _ := param["name"].(string)
	required, _ := param["required"].(bool)
	description, _ := param["description"].(string)

	schema, _ := param["schema"].(map[string]interface{})
	schemaType, _ := schema["type"].(string)
	defaultVal := schema["default"]

	flagType := mapSchemaType(schemaType)
	defaultStr := ""
	if defaultVal != nil {
		switch v := defaultVal.(type) {
		case float64:
			if v == float64(int64(v)) {
				defaultStr = strconv.FormatInt(int64(v), 10)
			} else {
				defaultStr = strconv.FormatFloat(v, 'f', -1, 64)
			}
		default:
			defaultStr = fmt.Sprintf("%v", defaultVal)
		}
	}

	var enumVals []string
	if rawEnum, ok := schema["enum"].([]interface{}); ok {
		for _, v := range rawEnum {
			enumVals = append(enumVals, fmt.Sprintf("%v", v))
		}
	}

	return Flag{
		Name:        name,
		Type:        flagType,
		Required:    required,
		Default:     defaultStr,
		Description: description,
		Source:      FlagSourceQuery,
		Enum:        enumVals,
	}, nil
}

// mapSchemaType converts an OpenAPI schema type string to a FlagType.
// Unknown types default to FlagTypeString.
func mapSchemaType(schemaType string) FlagType {
	switch schemaType {
	case "integer":
		return FlagTypeInt
	case "boolean":
		return FlagTypeBool
	case "array":
		return FlagTypeStringSlice
	default:
		return FlagTypeString
	}
}

// camelToKebab converts a camelCase identifier to kebab-case.
// Examples: "userId" → "user-id", "petId" → "pet-id", "id" → "id".
func camelToKebab(s string) string {
	if s == "" {
		return s
	}

	var builder strings.Builder
	runes := []rune(s)

	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 && unicode.IsLower(runes[i-1]) {
			builder.WriteRune('-')
		}
		builder.WriteRune(unicode.ToLower(r))
	}

	return builder.String()
}
