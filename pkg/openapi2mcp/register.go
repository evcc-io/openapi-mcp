// register.go
package openapi2mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
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
func formatParameterValue(val any, isInteger bool) string {
	if isInteger {
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

// generateAIFriendlyDescription creates a comprehensive, AI-optimized description for an operation
// that includes all the information an AI agent needs to understand how to use the tool.
func generateAIFriendlyDescription(op OpenAPIOperation, inputSchema map[string]any, apiKeyHeader string) string {
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
	var requiredParams []string
	switch req := inputSchema["required"].(type) {
	case []any:
		for _, r := range req {
			if str, ok := r.(string); ok {
				requiredParams = append(requiredParams, str)
			}
		}
	case []string:
		requiredParams = req
	}

	// Add parameter information with examples
	if properties, ok := inputSchema["properties"].(map[string]any); ok && len(properties) > 0 {
		desc.WriteString("\n\nPARAMETERS:")

		if len(requiredParams) > 0 {
			desc.WriteString("\n• Required:")
			for _, reqStr := range requiredParams {
				if prop, ok := properties[reqStr].(map[string]any); ok {
					desc.WriteString(fmt.Sprintf("\n  - %s", reqStr))
					if typeStr, ok := prop["type"].(string); ok {
						desc.WriteString(fmt.Sprintf(" (%s)", typeStr))
					}
					if propDesc, ok := prop["description"].(string); ok && propDesc != "" {
						desc.WriteString(": " + propDesc)
					}
					// Add enum values if present
					if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
						var enumStrs []string
						for _, e := range enum {
							enumStrs = append(enumStrs, fmt.Sprintf("%v", e))
						}
						desc.WriteString(" [values: " + strings.Join(enumStrs, ", ") + "]")
					}
				}
			}
		}

		// Optional parameters
		var optionalParams []string
		for paramName, paramDef := range properties {
			isRequired := false
			for _, reqParam := range requiredParams {
				if reqParam == paramName {
					isRequired = true
					break
				}
			}
			if !isRequired {
				if prop, ok := paramDef.(map[string]any); ok {
					paramInfo := fmt.Sprintf("  - %s", paramName)
					if typeStr, ok := prop["type"].(string); ok {
						paramInfo += fmt.Sprintf(" (%s)", typeStr)
					}
					if propDesc, ok := prop["description"].(string); ok && propDesc != "" {
						paramInfo += ": " + propDesc
					}
					if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
						var enumStrs []string
						for _, e := range enum {
							enumStrs = append(enumStrs, fmt.Sprintf("%v", e))
						}
						paramInfo += " [values: " + strings.Join(enumStrs, ", ") + "]"
					}
					optionalParams = append(optionalParams, paramInfo)
				}
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
	if properties, ok := inputSchema["properties"].(map[string]any); ok {
		// Add required parameters to example
		for _, reqStr := range requiredParams {
			if prop, ok := properties[reqStr].(map[string]any); ok {
				exampleArgs[reqStr] = generateExampleValue(prop)
			}
		}
		// Add one or two optional parameters to show structure
		count := 0
		for paramName, paramDef := range properties {
			if _, exists := exampleArgs[paramName]; !exists && count < 2 {
				if prop, ok := paramDef.(map[string]any); ok {
					// Skip adding optional params if there are already many required ones
					if len(exampleArgs) < 3 {
						exampleArgs[paramName] = generateExampleValue(prop)
						count++
					}
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

// generateExampleValue creates appropriate example values based on the parameter schema
func generateExampleValue(prop map[string]any) any {
	typeStr, _ := prop["type"].(string)

	// Check for enum values first
	if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}

	// Check for example values in schema
	if example, ok := prop["example"]; ok {
		return example
	}

	// Generate based on type
	switch typeStr {
	case "string":
		if format, ok := prop["format"].(string); ok {
			switch format {
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
		}
		return "example_string"
	case "number":
		return 123.45
	case "integer":
		return 123
	case "boolean":
		return true
	case "array":
		if items, ok := prop["items"].(map[string]any); ok {
			return []any{generateExampleValue(items)}
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

	// Extract API key header name from securitySchemes
	apiKeyHeader := "Fastly-Key" // default fallback
	if doc.Components != nil && doc.Components.SecuritySchemes != nil {
		if sec, ok := doc.Components.SecuritySchemes["ApiKeyAuth"]; ok && sec.Value != nil {
			if sec.Value.Type == "apiKey" && sec.Value.In == "header" && sec.Value.Name != "" {
				apiKeyHeader = sec.Value.Name
			}
		}
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
		for _, tag := range op.Tags {
			return slices.Contains(opts.TagFilter, tag)
		}
		return false
	}

	for _, op := range ops {
		if !filterByTag(op) {
			continue
		}

		inputSchema := BuildInputSchema(op.Parameters, op.RequestBody)
		if opts != nil && opts.PostProcessSchema != nil {
			inputSchema = opts.PostProcessSchema(op.OperationID, inputSchema)
		}
		inputSchemaJSON, _ := json.MarshalIndent(inputSchema, "", "  ")

		// Generate AI-friendly description
		desc := generateAIFriendlyDescription(op, inputSchema, apiKeyHeader)

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

		inputSchemaObj := &jsonschema.Schema{}
		if err := json.Unmarshal(inputSchemaJSON, inputSchemaObj); err != nil {
			inputSchemaObj = nil // fallback to nil if parsing fails
		}
		
		tool := &mcp.Tool{
			Name: name,
			Description: desc,
			InputSchema: inputSchemaObj,
		}
		tool.Annotations = &annotations
		// toolSchemas[name] = inputSchemaJSON

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

		mcp.AddTool(server, tool, toolHandler(
			name,
			op,
			doc,
			inputSchemaJSON,
			baseURLs,
			opts != nil && opts.ConfirmDangerousActions,
			requestHandler,
		))

		toolNames = append(toolNames, name)
	}

	// Add a tool for externalDocs if present
	if doc.ExternalDocs != nil && doc.ExternalDocs.URL != "" && (opts == nil || !opts.DryRun) {
		desc := "Show the OpenAPI external documentation URL and description."
		inputSchema := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		inputSchemaJSON, _ := json.MarshalIndent(inputSchema, "", "  ")
		inputSchemaObj := &jsonschema.Schema{}
		if err := json.Unmarshal(inputSchemaJSON, inputSchemaObj); err != nil {
			inputSchemaObj = nil // fallback to nil if parsing fails
		}
		
		tool := &mcp.Tool{
			Name: "externalDocs",
			Description: desc,
			InputSchema: inputSchemaObj,
		}
		tool.Annotations = &mcp.ToolAnnotations{}
		if opts != nil && opts.Version != "" {
			tool.Annotations.Title = "OpenAPI " + opts.Version
		}
		mcp.AddTool(server, tool, func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
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
			}, nil
		})
		toolNames = append(toolNames, "externalDocs")
	}

	// Add a tool for info if present
	if doc.Info != nil && (opts == nil || !opts.DryRun) {
		desc := "Show API metadata: title, version, description, and terms of service."
		inputSchema := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}

		inputSchemaJSON, _ := json.MarshalIndent(inputSchema, "", "  ")
		inputSchemaObj := &jsonschema.Schema{}
		if err := json.Unmarshal(inputSchemaJSON, inputSchemaObj); err != nil {
			inputSchemaObj = nil // fallback to nil if parsing fails
		}
		
		tool := &mcp.Tool{
			Name: "info",
			Description: desc,
			InputSchema: inputSchemaObj,
		}
		tool.Annotations = &mcp.ToolAnnotations{}
		if opts != nil && opts.Version != "" {
			tool.Annotations.Title = "OpenAPI " + opts.Version
		}

		mcp.AddTool(server, tool, func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
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
			}, nil
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

		server.AddResource(&timestampResource, func(ctx context.Context, session *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
			now := time.Now().Unix()
			content := fmt.Sprintf(`{"unix_timestamp": %d, "iso8601": "%s", "timezone": "%s"}`,
				now,
				time.Now().Format(time.RFC3339),
				time.Now().Format("MST"))

			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					&mcp.ResourceContents{
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
