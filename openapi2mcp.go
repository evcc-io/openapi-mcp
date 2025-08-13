// Package openapi2mcp provides functions to expose OpenAPI operations as MCP tools and servers.
// It enables loading OpenAPI specs, generating MCP tool schemas, and running MCP servers that proxy real HTTP calls.
package openapi2mcp

import (
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/jsonschema-go/jsonschema"
)

// OpenAPIOperation describes a single OpenAPI operation to be mapped to an MCP tool.
// It includes the operation's ID, summary, description, HTTP path/method, parameters, request body, and tags.
type OpenAPIOperation struct {
	OperationID string
	Summary     string
	Description string
	Path        string
	Method      string
	Parameters  openapi3.Parameters
	RequestBody *openapi3.RequestBodyRef
	Tags        []string
	Security    openapi3.SecurityRequirements
}

// ToolGenOptions controls tool generation and output for OpenAPI-MCP conversion.
//
// NameFormat: function to format tool names (e.g., strings.ToLower)
// TagFilter: only include operations with at least one of these tags (if non-empty)
// DryRun: if true, only print the generated tool schemas, don't register
// PrettyPrint: if true, pretty-print the output
// Version: version string to embed in tool annotations
// PostProcessSchema: optional hook to modify each tool's input schema before registration/output
// ConfirmDangerousActions: if true (default), require confirmation for PUT/POST/DELETE tools
//
//	func(toolName string, schema jsonschema.Schema) jsonschema.Schema
type ToolGenOptions struct {
	NameFormat              func(string) string
	TagFilter               []string
	DryRun                  bool
	PrettyPrint             bool
	Version                 string
	PostProcessSchema       func(toolName string, schema jsonschema.Schema) jsonschema.Schema
	ConfirmDangerousActions bool // if true, add confirmation prompt for dangerous actions
	RequestHandler          func(req *http.Request) (*http.Response, error)
}
