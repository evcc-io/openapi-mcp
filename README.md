# <img src="https://raw.githubusercontent.com/jedisct1/openapi-mcp/main/.github/banner.png" alt="openapi-mcp" width="600"/>

# openapi-mcp

> **Expose any OpenAPI 3.x API as a robust, agent-friendly MCP tool server in seconds!**

[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)](https://golang.org/dl/)
[![Build Status](https://img.shields.io/github/actions/workflow/status/jedisct1/openapi-mcp/ci.yml?branch=main)](https://github.com/jedisct1/openapi-mcp/actions)
[![License](https://img.shields.io/github/license/jedisct1/openapi-mcp)](LICENSE)
[![GoDoc](https://pkg.go.dev/badge/github.com/jedisct1/openapi-mcp/pkg/openapi2mcp.svg)](https://pkg.go.dev/github.com/jedisct1/openapi-mcp/pkg/openapi2mcp)

---

**openapi-mcp** transforms any OpenAPI 3.x specification into a powerful, AI-friendly MCP (Model Context Protocol) tool server. In seconds, it validates your OpenAPI spec, generates MCP tools for each operation, and starts serving through stdio or HTTP with structured, machine-readable output.

## 📋 Table of Contents

- [](#)
- [openapi-mcp](#openapi-mcp)
  - [📋 Table of Contents](#-table-of-contents)
  - [✨ Features](#-features)
  - [🤖 AI Agent Integration](#-ai-agent-integration)
  - [🔧 Installation](#-installation)
    - [Prerequisites](#prerequisites)
    - [Build from Source](#build-from-source)
  - [⚡ Quick Start](#-quick-start)
    - [1. Run the MCP Server](#1-run-the-mcp-server)
    - [2. Use the Interactive Client](#2-use-the-interactive-client)
  - [🔒 Authentication](#-authentication)
  - [🛠️ Usage Examples](#️-usage-examples)
    - [Integration with AI Code Editors](#integration-with-ai-code-editors)
    - [OpenAPI Validation and Linting](#openapi-validation-and-linting)
      - [HTTP API for Validation and Linting](#http-api-for-validation-and-linting)
    - [Dry Run (Preview Tools as JSON)](#dry-run-preview-tools-as-json)
    - [Generate Documentation](#generate-documentation)
    - [Filter Operations by Tag, Description, or Function List](#filter-operations-by-tag-description-or-function-list)
    - [Include/Exclude Operations by Description](#includeexclude-operations-by-description)
    - [Print Summary](#print-summary)
    - [Post-Process Schema with External Command](#post-process-schema-with-external-command)
    - [Disable Confirmation for Dangerous Actions](#disable-confirmation-for-dangerous-actions)
  - [🎮 Command-Line Options](#-command-line-options)
    - [Commands](#commands)
    - [Flags](#flags)
  - [📚 Library Usage](#-library-usage)
  - [📊 Output Structure](#-output-structure)
  - [🛡️ Safety Features](#️-safety-features)
  - [📝 Documentation Generation](#-documentation-generation)
  - [🙌 Contributing](#-contributing)
  - [📄 License](#-license)

## ✨ Features

- **Instant API to MCP Conversion**: Parses any OpenAPI 3.x YAML/JSON spec and generates MCP tools
- **Multiple Transport Options**: Supports stdio (default) and HTTP server modes
- **Complete Parameter Support**: Path, query, header, cookie, and body parameters
- **Authentication**: API key, Bearer token, Basic auth, and OAuth2 support
- **Structured Output**: All responses have consistent, well-structured formats with type information
- **Validation & Linting**: Comprehensive OpenAPI validation and linting with actionable suggestions
  - `validate` command for critical issues (missing operationIds, schema errors)
  - `lint` command for best practices (summaries, descriptions, tags, parameter recommendations)
- **Safety Features**: Confirmation required for dangerous operations (PUT/POST/DELETE)
- **Documentation**: Built-in documentation generation in Markdown or HTML
- **AI-Optimized**: Unique features specifically designed to enhance AI agent interactions:
  - Consistent output structures with OutputFormat and OutputType for reliable parsing
  - Rich machine-readable schema information with constraints and examples
  - Streamlined, agent-friendly response format with minimal verbosity
  - Intelligent error messages with suggestions for correction
  - Automatic handling of authentication, pagination, and complex data structures
- **Interactive Client**: Includes an MCP client with readline support and command history
- **Flexible Configuration**: Environment variables or command-line flags
- **CI/Testing Support**: Summary options, exit codes, and dry-run mode

## 🤖 AI Agent Integration

openapi-mcp is designed for seamless integration with AI coding agents, LLMs, and automation tools with unique features that set it apart from other API-to-tool converters:

- **Structured JSON Responses**: Every response includes `OutputFormat` and `OutputType` fields for consistent parsing
- **Rich Schema Information**: All tools provide detailed parameter constraints and examples that help AI agents understand API requirements
- **Actionable Error Messages**: Validation errors include detailed information and suggestions that guide agents toward correct usage
- **Safety Confirmations**: Standardized confirmation workflow for dangerous operations prevents unintended consequences
- **Self-Describing API**: The `describe` tool provides complete, machine-readable documentation for all operations
- **Minimal Verbosity**: No redundant warnings or messages to confuse agents—outputs are optimized for machine consumption
- **Smart Parameter Handling**: Automatic conversion between OpenAPI parameter types and MCP tool parameters
- **Contextual Examples**: Every tool includes context-aware examples based on the OpenAPI specification
- **Intelligent Default Values**: Sensible defaults are provided whenever possible to simplify API usage

## 🔧 Installation

### Prerequisites

- Go 1.21+
- An OpenAPI 3.x YAML or JSON specification file

### Build from Source

```sh
# Clone the repository
git clone <repo-url>
cd openapi-mcp

# Build the binaries
make

# This will create:
# - bin/openapi-mcp (main tool)
# - bin/mcp-client (interactive client)
```

## ⚡ Quick Start

### 1. Run the MCP Server

```sh
# Basic usage (stdio mode)
bin/openapi-mcp examples/fastly-openapi-mcp.yaml

# With API key
API_KEY=your_api_key bin/openapi-mcp examples/fastly-openapi-mcp.yaml

# As HTTP server
bin/openapi-mcp --http=:8080 examples/fastly-openapi-mcp.yaml

# Override base URL
bin/openapi-mcp --base-url=https://api.example.com examples/fastly-openapi-mcp.yaml
```

### Hosted Authenticated Spec Example

```sh
# Download Xquik's hosted OpenAPI document, then expose it as MCP tools.
curl -fsSL https://xquik.com/openapi.json -o /tmp/xquik-openapi.json
BEARER_TOKEN="$XQUIK_API_KEY" bin/openapi-mcp /tmp/xquik-openapi.json
```

Use this pattern for APIs that publish their OpenAPI document separately from the authenticated REST calls.

### 2. Use the Interactive Client

```sh
# Start the client (connects to openapi-mcp via stdio)
bin/mcp-client bin/openapi-mcp examples/fastly-openapi-mcp.yaml

# Client commands
mcp> list                              # List available tools
mcp> schema <tool-name>                # Show tool schema
mcp> call <tool-name> {arg1: value1}   # Call a tool with arguments
mcp> describe                          # Get full API documentation
```

## 🔒 Authentication

openapi-mcp supports all standard OpenAPI authentication methods via command-line flags, environment variables, or HTTP headers:

### Command-Line Flags & Environment Variables

```sh
# API Key authentication
bin/openapi-mcp --api-key=your_api_key examples/fastly-openapi-mcp.yaml
# or use environment variable
API_KEY=your_api_key bin/openapi-mcp examples/fastly-openapi-mcp.yaml

# Bearer token / OAuth2
bin/openapi-mcp --bearer-token=your_token examples/fastly-openapi-mcp.yaml
# or use environment variable
BEARER_TOKEN=your_token bin/openapi-mcp examples/fastly-openapi-mcp.yaml

# Basic authentication
bin/openapi-mcp --basic-auth=username:password examples/fastly-openapi-mcp.yaml
# or use environment variable
BASIC_AUTH=username:password bin/openapi-mcp examples/fastly-openapi-mcp.yaml
```

### HTTP Header Authentication (HTTP Mode Only)

When using HTTP mode (`--http=:8080`), you can provide authentication via HTTP headers in your requests:

```sh
# API Key via headers
curl -H "X-API-Key: your_api_key" http://localhost:8080/mcp -d '...'
curl -H "Api-Key: your_api_key" http://localhost:8080/mcp -d '...'

# Bearer token
curl -H "Authorization: Bearer your_token" http://localhost:8080/mcp -d '...'

# Basic authentication
curl -H "Authorization: Basic base64_credentials" http://localhost:8080/mcp -d '...'
```

**Supported Authentication Headers:**
- `X-API-Key` or `Api-Key` - for API key authentication
- `Authorization: Bearer <token>` - for OAuth2/Bearer token authentication
- `Authorization: Basic <credentials>` - for Basic authentication

Authentication is automatically applied to the appropriate endpoints as defined in your OpenAPI spec. HTTP header authentication takes precedence over environment variables for the duration of each request.

When using HTTP mode, openapi-mcp serves a StreamableHTTP-based MCP server by default. For developers building HTTP clients, the package provides convenient URL helper functions:

```go
import "github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"

// Get the Streamable HTTP endpoint URL
streamableURL := openapi2mcp.GetStreamableHTTPURL(":8080", "/mcp")
// Returns: "http://localhost:8080/mcp"

// For SSE mode (when using --http-transport=sse), you can use:
sseURL := openapi2mcp.GetSSEURL(":8080", "/mcp")
// Returns: "http://localhost:8080/mcp/sse"

messageURL := openapi2mcp.GetMessageURL(":8080", "/mcp", sessionID)
// Returns: "http://localhost:8080/mcp/message?sessionId=<sessionID>"
```

**StreamableHTTP Client Connection Flow:**
1. Send POST requests to the Streamable HTTP endpoint for requests/notifications
2. Send GET requests to the same endpoint to listen for notifications
3. Send DELETE requests to terminate the session

**Example with curl:**
```sh
# Step 1: Initialize the session
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}'

# The response will include a Mcp-Session-Id header

# Step 2: Send JSON-RPC requests
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <session-id>" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

# Step 3: Listen for notifications
curl -N http://localhost:8080/mcp \
  -H "Mcp-Session-Id: <session-id>"
```

**SSE Client Connection Flow (when using --http-transport=sse):**
1. Connect to the SSE endpoint to establish a persistent connection
2. Receive an `endpoint` event containing the session ID
3. Send JSON-RPC requests to the message endpoint using the session ID
4. Receive responses and notifications via the SSE stream

**Example with curl (SSE mode):**
```sh
# Step 1: Connect to SSE endpoint (keep connection open)
curl -N http://localhost:8080/mcp/sse

# Output: event: endpoint
#         data: /mcp/message?sessionId=<session-id>

# Step 2: Send JSON-RPC requests (in another terminal)
curl -X POST http://localhost:8080/mcp/message?sessionId=<session-id> \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## 🛠️ Usage Examples

### Integration with AI Code Editors

You can easily integrate openapi-mcp with AI code editors that support MCP tools, such as Roo Code:

```json
{
    "fastly": {
        "command": "/opt/bin/openapi-mcp",
        "args": [
            "-api-key",
            "YOUR_API_KEY",
            "/opt/etc/openapi/fastly-openapi-mcp.yaml"
        ]
    }
}
```

Add this configuration to your editor's MCP tools configuration to provide AI assistants with direct access to the API. The assistant can then discover and use the API operations without additional setup.

### OpenAPI Validation and Linting

openapi-mcp includes powerful OpenAPI validation and linting capabilities to help you improve your API specifications:

```sh
# Validate OpenAPI spec and check for critical issues
bin/openapi-mcp validate examples/fastly-openapi-mcp.yaml

# Comprehensive linting with detailed suggestions
bin/openapi-mcp lint examples/fastly-openapi-mcp.yaml

# Start HTTP validation service
bin/openapi-mcp --http=:8080 validate

# Start HTTP linting service
bin/openapi-mcp --http=:8080 lint
```

The **validate** command performs essential checks:
- Missing `operationId` fields (required for MCP tool generation)
- Schema validation errors
- Basic structural issues

The **lint** command provides comprehensive analysis with suggestions for:
- Missing summaries and descriptions
- Untagged operations
- Parameter naming and type recommendations
- Security scheme validation
- Best practices for API design

Both commands exit with non-zero status codes when issues are found, making them perfect for CI/CD pipelines.

#### HTTP API for Validation and Linting

Both validate and lint commands can be run as HTTP services using the `--http` flag, allowing you to validate OpenAPI specs via REST API. Note that these endpoints are only available when using the `validate` or `lint` commands, not during normal MCP server operation:

```sh
# Start validation HTTP service
bin/openapi-mcp --http=:8080 validate

# Start linting HTTP service
bin/openapi-mcp --http=:8080 lint
```

**API Endpoints:**

- `POST /validate` - Validate OpenAPI specs for critical issues
- `POST /lint` - Comprehensive linting with detailed suggestions
- `GET /health` - Health check endpoint

**Request Format:**
```json
{
  "openapi_spec": "openapi: 3.0.0\ninfo:\n  title: My API\n  version: 1.0.0\npaths: {}"
}
```

**Response Format:**
```json
{
  "success": false,
  "error_count": 1,
  "warning_count": 2,
  "issues": [
    {
      "type": "error",
      "message": "Operation missing operationId",
      "suggestion": "Add an operationId field",
      "operation": "GET_/users",
      "path": "/users",
      "method": "GET"
    }
  ],
  "summary": "OpenAPI linting completed with issues: 1 errors, 2 warnings."
}
```

**Example Usage:**
```sh
curl -X POST http://localhost:8080/lint \
  -H "Content-Type: application/json" \
  -d '{"openapi_spec": "..."}'
```

### Dry Run (Preview Tools as JSON)

```sh
bin/openapi-mcp --dry-run examples/fastly-openapi-mcp.yaml
```

### Generate Documentation

```sh
bin/openapi-mcp --doc=tools.md examples/fastly-openapi-mcp.yaml
```

### Filter Operations by Tag, Description, or Function List

```sh
bin/openapi-mcp filter --tag=admin examples/fastly-openapi-mcp.yaml
bin/openapi-mcp filter --include-desc-regex="user|account" examples/fastly-openapi-mcp.yaml
bin/openapi-mcp filter --exclude-desc-regex="deprecated" examples/fastly-openapi-mcp.yaml
bin/openapi-mcp filter --function-list-file=funcs.txt examples/fastly-openapi-mcp.yaml
```

You can use `--function-list-file=funcs.txt` to restrict the output to only the operations whose `operationId` is listed (one per line) in the given file. This filter is applied after tag and description filters.

### Print Summary

```sh
bin/openapi-mcp --summary --dry-run examples/fastly-openapi-mcp.yaml
```

### Post-Process Schema with External Command

```sh
bin/openapi-mcp --doc=tools.md --post-hook-cmd='jq . | tee /tmp/filtered.json' examples/fastly-openapi-mcp.yaml
```

### Disable Confirmation for Dangerous Actions

```sh
bin/openapi-mcp --no-confirm-dangerous examples/fastly-openapi-mcp.yaml
```

## 🎮 Command-Line Options

### Commands

| Command           | Description                                                                                                                    |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `validate <spec>` | Validate OpenAPI spec and report critical issues (missing operationIds, schema errors)                                         |
| `lint <spec>`     | Comprehensive linting with detailed suggestions for best practices                                                             |
| `filter <spec>`   | Output a filtered list of operations as JSON, applying `--tag`, `--include-desc-regex`, `--exclude-desc-regex`, and `--function-list-file` (no server) |

### Flags

| Flag                     | Environment Variable | Description                                              |
| ------------------------ | -------------------- | -------------------------------------------------------- |
| `--api-key`              | `API_KEY`            | API key for authentication                               |
| `--bearer-token`         | `BEARER_TOKEN`       | Bearer token for Authorization header                    |
| `--basic-auth`           | `BASIC_AUTH`         | Basic auth credentials (user:pass)                       |
| `--base-url`             | `OPENAPI_BASE_URL`   | Override base URL for HTTP calls                         |
| `--http`                 | -                    | Serve MCP over HTTP instead of stdio                     |
| `--tag`                  | `OPENAPI_TAG`        | Only include operations with this tag                    |
| `--include-desc-regex`   | `INCLUDE_DESC_REGEX` | Only include APIs matching regex                         |
| `--exclude-desc-regex`   | `EXCLUDE_DESC_REGEX` | Exclude APIs matching regex                              |
| `--dry-run`              | -                    | Print tool schemas as JSON and exit                      |
| `--summary`              | -                    | Print operation count summary                            |
| `--doc`                  | -                    | Generate documentation file                              |
| `--doc-format`           | -                    | Documentation format (markdown or html)                  |
| `--post-hook-cmd`        | -                    | Command to post-process schema JSON                      |
| `--no-confirm-dangerous` | -                    | Disable confirmation for dangerous actions               |
| `--extended`             | -                    | Enable human-friendly output (default is agent-friendly) |
| `--function-list-file`   | -                    | Only include operations whose operationId is listed (one per line) in the given file (for filter command) |

## 📚 Library Usage

openapi-mcp can be imported as a Go module in your projects:

```go
package main

import (
        "github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
)

func main() {
        // Load OpenAPI spec
        doc, err := openapi2mcp.LoadOpenAPISpec("openapi.yaml")
        if err != nil {
                panic(err)
        }

        // Create MCP server
        srv := openapi2mcp.NewServer("myapi", doc.Info.Version, doc)

        // Serve over HTTP
        if err := openapi2mcp.ServeHTTP(srv, ":8080"); err != nil {
                panic(err)
        }

        // Or serve over stdio
        // if err := openapi2mcp.ServeStdio(srv); err != nil {
        //    panic(err)
        // }
}
```

See [GoDoc](https://pkg.go.dev/github.com/jedisct1/openapi-mcp/pkg/openapi2mcp) for complete API documentation.

## 📊 Output Structure

All tool results include consistent structure for machine readability:

```json
{
  "OutputFormat": "structured",
  "OutputType": "json",
  "type": "api_response",
  "data": {
    // API-specific response data
  },
  "metadata": {
    "status_code": 200,
    "headers": {
      // Response headers
    }
  }
}
```

For errors, you'll receive:

```json
{
  "OutputFormat": "structured",
  "OutputType": "json",
  "type": "error",
  "error": {
    "code": "validation_error",
    "message": "Invalid parameter",
    "details": {
      "field": "username",
      "reason": "required field missing"
    },
    "suggestions": [
      "Provide a username parameter"
    ]
  }
}
```

## 🛡️ Safety Features

For any operation that performs a PUT, POST, or DELETE, openapi-mcp requires confirmation:

```json
{
  "type": "confirmation_request",
  "confirmation_required": true,
  "message": "This action is irreversible. Proceed?",
  "action": "delete_resource"
}
```

To proceed, retry the call with:

```json
{
  "original_parameters": {},
  "__confirmed": true
}
```

This confirmation workflow can be disabled with `--no-confirm-dangerous`.

## 📝 Documentation Generation

Generate comprehensive documentation for all tools:

```sh
# Markdown documentation
bin/openapi-mcp --doc=tools.md examples/fastly-openapi-mcp.yaml

# HTML documentation
bin/openapi-mcp --doc=tools.html --doc-format=html examples/fastly-openapi-mcp.yaml
```

The documentation includes:
- Complete tool schemas with parameter types, constraints, and descriptions
- Example calls for each tool
- Response formats and examples
- Authentication requirements

## 🙌 Contributing

Contributions are welcome! Please open an issue or pull request on GitHub.

1. Fork the repository
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Run tests (`go test ./...`)
5. Push to the branch (`git push origin my-new-feature`)
6. Create a new Pull Request

## 📄 License

This project is licensed under the [MIT License](LICENSE).
