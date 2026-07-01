// register.go
package openapi2mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// getParameterValue retrieves a parameter value from args using the escaped parameter name.
// It tries the escaped name first, then falls back to the original name if not found.
func getParameterValue(args map[string]any, paramName string, paramNameMapping map[string]string) (any, bool) {
	escapedName := escapeParameterName(paramName)
	if val, ok := args[escapedName]; ok {
		return val, true
	}
	// Fallback to original name for backward compatibility
	if val, ok := args[paramName]; ok {
		return val, true
	}
	return nil, false
}

// formatParameterValue converts a parameter value to a string, formatting integers without decimals
// when the associated OpenAPI schema declares an integer type.
func formatParameterValue(val any, schema *openapi3.Schema) string {
	if schema != nil && schema.Type != nil && schema.Type.Is("integer") {
		// Handle integer formatting
		switch v := val.(type) {
		case float64:
			// Convert float64 to int64 to remove decimals
			return fmt.Sprintf("%d", int64(v))
		case float32:
			// Convert float32 to int64 to remove decimals
			return fmt.Sprintf("%d", int64(v))
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			// Already an integer type
			return fmt.Sprintf("%d", v)
		default:
			// Fallback to default formatting
			return fmt.Sprintf("%v", v)
		}
	}
	// Default formatting for non-integer types
	return fmt.Sprintf("%v", val)
}

// arrayItemSchema returns the schema for items in an array parameter, or nil if not applicable.
func arrayItemSchema(schema *openapi3.Schema) *openapi3.Schema {
	if schema == nil || schema.Items == nil {
		return nil
	}
	return schema.Items.Value
}

// getSeparator returns the separator for a given style
func getSeparator(style string) string {
	switch style {
	case "spaceDelimited":
		return " "
	case "pipeDelimited":
		return "|"
	default:
		return ","
	}
}

// objectPropertySchema returns the schema for a property in an object parameter, or nil if not applicable.
func objectPropertySchema(schema *openapi3.Schema, key string) *openapi3.Schema {
	if schema == nil {
		return nil
	}
	propRef, ok := schema.Properties[key]
	if !ok || propRef == nil {
		return nil
	}
	return propRef.Value
}

// serializeParameter is the main serialization function for all OpenAPI parameter types.
// It returns url.Values to handle all cases including multiple values for the same key.
// Ref. https://spec.openapis.org/oas/latest.html#parameter-object
//
// Styles:
//   - simple: comma-separated values (path, header)
//   - label: dot-prefixed values (path only)
//   - matrix: semicolon-prefixed name-value pairs (path only)
//   - form: ampersand-separated values (query, cookie)
//   - spaceDelimited: space-separated values (query only)
//   - pipeDelimited: pipe-separated values (query only)
//   - deepObject: nested object notation like id[role]=admin (query only)
//
// Note: it does NOT support allowReserved parameter, all values are percent-encoded as needed.
func serializeParameter(paramName string, val any, schema *openapi3.Schema, style string, explode, escapePath bool) url.Values {
	result := url.Values{}

	// Path params are percent-encoded per segment; other locations are encoded later by url.Values.Encode.
	escape := func(s string) string { return s }
	if escapePath {
		escape = url.PathEscape
	}

	// Check value type
	arr, isArray := val.([]any)
	obj, isObject := val.(map[string]any)

	// Helper to format and escape a value
	formatValue := func(v any, valueSchema *openapi3.Schema) string {
		return escape(formatParameterValue(v, valueSchema))
	}

	// Primitives - simple formatting
	// Example: id=5
	if !isArray && !isObject {
		formatted := formatValue(val, schema)
		switch style {
		case "label":
			// Label style: prefix with dot (e.g., .5)
			result.Set(paramName, "."+formatted)
		case "matrix":
			// Matrix style: semicolon-prefixed name=value (e.g., ;id=5)
			result.Set(paramName, ";"+escape(paramName)+"="+formatted)
		default: // form, simple, etc.
			// Default: just the value (e.g., 5)
			result.Set(paramName, formatted)
		}
		return result
	}

	// Arrays
	// Example: id=[3,4,5]
	if isArray {
		itemSchema := arrayItemSchema(schema)
		var parts []string
		for _, item := range arr {
			parts = append(parts, formatValue(item, itemSchema))
		}

		switch style {
		case "label":
			if explode {
				// Label exploded: dot-separated with dot prefix (e.g., .3.4.5)
				result.Set(paramName, "."+strings.Join(parts, "."))
			} else {
				// Label: comma-separated with dot prefix (e.g., .3,4,5)
				result.Set(paramName, "."+strings.Join(parts, ","))
			}
		case "matrix":
			if explode {
				// Matrix exploded: repeated name=value pairs (e.g., ;id=3;id=4;id=5)
				var matrixParts []string
				for _, part := range parts {
					matrixParts = append(matrixParts, escape(paramName)+"="+part)
				}
				result.Set(paramName, ";"+strings.Join(matrixParts, ";"))
			} else {
				// Matrix: single name with comma-separated values (e.g., ;id=3,4,5)
				result.Set(paramName, ";"+escape(paramName)+"="+strings.Join(parts, ","))
			}
		case "form", "spaceDelimited", "pipeDelimited":
			if explode {
				// Exploded: multiple parameters with same name (e.g., id=3&id=4&id=5)
				for _, part := range parts {
					result.Add(paramName, part)
				}
			} else {
				// Non-exploded: delimited values
				result.Set(paramName, strings.Join(parts, getSeparator(style)))
			}
		default: // simple, and any unlisted style: comma-separated
			result.Set(paramName, strings.Join(parts, ","))
		}
		return result
	}

	// Objects
	// Example: id={role:"admin",firstName:"Alex"}
	if isObject {
		// Sort keys for deterministic output
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		slices.Sort(keys)

		// Build key-value pairs for most styles
		buildParts := func() []string {
			var parts []string
			for _, k := range keys {
				propSchema := objectPropertySchema(schema, k)
				if explode {
					parts = append(parts, escape(k)+"="+formatValue(obj[k], propSchema))
				} else {
					parts = append(parts, escape(k), formatValue(obj[k], propSchema))
				}
			}
			return parts
		}

		switch style {
		case "label":
			sep := ","
			if explode {
				sep = "."
			}
			result.Set(paramName, "."+strings.Join(buildParts(), sep))
		case "matrix":
			if explode {
				// buildParts already yields k=v pairs when explode is true
				result.Set(paramName, ";"+strings.Join(buildParts(), ";"))
			} else {
				result.Set(paramName, ";"+escape(paramName)+"="+strings.Join(buildParts(), ","))
			}
		case "deepObject":
			// Deep object: bracket notation for nested properties (e.g., id[role]=admin&id[firstName]=Alex)
			for _, k := range keys {
				result.Set(paramName+"["+k+"]", formatValue(obj[k], objectPropertySchema(schema, k)))
			}
		case "form", "spaceDelimited", "pipeDelimited":
			if explode {
				// Exploded: each property as separate param
				for _, k := range keys {
					result.Set(k, formatValue(obj[k], objectPropertySchema(schema, k)))
				}
			} else {
				// Non-exploded: delimited key-value pairs
				var parts []string
				for _, k := range keys {
					parts = append(parts, k, formatValue(obj[k], objectPropertySchema(schema, k)))
				}
				result.Set(paramName, strings.Join(parts, getSeparator(style)))
			}
		default: // simple, and any unlisted style: comma-separated
			result.Set(paramName, strings.Join(buildParts(), ","))
		}
		return result
	}

	return result
}

// serializeQueryParameter serializes a query parameter (form/spaceDelimited/pipeDelimited/deepObject styles).
// NOTE: This implementation does NOT support the allowReserved parameter, all values are percent-encoded.
func serializeQueryParameter(paramName string, val any, schema *openapi3.Schema, style string, explode bool) url.Values {
	return serializeParameter(paramName, val, schema, style, explode, false)
}

// serializePathParameter serializes a path parameter (simple/label/matrix styles).
func serializePathParameter(paramName string, val any, schema *openapi3.Schema, style string, explode bool) string {
	// Use serializeParameter with URL path escaping for each scalar value
	result := serializeParameter(paramName, val, schema, style, explode, true)
	return result.Get(paramName)
}

// serializeHeaderParameter serializes a header parameter (simple style only).
// Headers always use simple style and are NOT URL-encoded.
func serializeHeaderParameter(val any, schema *openapi3.Schema, explode bool) string {
	values := serializeParameter("", val, schema, "simple", explode, false)
	return values.Get("")
}

// serializeCookieParameter serializes a cookie parameter (form style only).
// Uses semicolon-space ("; ") as separator per RFC6265 cookie format.
func serializeCookieParameter(paramName string, val any, schema *openapi3.Schema, explode bool) string {
	values := serializeParameter(paramName, val, schema, "form", explode, false)

	// Collect all key=value pairs
	var pairs []string

	// Check if val is an array to preserve order
	_, isArray := val.([]any)

	if isArray && explode {
		// For exploded arrays, preserve insertion order from url.Values
		if vals, ok := values[paramName]; ok {
			for _, v := range vals {
				pairs = append(pairs, paramName+"="+v)
			}
		}
	} else {
		// For objects or non-exploded values, collect and sort for determinism
		for key, vals := range values {
			for _, v := range vals {
				pairs = append(pairs, key+"="+v)
			}
		}
		slices.Sort(pairs)
	}

	return strings.Join(pairs, "; ")
}

// generateAIFriendlyDescription creates a comprehensive, AI-optimized description for an operation
// that includes all the information an AI agent needs to understand how to use the tool.
func generateAIFriendlyDescription(op OpenAPIOperation, inputSchema jsonschema.Schema) string {
	var desc strings.Builder

	// Start with the original description or summary
	if op.Description != "" {
		desc.WriteString(op.Description)
	} else if op.Summary != "" {
		desc.WriteString(op.Summary)
	}

	// Add authentication requirements if any
	if len(op.Security) > 0 {
		desc.WriteString("\n\nAUTHENTICATION: ")
		var authMethods []string
		for _, secReq := range op.Security {
			for schemeName := range secReq {
				authMethods = append(authMethods, schemeName)
			}
		}
		desc.WriteString("Required (" + strings.Join(authMethods, " OR ") + "). ")
		desc.WriteString("Set environment variables: API_KEY, BEARER_TOKEN, or BASIC_AUTH")
	}

	// Extract required parameters first
	requiredParams := inputSchema.Required

	// Add parameter information with examples
	properties := inputSchema.Properties
	if len(properties) > 0 {
		desc.WriteString("\n\nPARAMETERS:")

		if len(requiredParams) > 0 {
			desc.WriteString("\n• Required:")
			for _, reqStr := range requiredParams {
				if prop, ok := properties[reqStr]; ok && prop != nil {
					desc.WriteString(fmt.Sprintf("\n  - %s", reqStr))
					if prop.Type != "" {
						desc.WriteString(fmt.Sprintf(" (%s)", prop.Type))
					}
					if prop.Description != "" {
						desc.WriteString(": " + prop.Description)
					}
					// Add enum values if present
					if len(prop.Enum) > 0 {
						var enumStrs []string
						for _, e := range prop.Enum {
							enumStrs = append(enumStrs, fmt.Sprintf("%v", e))
						}
						desc.WriteString(" [values: " + strings.Join(enumStrs, ", ") + "]")
					}
				}
			}
		}

		// Optional parameters
		var optionalParams []string
		for paramName, prop := range properties {
			isRequired := false
			for _, reqParam := range requiredParams {
				if reqParam == paramName {
					isRequired = true
					break
				}
			}
			if !isRequired && prop != nil {
				paramInfo := fmt.Sprintf("  - %s", paramName)
				if prop.Type != "" {
					paramInfo += fmt.Sprintf(" (%s)", prop.Type)
				}
				if prop.Description != "" {
					paramInfo += ": " + prop.Description
				}
				// Add enum values if present
				if len(prop.Enum) > 0 {
					var enumStrs []string
					for _, e := range prop.Enum {
						enumStrs = append(enumStrs, fmt.Sprintf("%v", e))
					}
					paramInfo += " [values: " + strings.Join(enumStrs, ", ") + "]"
				}
				optionalParams = append(optionalParams, paramInfo)
			}
		}
		if len(optionalParams) > 0 {
			desc.WriteString("\n• Optional:")
			for _, param := range optionalParams {
				desc.WriteString("\n" + param)
			}
		}
	}

	// Add example usage
	desc.WriteString("\n\nEXAMPLE: call " + op.OperationID + " ")
	exampleArgs := make(map[string]any)

	// Generate example based on actual parameters
	if properties != nil {
		// Add required parameters to example
		for _, reqStr := range requiredParams {
			if prop, ok := properties[reqStr]; ok && prop != nil {
				exampleArgs[reqStr] = generateExampleValueFromSchema(prop)
			}
		}
		// Add one or two optional parameters to show structure
		count := 0
		for paramName, prop := range properties {
			if _, exists := exampleArgs[paramName]; !exists && count < 2 && prop != nil {
				// Skip adding optional params if there are already many required ones
				if len(exampleArgs) < 3 {
					exampleArgs[paramName] = generateExampleValueFromSchema(prop)
					count++
				}
			}
		}
	}

	exampleJSON, _ := json.Marshal(exampleArgs)
	desc.WriteString(string(exampleJSON))

	// Add response format info
	if op.Method == "get" || op.Method == "post" || op.Method == "put" {
		desc.WriteString("\n\nRESPONSE: Returns HTTP status, headers, and response body. ")
		desc.WriteString("Success responses (2xx) return the data. ")
		desc.WriteString("Error responses include troubleshooting guidance.")
	}

	// Add safety note for dangerous operations
	if op.Method == "delete" || op.Method == "put" || op.Method == "post" {
		desc.WriteString("\n\n⚠️  SAFETY: This operation modifies data. ")
		desc.WriteString("You will be asked to confirm before execution.")
	}

	return desc.String()
}

// generateExampleValueFromSchema creates appropriate example values based on the jsonschema.Schema
func generateExampleValueFromSchema(prop *jsonschema.Schema) any {
	if prop == nil {
		return nil
	}

	// Check for enum values first
	if len(prop.Enum) > 0 {
		return prop.Enum[0]
	}

	// Check for example values in schema
	if len(prop.Examples) > 0 {
		return prop.Examples[0]
	}

	// Generate based on type
	switch prop.Type {
	case "string":
		switch prop.Format {
		case "email":
			return "user@example.com"
		case "uri", "url":
			return "https://example.com"
		case "date":
			return "2024-01-01"
		case "date-time":
			return "2024-01-01T00:00:00Z"
		case "uuid":
			return "123e4567-e89b-12d3-a456-426614174000"
		default:
			return "example_string"
		}
	case "number":
		return 123.45
	case "integer":
		return 123
	case "boolean":
		return true
	case "array":
		if prop.Items != nil {
			return []any{generateExampleValueFromSchema(prop.Items)}
		}
		return []any{"item1", "item2"}
	case "object":
		return map[string]any{"key": "value"}
	default:
		return nil
	}
}

// hasDateTimeParameters checks if an operation has any date/time related parameters
func hasDateTimeParameters(op OpenAPIOperation) bool {
	// Check regular parameters
	for _, paramRef := range op.Parameters {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}

		// Check parameter name for date/time indicators
		paramName := strings.ToLower(paramRef.Value.Name)
		if strings.Contains(paramName, "date") || strings.Contains(paramName, "time") ||
			strings.Contains(paramName, "created_at") || strings.Contains(paramName, "updated_at") ||
			strings.Contains(paramName, "start_time") || strings.Contains(paramName, "end_time") {
			return true
		}

		// Check schema format
		if paramRef.Value.Schema != nil && paramRef.Value.Schema.Value != nil {
			schema := paramRef.Value.Schema.Value
			if schema.Format == "date" || schema.Format == "date-time" {
				return true
			}
			// Check for Unix timestamps (integers with certain names)
			if schema.Type != nil && schema.Type.Is("integer") && (strings.Contains(paramName, "time") || strings.Contains(paramName, "timestamp")) {
				return true
			}
		}
	}

	// Check request body schema if present
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for _, mediaType := range op.RequestBody.Value.Content {
			if mediaType.Schema != nil && mediaType.Schema.Value != nil {
				if hasDateTimeInSchema(mediaType.Schema.Value) {
					return true
				}
			}
		}
	}

	return false
}

// hasDateTimeInSchema recursively checks if a schema contains date/time formats
func hasDateTimeInSchema(schema *openapi3.Schema) bool {
	if schema.Format == "date" || schema.Format == "date-time" {
		return true
	}

	// Check properties in objects
	for _, propRef := range schema.Properties {
		if propRef != nil && propRef.Value != nil {
			if hasDateTimeInSchema(propRef.Value) {
				return true
			}
		}
	}

	// Check items in arrays
	if schema.Items != nil && schema.Items.Value != nil {
		if hasDateTimeInSchema(schema.Items.Value) {
			return true
		}
	}

	// Check allOf, anyOf, oneOf
	for _, schemaRef := range schema.AllOf {
		if schemaRef != nil && schemaRef.Value != nil {
			if hasDateTimeInSchema(schemaRef.Value) {
				return true
			}
		}
	}
	for _, schemaRef := range schema.AnyOf {
		if schemaRef != nil && schemaRef.Value != nil {
			if hasDateTimeInSchema(schemaRef.Value) {
				return true
			}
		}
	}
	for _, schemaRef := range schema.OneOf {
		if schemaRef != nil && schemaRef.Value != nil {
			if hasDateTimeInSchema(schemaRef.Value) {
				return true
			}
		}
	}

	return false
}

// RegisterOpenAPITools registers each OpenAPI operation as an MCP tool with a real HTTP handler.
// Also adds tools for externalDocs, info, and describe if present in the OpenAPI spec.
// The handler validates arguments, builds the HTTP request, and returns the HTTP response as the tool result.
// Returns the list of tool names registered.
func RegisterOpenAPITools(server *mcp.Server, ops []OpenAPIOperation, doc *openapi3.T, opts *ToolGenOptions) []string {
	baseURLs := []string{}
	if os.Getenv("OPENAPI_BASE_URL") != "" {
		baseURLs = append(baseURLs, os.Getenv("OPENAPI_BASE_URL"))
	} else if len(doc.Servers) > 0 {
		for _, s := range doc.Servers {
			if s != nil && s.URL != "" {
				baseURLs = append(baseURLs, s.URL)
			}
		}
	} else {
		baseURLs = append(baseURLs, "http://localhost:8080")
	}

	// Map from operationID to inputSchema JSON for validation
	// toolSchemas := make(map[string][]byte)
	var toolNames []string
	var toolSummaries []map[string]any

	// Tag filtering
	filterByTag := func(op OpenAPIOperation) bool {
		if opts == nil || len(opts.TagFilter) == 0 {
			return true
		}
		found := false
		for _, tag := range opts.TagFilter {
			if slices.Contains(op.Tags, tag) {
				found = true
				break
			}
		}
		return found
	}

	for _, op := range ops {
		if !filterByTag(op) {
			continue
		}

		inputSchema := BuildInputSchema(op.Parameters, op.RequestBody)
		if opts != nil && opts.PostProcessSchema != nil {
			inputSchema = opts.PostProcessSchema(op.OperationID, inputSchema)
		}

		// Generate AI-friendly description
		desc := generateAIFriendlyDescription(op, inputSchema)

		name := op.OperationID
		if opts != nil && opts.NameFormat != nil {
			name = opts.NameFormat(name)
		}

		annotations := mcp.ToolAnnotations{}
		var titleParts []string
		if opts != nil && opts.Version != "" {
			titleParts = append(titleParts, "OpenAPI "+opts.Version)
		}
		if len(op.Tags) > 0 {
			titleParts = append(titleParts, "Tags: "+strings.Join(op.Tags, ", "))
		}
		if len(titleParts) > 0 {
			annotations.Title = strings.Join(titleParts, " | ")
		}

		tool := &mcp.Tool{
			Name:        name,
			Description: desc,
			InputSchema: &inputSchema,
		}
		tool.Annotations = &annotations

		if opts != nil && opts.DryRun {
			// For dry run, collect summary info
			toolSummaries = append(toolSummaries, map[string]any{
				"name":        name,
				"description": desc,
				"tags":        op.Tags,
				"inputSchema": inputSchema,
			})
			toolNames = append(toolNames, name)
			continue
		}

		requestHandler := defaultRequestHandler
		if opts != nil && opts.RequestHandler != nil {
			requestHandler = opts.RequestHandler
		}

		// j, _ := json.MarshalIndent(inputSchema, "", "  ")
		// fmt.Println(string(j))

		mcp.AddTool(server, tool, toolHandler(
			name,
			op,
			doc,
			inputSchema,
			baseURLs,
			opts != nil && opts.ConfirmDangerousActions,
			requestHandler,
		))

		toolNames = append(toolNames, name)
	}

	// Add a tool for externalDocs if present
	if doc.ExternalDocs != nil && doc.ExternalDocs.URL != "" && (opts == nil || !opts.DryRun) {
		tool := &mcp.Tool{
			Name:        "externalDocs",
			Description: "Show the OpenAPI external documentation URL and description.",
		}

		if opts != nil && opts.Version != "" {
			tool.Annotations = &mcp.ToolAnnotations{
				Title: "OpenAPI " + opts.Version,
			}
		}

		mcp.AddTool(server, tool, func(_ context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			info := "External documentation URL: " + doc.ExternalDocs.URL
			if doc.ExternalDocs.Description != "" {
				info += "\nDescription: " + doc.ExternalDocs.Description
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: info,
					},
				},
			}, nil, nil
		})
		toolNames = append(toolNames, "externalDocs")
	}

	// Add a tool for info if present
	if doc.Info != nil && (opts == nil || !opts.DryRun) {
		tool := &mcp.Tool{
			Name:        "info",
			Description: "Show API metadata: title, version, description, and terms of service.",
		}

		if opts != nil && opts.Version != "" {
			tool.Annotations = &mcp.ToolAnnotations{
				Title: "OpenAPI " + opts.Version,
			}
		}

		mcp.AddTool(server, tool, func(_ context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			var sb strings.Builder
			if doc.Info.Title != "" {
				sb.WriteString("Title: " + doc.Info.Title + "\n")
			}
			if doc.Info.Version != "" {
				sb.WriteString("Version: " + doc.Info.Version + "\n")
			}
			if doc.Info.Description != "" {
				sb.WriteString("Description: " + doc.Info.Description + "\n")
			}
			if doc.Info.TermsOfService != "" {
				sb.WriteString("Terms of Service: " + doc.Info.TermsOfService + "\n")
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: strings.TrimSpace(sb.String()),
					},
				},
			}, nil, nil
		})
		toolNames = append(toolNames, "info")
	}

	if opts != nil && opts.DryRun {
		if opts.PrettyPrint {
			out, _ := json.MarshalIndent(toolSummaries, "", "  ")
			fmt.Println(string(out))
		} else {
			out, _ := json.Marshal(toolSummaries)
			fmt.Println(string(out))
		}
	}

	// Check if any operations use date/time parameters
	hasTimeRelatedOps := false
	for _, op := range ops {
		if hasDateTimeParameters(op) {
			hasTimeRelatedOps = true
			break
		}
	}

	// Add a resource that provides the current Unix timestamp only if there are time-related operations
	if hasTimeRelatedOps && (opts == nil || !opts.DryRun) {
		timestampResource := mcp.Resource{
			URI:         "timestamp://current",
			Name:        "Current Unix Timestamp",
			Description: "Provides the current Unix timestamp in seconds to help the AI understand the current date and time",
			MIMEType:    "application/json",
		}

		server.AddResource(&timestampResource, func(ctx context.Context, req *mcp.ServerRequest[*mcp.ReadResourceParams]) (*mcp.ReadResourceResult, error) {
			now := time.Now().Unix()
			content := fmt.Sprintf(`{"unix_timestamp": %d, "iso8601": "%s", "timezone": "%s"}`,
				now,
				time.Now().Format(time.RFC3339),
				time.Now().Format("MST"))

			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      timestampResource.URI,
						MIMEType: "application/json",
						Text:     content,
					},
				},
			}, nil
		})
	}

	return toolNames
}
