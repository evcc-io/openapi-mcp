package openapi2mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/xeipuuv/gojsonschema"
)

func toolHandler(name string, op OpenAPIOperation, doc *openapi3.T, inputSchemaJSON []byte, baseURLs []string, confirmDangerousActions bool) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			args = map[string]any{}
		}

		// Build parameter name mapping for escaped parameter names
		paramNameMapping := buildParameterNameMapping(op.Parameters)

		// Validate arguments against inputSchema
		argsJSON, _ := json.Marshal(args)
		schemaLoader := gojsonschema.NewBytesLoader(inputSchemaJSON)
		argsLoader := gojsonschema.NewBytesLoader(argsJSON)
		result, err := gojsonschema.Validate(schemaLoader, argsLoader)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: "Validation error: " + err.Error(),
					},
				},
				IsError: true,
			}, nil
		}

		if !result.Valid() {
			var missingFields []string
			var suggestions []string
			errMsgs := ""
			// Parse the input schema for property descriptions
			var schemaObj map[string]any
			_ = json.Unmarshal(inputSchemaJSON, &schemaObj)
			properties, _ := schemaObj["properties"].(map[string]any)
			for _, verr := range result.Errors() {
				errMsg := ""

				// Handle different validation error types with plain text messages
				switch verr.Type() {
				case "required":
					if missingRaw, ok := verr.Details()["property"]; ok {
						if missing, ok := missingRaw.(string); ok {
							missingFields = append(missingFields, missing)
							if prop, ok := properties[missing].(map[string]any); ok {
								desc, _ := prop["description"].(string)
								typeStr, _ := prop["type"].(string)
								info := ""
								if desc != "" {
									info = desc
								}
								if typeStr != "" {
									if info != "" {
										info += ", "
									}
									info += "type: " + typeStr
								}
								if info != "" {
									errMsg = "Missing required parameter: '" + missing + "' (" + info + "). Please provide this parameter."
								} else {
									errMsg = "Missing required parameter: '" + missing + "'"
								}
							} else {
								errMsg = "Missing required parameter: '" + missing + "'"
							}
						}
					}
				case "invalid_type":
					// Convert "Invalid type. Expected: string, given: integer" to plain text
					errMsg = verr.String()
				case "enum":
					// Convert enum validation errors to plain text
					errMsg = verr.String()
				case "invalid_union", "one_of", "any_of":
					// Convert union/oneOf/anyOf errors to plain text
					errMsg = "Invalid value. " + verr.String()
				default:
					// For any other validation error types, ensure it's plain text
					errMsg = verr.String()
				}

				if errMsg != "" {
					errMsgs += errMsg + "\n"
				}
			}
			// Suggest a retry with an example argument set
			exampleArgs := map[string]any{}
			for k, v := range properties {
				if prop, ok := v.(map[string]any); ok {
					typeStr, _ := prop["type"].(string)
					switch typeStr {
					case "string":
						exampleArgs[k] = "example"
					case "number":
						exampleArgs[k] = 123.45
					case "integer":
						exampleArgs[k] = 123
					case "boolean":
						exampleArgs[k] = true
					case "array":
						exampleArgs[k] = []any{"item1", "item2"}
					case "object":
						exampleArgs[k] = map[string]any{"key": "value"}
					default:
						exampleArgs[k] = nil
					}
				} else {
					exampleArgs[k] = nil
				}
			}
			suggestionStr := "Try again with: call " + name + " "
			exampleJSON, _ := json.Marshal(exampleArgs)
			suggestionStr += string(exampleJSON)
			suggestions = append(suggestions, suggestionStr)

			// Create a simple text error message
			errorText := strings.TrimSpace(errMsgs)
			if len(suggestions) > 0 {
				errorText += "\n\n" + strings.Join(suggestions, "\n")
			}

			return mcp.NewToolResultError(errorText), nil
		}

		// Build URL path with path parameters
		path := op.Path
		for _, paramRef := range op.Parameters {
			if paramRef == nil || paramRef.Value == nil {
				continue
			}
			p := paramRef.Value
			if p.In == "path" {
				if val, ok := getParameterValue(args, p.Name, paramNameMapping); ok {
					// Check if parameter is integer type
					isInteger := false
					if p.Schema != nil && p.Schema.Value != nil && p.Schema.Value.Type != nil {
						isInteger = p.Schema.Value.Type.Is("integer")
					}
					path = strings.ReplaceAll(path, "{"+p.Name+"}", formatParameterValue(val, isInteger))
				}
			}
		}

		// Build query parameters
		query := url.Values{}
		for _, paramRef := range op.Parameters {
			if paramRef == nil || paramRef.Value == nil {
				continue
			}
			p := paramRef.Value
			if p.In == "query" {
				if val, ok := getParameterValue(args, p.Name, paramNameMapping); ok {
					// Check if parameter is integer type
					isInteger := false
					if p.Schema != nil && p.Schema.Value != nil && p.Schema.Value.Type != nil {
						isInteger = p.Schema.Value.Type.Is("integer")
					}
					query.Set(p.Name, formatParameterValue(val, isInteger))
				}
			}
		}

		// Pick a random baseURL for each call using the global rand
		baseURL := baseURLs[rand.Intn(len(baseURLs))]
		fullURL, err := url.JoinPath(baseURL, path)
		if err != nil {
			return nil, err
		}
		if len(query) > 0 {
			fullURL += "?" + query.Encode()
		}

		// Build request body if needed
		var body []byte
		var requestContentType string
		if op.RequestBody != nil && op.RequestBody.Value != nil {
			// Check for application/json first, then application/vnd.api+json (including with parameters)
			mt := getContentByType(op.RequestBody.Value.Content, "application/json")
			if mt != nil {
				requestContentType = "application/json"
			} else {
				mt = getContentByType(op.RequestBody.Value.Content, "application/vnd.api+json")
				if mt != nil {
					requestContentType = "application/vnd.api+json"
				}
			}

			if mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
				if v, ok := args["requestBody"]; ok && v != nil {
					body, _ = json.Marshal(v)
				}
			}
		}

		// Build HTTP request
		method := strings.ToUpper(op.Method)
		httpReq, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if len(body) > 0 && requestContentType != "" {
			httpReq.Header.Set("Content-Type", requestContentType)
		}

		// Set Accept header to accept both JSON and JSON:API responses
		httpReq.Header.Set("Accept", "application/json, application/vnd.api+json")

		// --- AUTH HANDLING: inject per-operation security requirements ---
		// For each security requirement object, try to satisfy at least one scheme
		securitySatisfied := false
		for _, secReq := range op.Security {
			for secName := range secReq {
				// TODO fulfill ALL requirements
				securitySatisfied = fulfillSecurity(secName, httpReq, doc)
			}
		}

		// If no security requirements, fallback to legacy env handling (for backward compatibility)
		if !securitySatisfied {
			apiKeyHeader := os.Getenv("API_KEY_HEADER")
			if apiKey := os.Getenv("API_KEY"); apiKey != "" && apiKeyHeader != "" {
				httpReq.Header.Set(apiKeyHeader, apiKey)
			}
			if bearer := os.Getenv("BEARER_TOKEN"); bearer != "" {
				httpReq.Header.Set("Authorization", "Bearer "+bearer)
			} else if basic := os.Getenv("BASIC_AUTH"); basic != "" {
				encoded := base64.StdEncoding.EncodeToString([]byte(basic))
				httpReq.Header.Set("Authorization", "Basic "+encoded)
			}
		}

		// Add header parameters
		for _, paramRef := range op.Parameters {
			if paramRef == nil || paramRef.Value == nil {
				continue
			}

			p := paramRef.Value
			if p.In == "header" {
				if val, ok := getParameterValue(args, p.Name, paramNameMapping); ok {
					// Check if parameter is integer type
					isInteger := false
					if p.Schema != nil && p.Schema.Value != nil && p.Schema.Value.Type != nil {
						isInteger = p.Schema.Value.Type.Is("integer")
					}
					httpReq.Header.Set(p.Name, formatParameterValue(val, isInteger))
				}
			}
		}

		// Add cookie parameters (RFC 6265)
		var cookiePairs []string
		for _, paramRef := range op.Parameters {
			if paramRef == nil || paramRef.Value == nil {
				continue
			}

			p := paramRef.Value
			if p.In == "cookie" {
				if val, ok := getParameterValue(args, p.Name, paramNameMapping); ok {
					// Check if parameter is integer type
					isInteger := false
					if p.Schema != nil && p.Schema.Value != nil && p.Schema.Value.Type != nil {
						isInteger = p.Schema.Value.Type.Is("integer")
					}
					cookiePairs = append(cookiePairs, fmt.Sprintf("%s=%s", p.Name, formatParameterValue(val, isInteger)))
				}
			}
		}

		if len(cookiePairs) > 0 {
			httpReq.Header.Set("Cookie", strings.Join(cookiePairs, "; "))
		}

		// Log HTTP request if logging is enabled
		if os.Getenv("MCP_LOG_HTTP") != "" || os.Getenv("DEBUG") != "" {
			logHTTPRequest(httpReq, body)
		}

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)

		// Log HTTP response if logging is enabled
		if os.Getenv("MCP_LOG_HTTP") != "" || os.Getenv("DEBUG") != "" {
			logHTTPResponse(resp, respBody)
		}

		contentType := resp.Header.Get("Content-Type")
		isJSON := strings.HasPrefix(contentType, "application/json") || strings.HasPrefix(contentType, "application/vnd.api+json")
		isText := strings.HasPrefix(contentType, "text/")
		isBinary := !isJSON && !isText

		// LLM-friendly error handling for non-2xx responses
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			opSummary := op.Summary
			if opSummary == "" {
				opSummary = op.Description
			}
			opDesc := op.Description
			suggestion := "Check the input parameters, authentication, and consult the tool schema. See the OpenAPI documentation for more details."
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				suggestion = generateAI401403ErrorResponse(op, inputSchemaJSON, args, string(respBody), resp.StatusCode)
			} else if resp.StatusCode == 404 {
				suggestion = generateAI404ErrorResponse(op, inputSchemaJSON, args, string(respBody))
			} else if resp.StatusCode == 400 {
				suggestion = generateAI400ErrorResponse(op, inputSchemaJSON, args, string(respBody))
			} else if resp.StatusCode >= 500 {
				suggestion = generateAI5xxErrorResponse(op, inputSchemaJSON, args, string(respBody), resp.StatusCode)
			}

			// For binary error responses, include base64 and mime type
			if isBinary {
				fileBase64 := base64.StdEncoding.EncodeToString(respBody)
				fileName := "file"
				if cd := resp.Header.Get("Content-Disposition"); cd != "" {
					if parts := strings.Split(cd, "filename="); len(parts) > 1 {
						fileName = strings.Trim(parts[1], `"`)
					}
				}
				errorObj := map[string]any{
					"type": "api_response",
					"error": map[string]any{
						"code":        "http_error",
						"http_status": resp.StatusCode,
						"message":     fmt.Sprintf("%s (HTTP %d)", http.StatusText(resp.StatusCode), resp.StatusCode),
						"details":     "Binary response (see file_base64)",
						"suggestion":  suggestion,
						"mime_type":   contentType,
						"file_base64": fileBase64,
						"file_name":   fileName,
						"operation": map[string]any{
							"id":          op.OperationID,
							"summary":     opSummary,
							"description": opDesc,
						},
					},
				}
				errorJSON, _ := json.MarshalIndent(errorObj, "", "  ")

				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "json",
							Text: string(errorJSON),
						},
					},
					IsError: true,
				}, nil
			}

			// Create a simple text error message
			errorText := fmt.Sprintf("HTTP %s %s\nError: %s (HTTP %d)", op.Method, fullURL, http.StatusText(resp.StatusCode), resp.StatusCode)
			if len(respBody) > 0 {
				errorText += "\nDetails: " + string(respBody)
			}
			if suggestion != "" {
				errorText += "\nSuggestion: " + suggestion
			}
			errorText += fmt.Sprintf("\nOperation: %s (%s)", op.OperationID, opSummary)

			return mcp.NewToolResultError(errorText), nil
		}

		// Handle binary/file responses for success
		if isBinary && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			fileBase64 := base64.StdEncoding.EncodeToString(respBody)
			fileName := "file"
			if cd := resp.Header.Get("Content-Disposition"); cd != "" {
				if parts := strings.Split(cd, "filename="); len(parts) > 1 {
					fileName = strings.Trim(parts[1], `"`)
				}
			}
			resultObj := map[string]any{
				"type":        "api_response",
				"http_status": resp.StatusCode,
				"mime_type":   contentType,
				"file_base64": fileBase64,
				"file_name":   fileName,
				"operation": map[string]any{
					"id":          op.OperationID,
					"summary":     op.Summary,
					"description": op.Description,
				},
			}
			resultJSON, _ := json.MarshalIndent(resultObj, "", "  ")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "json",
						Text: string(resultJSON),
					},
				},
			}, nil
		}

		// Always format the response as: HTTP <METHOD> <URL>\nStatus: <status>\nResponse:\n<respBody>
		respText := fmt.Sprintf("HTTP %s %s\nStatus: %d\nResponse:\n%s", op.Method, fullURL, resp.StatusCode, string(respBody))
		if args["stream"] == true {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: respText,
					},
				},
			}, nil
		}
		if confirmDangerousActions && (method == "PUT" || method == "POST" || method == "DELETE") {
			if _, confirmed := args["__confirmed"]; !confirmed {
				confirmText := fmt.Sprintf("⚠️  CONFIRMATION REQUIRED\n\nAction: %s\nThis action is irreversible. Proceed?\n\nTo confirm, retry the call with {\"__confirmed\": true} added to your arguments.", name)
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "text",
							Text: confirmText,
						},
					},
				}, nil
			}
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: respText,
				},
			},
		}, nil
	}
}

func fulfillSecurity(secName string, httpReq *http.Request, doc *openapi3.T) bool {
	if doc.Components != nil && doc.Components.SecuritySchemes != nil {
		if secSchemeRef, ok := doc.Components.SecuritySchemes[secName]; ok && secSchemeRef.Value != nil {
			secScheme := secSchemeRef.Value
			switch secScheme.Type {
			case "http":
				if secScheme.Scheme == "bearer" {
					if bearer := os.Getenv("BEARER_TOKEN"); bearer != "" {
						httpReq.Header.Set("Authorization", "Bearer "+bearer)
						return true
					}
				} else if secScheme.Scheme == "basic" {
					if basic := os.Getenv("BASIC_AUTH"); basic != "" {
						encoded := base64.StdEncoding.EncodeToString([]byte(basic))
						httpReq.Header.Set("Authorization", "Basic "+encoded)
						return true
					}
				}

			case "apiKey":
				if secScheme.In == "header" && secScheme.Name != "" {
					if apiKey := os.Getenv("API_KEY"); apiKey != "" {
						httpReq.Header.Set(secScheme.Name, apiKey)
						return true
					}
				} else if secScheme.In == "query" && secScheme.Name != "" {
					if apiKey := os.Getenv("API_KEY"); apiKey != "" {
						q := httpReq.URL.Query()
						q.Set(secScheme.Name, apiKey)
						httpReq.URL.RawQuery = q.Encode()
						return true
					}
				} else if secScheme.In == "cookie" && secScheme.Name != "" {
					if apiKey := os.Getenv("API_KEY"); apiKey != "" {
						cookie := httpReq.Header.Get("Cookie")
						if cookie != "" {
							cookie += "; "
						}
						cookie += secScheme.Name + "=" + apiKey
						httpReq.Header.Set("Cookie", cookie)
						return true
					}
				}

			case "oauth2":
				if bearer := os.Getenv("BEARER_TOKEN"); bearer != "" {
					httpReq.Header.Set("Authorization", "Bearer "+bearer)
					return true
				}
			}
		}
	}

	return false
}
