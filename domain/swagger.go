package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var swaggerHTTPMethods = map[string]struct{}{
	"get":     {},
	"post":    {},
	"put":     {},
	"delete":  {},
	"patch":   {},
	"options": {},
	"head":    {},
}

// ParseSwaggerSpec parses an OpenAPI v2 or v3 specification file into endpoint templates.
// The file format is auto-detected from the extension (.json, .yaml, .yml).
func ParseSwaggerSpec(filePath string) ([]EndpointTemplate, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read swagger file: %w", err)
	}

	var doc map[string]interface{}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parse swagger yaml: %w", err)
		}
	default:
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parse swagger json: %w", err)
		}
	}

	if doc == nil {
		return nil, fmt.Errorf("swagger spec is empty")
	}

	if err := validateOpenAPIVersion(doc); err != nil {
		return nil, err
	}

	return extractEndpoints(doc), nil
}

func validateOpenAPIVersion(doc map[string]interface{}) error {
	if v, ok := doc["swagger"]; ok {
		if s, ok := v.(string); ok && s == "2.0" {
			return nil
		}
		return fmt.Errorf("unsupported swagger version: %v", v)
	}

	if v, ok := doc["openapi"]; ok {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "3.") {
			return nil
		}
		return fmt.Errorf("unsupported openapi version: %v", v)
	}

	return fmt.Errorf("missing openapi version field (expected \"swagger\" or \"openapi\" key)")
}

// extractServerPrefix returns the path prefix to prepend to every path in the spec.
//
// For OpenAPI v2 it reads the top-level "basePath" field (e.g. "/api/v1").
// For OpenAPI v3 it reads the path component of the first "servers[].url" entry
// (e.g. "https://api.example.com/v1/users" → "/v1/users").
// Returns an empty string when there is no meaningful prefix.
func extractServerPrefix(doc map[string]interface{}) string {
	// OpenAPI v2: top-level "basePath"
	if bp, ok := doc["basePath"].(string); ok && bp != "" && bp != "/" {
		return strings.TrimRight(bp, "/")
	}

	// OpenAPI v3: "servers[0].url" path component
	servers, ok := doc["servers"].([]interface{})
	if !ok || len(servers) == 0 {
		return ""
	}

	first, ok := servers[0].(map[string]interface{})
	if !ok {
		return ""
	}

	rawURL, _ := first["url"].(string)
	if rawURL == "" {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" || u.Path == "/" {
		return ""
	}

	return strings.TrimRight(u.Path, "/")
}

func extractEndpoints(doc map[string]interface{}) []EndpointTemplate {
	prefix := extractServerPrefix(doc)

	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		return []EndpointTemplate{}
	}

	var result []EndpointTemplate

	for pathStr, pathItemRaw := range paths {
		pathItem, ok := pathItemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		pathLevelParams := extractParameters(pathItem["parameters"])

		for method, operationRaw := range pathItem {
			if _, isMethod := swaggerHTTPMethods[strings.ToLower(method)]; !isMethod {
				continue
			}

			operation, ok := operationRaw.(map[string]interface{})
			if !ok {
				continue
			}

			opParams := extractParameters(operation["parameters"])
			params := mergeParameters(pathLevelParams, opParams)

			result = append(result, EndpointTemplate{
				Method:       strings.ToUpper(method),
				PathTemplate: prefix + pathStr,
				Parameters:   params,
			})
		}
	}

	return result
}

// mergeParameters merges path-level and operation-level parameters.
// Operation-level parameters with the same name override path-level ones.
func mergeParameters(pathLevel, opLevel []Parameter) []Parameter {
	if len(opLevel) == 0 {
		return pathLevel
	}
	if len(pathLevel) == 0 {
		return opLevel
	}

	overrides := make(map[string]Parameter, len(opLevel))
	for _, p := range opLevel {
		overrides[p.Name] = p
	}

	merged := make([]Parameter, 0, len(pathLevel)+len(opLevel))
	seen := make(map[string]struct{}, len(pathLevel)+len(opLevel))

	for _, p := range pathLevel {
		if op, overridden := overrides[p.Name]; overridden {
			merged = append(merged, op)
		} else {
			merged = append(merged, p)
		}
		seen[p.Name] = struct{}{}
	}

	for _, p := range opLevel {
		if _, alreadyAdded := seen[p.Name]; !alreadyAdded {
			merged = append(merged, p)
		}
	}

	return merged
}

// extractParameters extracts path parameters from a raw "parameters" array value.
func extractParameters(raw interface{}) []Parameter {
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	var params []Parameter
	for _, item := range arr {
		p, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		inVal, _ := p["in"].(string)
		if !strings.EqualFold(inVal, "path") {
			continue
		}

		name, _ := p["name"].(string)
		if name == "" {
			continue
		}

		params = append(params, Parameter{
			Name:   name,
			Type:   resolveParamType(p),
			Source: "swagger",
		})
	}

	return params
}

// resolveParamType maps an OpenAPI parameter object's type and format to the internal type string.
func resolveParamType(param map[string]interface{}) string {
	// OpenAPI v3 uses a nested "schema" object
	if schema, ok := param["schema"].(map[string]interface{}); ok {
		return resolveTypeFromSchema(schema)
	}

	// OpenAPI v2 has type/format at the parameter level directly
	return resolveTypeFromSchema(param)
}

func resolveTypeFromSchema(schema map[string]interface{}) string {
	typVal, _ := schema["type"].(string)
	formatVal, _ := schema["format"].(string)

	switch typVal {
	case "integer":
		return "int"
	case "string":
		if strings.EqualFold(formatVal, "uuid") {
			return "uuid"
		}
		return "string"
	case "":
		return "unknown"
	default:
		return "unknown"
	}
}
