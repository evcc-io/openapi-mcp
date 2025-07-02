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
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
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

// generateAI400ErrorResponse creates a comprehensive, AI-optimized error response for 400 HTTP errors
// that helps agents understand how to correctly use the tool.
func generateAI400ErrorResponse(op OpenAPIOperation, inputSchemaJSON []byte, args map[string]any, responseBody string) string {
	var response strings.Builder

	// Start with clear explanation
	response.WriteString("BAD REQUEST (400): The API call failed due to incorrect or invalid parameters.\n\n")

	// Operation context
	response.WriteString(fmt.Sprintf("OPERATION: %s", op.OperationID))
	if op.Summary != "" {
		response.WriteString(fmt.Sprintf(" - %s", op.Summary))
	}
	response.WriteString("\n")
	if op.Description != "" {
		response.WriteString(fmt.Sprintf("DESCRIPTION: %s\n", op.Description))
	}
	response.WriteString("\n")

	// Parse the input schema to provide detailed parameter guidance
	var schemaObj map[string]any
	_ = json.Unmarshal(inputSchemaJSON, &schemaObj)

	if properties, ok := schemaObj["properties"].(map[string]any); ok && len(properties) > 0 {
		response.WriteString("PARAMETER REQUIREMENTS:\n")

		// Required parameters
		if required, ok := schemaObj["required"].([]any); ok && len(required) > 0 {
			response.WriteString("• Required parameters:\n")
			for _, req := range required {
				if reqStr, ok := req.(string); ok {
					if prop, ok := properties[reqStr].(map[string]any); ok {
						response.WriteString(fmt.Sprintf("  - %s", reqStr))
						if typeStr, ok := prop["type"].(string); ok {
							response.WriteString(fmt.Sprintf(" (%s)", typeStr))
						}
						if desc, ok := prop["description"].(string); ok && desc != "" {
							response.WriteString(fmt.Sprintf(": %s", desc))
						}
						response.WriteString("\n")
					}
				}
			}
			response.WriteString("\n")
		}

		// All parameters with details
		response.WriteString("• All available parameters:\n")
		for paramName, paramDef := range properties {
			if prop, ok := paramDef.(map[string]any); ok {
				response.WriteString(fmt.Sprintf("  - %s", paramName))

				// Type information
				if typeStr, ok := prop["type"].(string); ok {
					response.WriteString(fmt.Sprintf(" (%s)", typeStr))
				}

				// Required indicator
				if required, ok := schemaObj["required"].([]any); ok {
					for _, req := range required {
						if reqStr, ok := req.(string); ok && reqStr == paramName {
							response.WriteString(" [REQUIRED]")
							break
						}
					}
				}

				// Description
				if desc, ok := prop["description"].(string); ok && desc != "" {
					response.WriteString(fmt.Sprintf(": %s", desc))
				}

				// Enum values
				if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
					response.WriteString(" | Valid values: ")
					var enumStrs []string
					for _, e := range enum {
						enumStrs = append(enumStrs, fmt.Sprintf("%v", e))
					}
					response.WriteString(strings.Join(enumStrs, ", "))
				}

				response.WriteString("\n")
			}
		}
		response.WriteString("\n")
	}

	// Analyze current arguments
	if len(args) > 0 {
		response.WriteString("YOUR CURRENT ARGUMENTS:\n")
		argsJSON, _ := json.MarshalIndent(args, "", "  ")
		response.WriteString(string(argsJSON))
		response.WriteString("\n\n")
	}

	// Server error details if available
	if responseBody != "" {
		response.WriteString("SERVER ERROR DETAILS:\n")
		response.WriteString(responseBody)
		response.WriteString("\n\n")
	}

	// Generate example with correct parameters
	response.WriteString("EXAMPLE CORRECT USAGE:\n")
	if properties, ok := schemaObj["properties"].(map[string]any); ok {
		exampleArgs := map[string]any{}

		// Prioritize required parameters
		if required, ok := schemaObj["required"].([]any); ok {
			for _, req := range required {
				if reqStr, ok := req.(string); ok {
					if prop, ok := properties[reqStr].(map[string]any); ok {
						exampleArgs[reqStr] = generateExampleValue(prop)
					}
				}
			}
		}

		// Add some optional parameters for completeness
		count := 0
		for paramName, paramDef := range properties {
			if _, exists := exampleArgs[paramName]; !exists && count < 3 {
				if prop, ok := paramDef.(map[string]any); ok {
					exampleArgs[paramName] = generateExampleValue(prop)
					count++
				}
			}
		}

		exampleJSON, _ := json.MarshalIndent(exampleArgs, "", "  ")
		response.WriteString(fmt.Sprintf("call %s %s\n\n", op.OperationID, string(exampleJSON)))
	}

	// Actionable guidance
	response.WriteString("TROUBLESHOOTING STEPS:\n")
	response.WriteString("1. Verify all required parameters are provided\n")
	response.WriteString("2. Check parameter types match the schema (string, number, boolean, etc.)\n")
	response.WriteString("3. Ensure enum values are from the allowed list\n")
	response.WriteString("4. Validate parameter formats (dates, emails, URLs, etc.)\n")
	response.WriteString("5. Check for missing or incorrectly named parameters\n")
	response.WriteString("6. Review the server error details above for specific validation failures\n")

	return response.String()
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

// generateAI401403ErrorResponse creates comprehensive, AI-optimized error response for authentication/authorization failures
func generateAI401403ErrorResponse(op OpenAPIOperation, inputSchemaJSON []byte, args map[string]any, responseBody string, statusCode int) string {
	var response strings.Builder

	if statusCode == 401 {
		response.WriteString("AUTHENTICATION REQUIRED (401): Your request lacks valid authentication credentials.\n\n")
	} else {
		response.WriteString("AUTHORIZATION FAILED (403): You don't have permission to access this resource.\n\n")
	}

	// Operation context
	response.WriteString(fmt.Sprintf("OPERATION: %s", op.OperationID))
	if op.Summary != "" {
		response.WriteString(fmt.Sprintf(" - %s", op.Summary))
	}
	response.WriteString("\n\n")

	// Parse security requirements from the operation
	var schemaObj map[string]any
	_ = json.Unmarshal(inputSchemaJSON, &schemaObj)

	response.WriteString("AUTHENTICATION METHODS:\n")
	if len(op.Security) > 0 {
		response.WriteString("This operation requires one of the following authentication methods:\n")
		for i, secReq := range op.Security {
			response.WriteString(fmt.Sprintf("%d. ", i+1))
			var schemes []string
			for schemeName := range secReq {
				schemes = append(schemes, schemeName)
			}
			response.WriteString(strings.Join(schemes, " + "))
			response.WriteString("\n")
		}
	} else {
		response.WriteString("• Check the OpenAPI spec for security requirements\n")
		response.WriteString("• This operation may require global authentication\n")
	}
	response.WriteString("\n")

	response.WriteString("AUTHENTICATION SETUP:\n")
	response.WriteString("Set one of these environment variables based on your API:\n\n")

	response.WriteString("• API Key Authentication:\n")
	response.WriteString("  export API_KEY=\"your-api-key-here\"\n")
	response.WriteString("  # Common header names: X-API-Key, Authorization, Api-Key\n\n")

	response.WriteString("• Bearer Token Authentication:\n")
	response.WriteString("  export BEARER_TOKEN=\"your-bearer-token-here\"\n")
	response.WriteString("  # Sets Authorization: Bearer <token>\n\n")

	response.WriteString("• Basic Authentication:\n")
	response.WriteString("  export BASIC_AUTH=\"username:password\"\n")
	response.WriteString("  # Sets Authorization: Basic <base64-encoded-credentials>\n\n")

	// Server error details if available
	if responseBody != "" {
		response.WriteString("SERVER ERROR DETAILS:\n")
		response.WriteString(responseBody)
		response.WriteString("\n\n")
	}

	response.WriteString("TROUBLESHOOTING STEPS:\n")
	if statusCode == 401 {
		response.WriteString("1. Verify you have set the correct authentication environment variable\n")
		response.WriteString("2. Check that your API key/token is valid and not expired\n")
		response.WriteString("3. Ensure the authentication method matches what the API expects\n")
		response.WriteString("4. Test your credentials with a simple API call (like GET /health)\n")
		response.WriteString("5. Check the API documentation for required authentication format\n")
		response.WriteString("6. Verify the API endpoint URL is correct\n")
	} else {
		response.WriteString("1. Verify your account has permission to access this resource\n")
		response.WriteString("2. Check if your API key has the required scopes/permissions\n")
		response.WriteString("3. Ensure you're accessing the correct resource ID/path\n")
		response.WriteString("4. Contact the API provider to verify your account permissions\n")
		response.WriteString("5. Check if there are rate limits or usage restrictions\n")
		response.WriteString("6. Verify your subscription/plan includes access to this endpoint\n")
	}

	return response.String()
}

// generateAI404ErrorResponse creates comprehensive, AI-optimized error response for resource not found errors
func generateAI404ErrorResponse(op OpenAPIOperation, inputSchemaJSON []byte, args map[string]any, responseBody string) string {
	var response strings.Builder

	response.WriteString("RESOURCE NOT FOUND (404): The requested resource could not be found.\n\n")

	// Operation context
	response.WriteString(fmt.Sprintf("OPERATION: %s", op.OperationID))
	if op.Summary != "" {
		response.WriteString(fmt.Sprintf(" - %s", op.Summary))
	}
	response.WriteString("\n")
	response.WriteString(fmt.Sprintf("PATH: %s %s\n\n", strings.ToUpper(op.Method), op.Path))

	// Analyze current arguments
	if len(args) > 0 {
		response.WriteString("YOUR CURRENT ARGUMENTS:\n")
		argsJSON, _ := json.MarshalIndent(args, "", "  ")
		response.WriteString(string(argsJSON))
		response.WriteString("\n\n")
	}

	// Parse path parameters to help with troubleshooting
	var pathParams []string
	for _, paramRef := range op.Parameters {
		if paramRef != nil && paramRef.Value != nil && paramRef.Value.In == "path" {
			pathParams = append(pathParams, paramRef.Value.Name)
		}
	}

	if len(pathParams) > 0 {
		response.WriteString("PATH PARAMETERS IN THIS ENDPOINT:\n")
		for _, param := range pathParams {
			value := "NOT_PROVIDED"
			if val, ok := args[param]; ok {
				value = fmt.Sprintf("%v", val)
			}
			response.WriteString(fmt.Sprintf("• %s: %s\n", param, value))
		}
		response.WriteString("\n")
	}

	// Server error details if available
	if responseBody != "" {
		response.WriteString("SERVER ERROR DETAILS:\n")
		response.WriteString(responseBody)
		response.WriteString("\n\n")
	}

	response.WriteString("TROUBLESHOOTING STEPS:\n")
	response.WriteString("1. Verify all path parameters are correct and exist:\n")
	if len(pathParams) > 0 {
		for _, param := range pathParams {
			response.WriteString(fmt.Sprintf("   - Check that %s exists and is accessible\n", param))
		}
	} else {
		response.WriteString("   - Verify the endpoint path is correct\n")
	}
	response.WriteString("2. Ensure you're using the correct resource identifiers\n")
	response.WriteString("3. Check if the resource was recently deleted or moved\n")
	response.WriteString("4. Verify you have permission to access this resource\n")
	response.WriteString("5. Try listing resources first to find valid identifiers\n")
	response.WriteString("6. Check the API documentation for correct endpoint paths\n")
	response.WriteString("7. Ensure you're using the correct API base URL\n")

	return response.String()
}

// generateAI5xxErrorResponse creates comprehensive, AI-optimized error response for server errors
func generateAI5xxErrorResponse(op OpenAPIOperation, inputSchemaJSON []byte, args map[string]any, responseBody string, statusCode int) string {
	var response strings.Builder

	response.WriteString(fmt.Sprintf("SERVER ERROR (%d): The server encountered an error processing your request.\n\n", statusCode))

	// Operation context
	response.WriteString(fmt.Sprintf("OPERATION: %s", op.OperationID))
	if op.Summary != "" {
		response.WriteString(fmt.Sprintf(" - %s", op.Summary))
	}
	response.WriteString("\n\n")

	// Categorize the server error
	if statusCode == 500 {
		response.WriteString("ERROR TYPE: Internal Server Error\n")
		response.WriteString("This indicates a problem with the server's code or configuration.\n\n")
	} else if statusCode == 502 {
		response.WriteString("ERROR TYPE: Bad Gateway\n")
		response.WriteString("The server received an invalid response from an upstream server.\n\n")
	} else if statusCode == 503 {
		response.WriteString("ERROR TYPE: Service Unavailable\n")
		response.WriteString("The server is temporarily unable to handle the request.\n\n")
	} else if statusCode == 504 {
		response.WriteString("ERROR TYPE: Gateway Timeout\n")
		response.WriteString("The server didn't receive a timely response from an upstream server.\n\n")
	} else {
		response.WriteString(fmt.Sprintf("ERROR TYPE: Server Error (%d)\n", statusCode))
		response.WriteString("An unexpected server-side error occurred.\n\n")
	}

	// Server error details if available
	if responseBody != "" {
		response.WriteString("SERVER ERROR DETAILS:\n")
		response.WriteString(responseBody)
		response.WriteString("\n\n")
	}

	// Analyze current arguments for potential issues
	if len(args) > 0 {
		response.WriteString("YOUR REQUEST DETAILS:\n")
		argsJSON, _ := json.MarshalIndent(args, "", "  ")
		response.WriteString(string(argsJSON))
		response.WriteString("\n\n")
	}

	response.WriteString("IMMEDIATE ACTIONS:\n")
	if statusCode == 500 {
		response.WriteString("1. Retry the request after a short delay (server issue)\n")
		response.WriteString("2. Check if the request data is valid and within expected limits\n")
		response.WriteString("3. Report the error to the API provider with request details\n")
	} else if statusCode == 502 || statusCode == 503 || statusCode == 504 {
		response.WriteString("1. Wait and retry after a few seconds (temporary issue)\n")
		response.WriteString("2. Check the API status page for known outages\n")
		response.WriteString("3. Implement exponential backoff for retries\n")
	} else {
		response.WriteString("1. Retry the request after a brief delay\n")
		response.WriteString("2. Check if this is a known issue with the API\n")
	}

	response.WriteString("\nTROUBLESHOoting STEPS:\n")
	response.WriteString("1. Verify your request parameters are valid and properly formatted\n")
	response.WriteString("2. Check for any size limits on request data\n")
	response.WriteString("3. Ensure you're not hitting rate limits\n")
	response.WriteString("4. Try with a simpler request to isolate the issue\n")
	response.WriteString("5. Check the API's status page or documentation for known issues\n")
	response.WriteString("6. Monitor if the error persists or is intermittent\n")
	response.WriteString("7. Contact the API provider's support with error details\n")

	response.WriteString("\nRETRY STRATEGY:\n")
	response.WriteString("• Wait 1-2 seconds and retry once\n")
	response.WriteString("• If it fails again, wait longer (exponential backoff)\n")
	response.WriteString("• Maximum 3-5 retry attempts\n")
	response.WriteString("• Report persistent errors to the API provider\n")

	// Add tool usage information for AI agents
	var schemaObj map[string]any
	_ = json.Unmarshal(inputSchemaJSON, &schemaObj)

	if properties, ok := schemaObj["properties"].(map[string]any); ok && len(properties) > 0 {
		response.WriteString("\nTOOL USAGE INFORMATION:\n")
		response.WriteString(fmt.Sprintf("Tool Name: %s\n", op.OperationID))

		// Show required parameters
		if required, ok := schemaObj["required"].([]any); ok && len(required) > 0 {
			response.WriteString("Required Parameters (mandatory for all calls):\n")
			for _, req := range required {
				if reqStr, ok := req.(string); ok {
					if prop, ok := properties[reqStr].(map[string]any); ok {
						response.WriteString(fmt.Sprintf("  - %s", reqStr))
						if typeStr, ok := prop["type"].(string); ok {
							response.WriteString(fmt.Sprintf(" (%s)", typeStr))
						}
						if desc, ok := prop["description"].(string); ok && desc != "" {
							response.WriteString(fmt.Sprintf(": %s", desc))
						}
						response.WriteString(" [MANDATORY]")
						response.WriteString("\n")
					}
				}
			}
		}

		// Generate example usage with correct parameters
		response.WriteString("\nExample Usage (retry with these correct parameters):\n")
		exampleArgs := map[string]any{}

		// Add required parameters to example
		if required, ok := schemaObj["required"].([]any); ok {
			for _, req := range required {
				if reqStr, ok := req.(string); ok {
					if prop, ok := properties[reqStr].(map[string]any); ok {
						exampleArgs[reqStr] = generateExampleValue(prop)
					}
				}
			}
		}

		// Add a few optional parameters for completeness
		count := 0
		for paramName, paramDef := range properties {
			if _, exists := exampleArgs[paramName]; !exists && count < 2 {
				if prop, ok := paramDef.(map[string]any); ok {
					exampleArgs[paramName] = generateExampleValue(prop)
					count++
				}
			}
		}

		exampleJSON, _ := json.MarshalIndent(exampleArgs, "", "  ")
		response.WriteString(fmt.Sprintf("call %s %s\n", op.OperationID, string(exampleJSON)))
	}

	return response.String()
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
func RegisterOpenAPITools(server *mcpserver.MCPServer, ops []OpenAPIOperation, doc *openapi3.T, opts *ToolGenOptions) []string {
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

		annotations := mcp.ToolAnnotation{}
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

		tool := mcp.NewToolWithRawSchema(name, desc, inputSchemaJSON)
		tool.Annotations = annotations
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

		server.AddTool(tool, toolHandler(name, op, doc, inputSchemaJSON, baseURLs, opts != nil && opts.ConfirmDangerousActions))

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
		tool := mcp.NewToolWithRawSchema("externalDocs", desc, inputSchemaJSON)
		tool.Annotations = mcp.ToolAnnotation{}
		if opts != nil && opts.Version != "" {
			tool.Annotations.Title = "OpenAPI " + opts.Version
		}
		server.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			info := "External documentation URL: " + doc.ExternalDocs.URL
			if doc.ExternalDocs.Description != "" {
				info += "\nDescription: " + doc.ExternalDocs.Description
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
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
		tool := mcp.NewToolWithRawSchema("info", desc, inputSchemaJSON)
		tool.Annotations = mcp.ToolAnnotation{}
		if opts != nil && opts.Version != "" {
			tool.Annotations.Title = "OpenAPI " + opts.Version
		}
		server.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
					mcp.TextContent{
						Type: "text",
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

		server.AddResource(timestampResource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			now := time.Now().Unix()
			content := fmt.Sprintf(`{"unix_timestamp": %d, "iso8601": "%s", "timezone": "%s"}`,
				now,
				time.Now().Format(time.RFC3339),
				time.Now().Format("MST"))

			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      timestampResource.URI,
					MIMEType: "application/json",
					Text:     content,
				},
			}, nil
		})
	}

	return toolNames
}
