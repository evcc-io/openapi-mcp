// schema.go
package openapi2mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
)

// escapeParameterName converts parameter names with brackets to MCP-compatible names.
// For example: "filter[created_at]" becomes "filter_created_at_"
// The trailing underscore distinguishes escaped names from naturally occurring names.
func escapeParameterName(name string) string {
	if !strings.Contains(name, "[") && !strings.Contains(name, "]") {
		return name // No escaping needed
	}

	// Replace brackets with underscores and add trailing underscore
	escaped := strings.ReplaceAll(name, "[", "_")
	escaped = strings.ReplaceAll(escaped, "]", "_")

	// Add trailing underscore if not already present to mark as escaped
	if !strings.HasSuffix(escaped, "_") {
		escaped += "_"
	}

	return escaped
}

// unescapeParameterName converts escaped parameter names back to their original form.
// This maintains a mapping from escaped names to original names for parameter lookup.
func unescapeParameterName(escaped string, originalNames map[string]string) string {
	if original, exists := originalNames[escaped]; exists {
		return original
	}
	return escaped // Return as-is if not found in mapping
}

// buildParameterNameMapping creates a mapping from escaped parameter names to original names.
// This is used to reverse the escaping when looking up parameter values.
func buildParameterNameMapping(params openapi3.Parameters) map[string]string {
	mapping := make(map[string]string)
	for _, paramRef := range params {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		p := paramRef.Value
		escaped := escapeParameterName(p.Name)
		if escaped != p.Name {
			mapping[escaped] = p.Name
		}
	}
	return mapping
}

// extractProperty recursively extracts a property schema from an OpenAPI SchemaRef.
// Handles allOf, oneOf, anyOf, discriminator, default, example, and basic OpenAPI 3.1 features.
func extractProperty(s *openapi3.SchemaRef) *jsonschema.Schema {
	if s == nil || s.Value == nil {
		return nil
	}
	val := s.Value
	prop := &jsonschema.Schema{}
	
	// Handle allOf (merge all subschemas)
	if len(val.AllOf) > 0 {
		allOfSchemas := make([]*jsonschema.Schema, len(val.AllOf))
		for i, sub := range val.AllOf {
			allOfSchemas[i] = extractProperty(sub)
		}
		prop.AllOf = allOfSchemas
	}
	
	// Handle oneOf/anyOf
	if len(val.OneOf) > 0 {
		fmt.Fprintf(os.Stderr, "[WARN] oneOf used in schema at %p. Only basic support is provided.\n", val)
		oneOfSchemas := make([]*jsonschema.Schema, len(val.OneOf))
		for i, sub := range val.OneOf {
			oneOfSchemas[i] = extractProperty(sub)
		}
		prop.OneOf = oneOfSchemas
	}
	if len(val.AnyOf) > 0 {
		fmt.Fprintf(os.Stderr, "[WARN] anyOf used in schema at %p. Only basic support is provided.\n", val)
		anyOfSchemas := make([]*jsonschema.Schema, len(val.AnyOf))
		for i, sub := range val.AnyOf {
			anyOfSchemas[i] = extractProperty(sub)
		}
		prop.AnyOf = anyOfSchemas
	}
	
	// Handle discriminator (OpenAPI 3.0/3.1)
	if val.Discriminator != nil {
		fmt.Fprintf(os.Stderr, "[WARN] discriminator used in schema at %p. Only basic support is provided.\n", val)
		// Store discriminator in Extra map since it's not a standard JSON Schema field
		if prop.Extra == nil {
			prop.Extra = make(map[string]any)
		}
		prop.Extra["discriminator"] = val.Discriminator
	}
	
	// Type, format, description, enum, default, example
	if val.Type != nil && len(*val.Type) > 0 {
		// Use the first type if multiple types are specified
		prop.Type = (*val.Type)[0]
	}
	if val.Format != "" {
		prop.Format = val.Format
	}
	if val.Description != "" {
		prop.Description = val.Description
	}
	if len(val.Enum) > 0 {
		prop.Enum = val.Enum
	}
	if val.Default != nil {
		defaultBytes, _ := json.Marshal(val.Default)
		prop.Default = json.RawMessage(defaultBytes)
	}
	if val.Example != nil {
		prop.Examples = []any{val.Example}
	}
	
	// Object properties
	if val.Type != nil && val.Type.Is("object") && val.Properties != nil {
		prop.Properties = make(map[string]*jsonschema.Schema)
		for name, sub := range val.Properties {
			prop.Properties[name] = extractProperty(sub)
		}
		if len(val.Required) > 0 {
			prop.Required = val.Required
		}
	}
	
	// Array items
	if val.Type != nil && val.Type.Is("array") && val.Items != nil {
		prop.Items = extractProperty(val.Items)
	}
	
	return prop
}

// BuildInputSchema converts OpenAPI parameters and request body schema to a single JSON Schema object for MCP tool input validation.
// Returns a JSON Schema as a jsonschema.Schema.
// Example usage for BuildInputSchema:
//
//	params := ... // openapi3.Parameters from an operation
//	reqBody := ... // *openapi3.RequestBodyRef from an operation
//	schema := openapi2mcp.BuildInputSchema(params, reqBody)
//	// schema is a jsonschema.Schema representing the JSON schema for tool input
func BuildInputSchema(params openapi3.Parameters, requestBody *openapi3.RequestBodyRef) jsonschema.Schema {
	schema := jsonschema.Schema{
		Type:       "object",
		Properties: make(map[string]*jsonschema.Schema),
	}
	var required []string

	// Parameters (query, path, header, cookie)
	for _, paramRef := range params {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		p := paramRef.Value
		if p.Schema != nil && p.Schema.Value != nil {
			if p.Schema.Value.Type != nil && p.Schema.Value.Type.Is("string") && p.Schema.Value.Format == "binary" {
				fmt.Fprintf(os.Stderr, "[WARN] Parameter '%s' uses 'string' with 'binary' format. Non-JSON body types are not fully supported.\n", p.Name)
			}
			prop := extractProperty(p.Schema)
			if prop != nil {
				// Override description if parameter has its own description
				if p.Description != "" {
					prop.Description = p.Description
				}
				// Use escaped parameter name for MCP schema compatibility
				escapedName := escapeParameterName(p.Name)
				schema.Properties[escapedName] = prop
				if p.Required {
					required = append(required, escapedName)
				}
			}
		}
		// Warn about unsupported parameter locations
		if p.In != "query" && p.In != "path" && p.In != "header" && p.In != "cookie" {
			fmt.Fprintf(os.Stderr, "[WARN] Parameter '%s' uses unsupported location '%s'.\n", p.Name, p.In)
		}
	}

	// Request body (application/json and application/vnd.api+json)
	if requestBody != nil && requestBody.Value != nil {
		for mtName := range requestBody.Value.Content {
			// Check base content type without parameters
			baseMT := mtName
			if idx := strings.IndexByte(mtName, ';'); idx > 0 {
				baseMT = strings.TrimSpace(mtName[:idx])
			}
			if baseMT != "application/json" && baseMT != "application/vnd.api+json" {
				fmt.Fprintf(os.Stderr, "[WARN] Request body uses media type '%s'. Only 'application/json' and 'application/vnd.api+json' are fully supported.\n", mtName)
			}
		}
		// Try application/json first, then application/vnd.api+json (including with parameters)
		mt := getContentByType(requestBody.Value.Content, "application/json")
		if mt == nil {
			mt = getContentByType(requestBody.Value.Content, "application/vnd.api+json")
		}
		if mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
			bodyProp := extractProperty(mt.Schema)
			if bodyProp != nil {
				bodyProp.Description = "The JSON request body."
				schema.Properties["requestBody"] = bodyProp
				if requestBody.Value.Required {
					required = append(required, "requestBody")
				}
			}
		}
	}

	if len(required) > 0 {
		schema.Required = required
	}

	return schema
}

// SchemaToMap converts a jsonschema.Schema to map[string]any for backward compatibility
func SchemaToMap(schema jsonschema.Schema) map[string]any {
	schemaBytes, _ := json.Marshal(schema)
	var result map[string]any
	json.Unmarshal(schemaBytes, &result)
	return result
}

// MapToSchema converts a map[string]any to jsonschema.Schema
func MapToSchema(m map[string]any) jsonschema.Schema {
	schemaBytes, _ := json.Marshal(m)
	var result jsonschema.Schema
	json.Unmarshal(schemaBytes, &result)
	return result
}
