package openapi2mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
