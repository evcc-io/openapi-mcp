package openapi2mcp

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
)

func TestGenerateAIFriendlyDescription_WithJsonSchema(t *testing.T) {
	// Create a test operation
	op := OpenAPIOperation{
		OperationID: "testOperation",
		Summary:     "Test operation for API",
		Description: "This is a test operation that demonstrates the refactored description generation",
		Method:      "post",
		Path:        "/test/{id}",
	}

	// Create a jsonschema.Schema with various property types
	schema := jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"id": {
				Type:        "string",
				Description: "The unique identifier",
				Format:      "uuid",
			},
			"name": {
				Type:        "string",
				Description: "The name field",
			},
			"age": {
				Type:        "integer",
				Description: "Age in years",
			},
			"email": {
				Type:        "string",
				Format:      "email",
				Description: "Email address",
			},
			"status": {
				Type:        "string",
				Description: "Current status",
				Enum:        []any{"active", "inactive", "pending"},
			},
			"tags": {
				Type:        "array",
				Description: "List of tags",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
		},
		Required: []string{"id", "name"},
	}

	// Generate the description
	description := generateAIFriendlyDescription(op, schema)

	// Verify that the description contains expected content
	if !strings.Contains(description, "This is a test operation") {
		t.Error("Description should contain the operation description")
	}

	if !strings.Contains(description, "PARAMETERS:") {
		t.Error("Description should contain PARAMETERS section")
	}

	if !strings.Contains(description, "• Required:") {
		t.Error("Description should contain Required parameters section")
	}

	if !strings.Contains(description, "id (string)") {
		t.Error("Description should contain required id parameter")
	}

	if !strings.Contains(description, "name (string)") {
		t.Error("Description should contain required name parameter")
	}

	if !strings.Contains(description, "• Optional:") {
		t.Error("Description should contain Optional parameters section")
	}

	if !strings.Contains(description, "email (string): Email address") {
		t.Error("Description should contain optional email parameter with description")
	}

	if !strings.Contains(description, "[values: active, inactive, pending]") {
		t.Error("Description should contain enum values for status")
	}

	if !strings.Contains(description, "EXAMPLE: call testOperation") {
		t.Error("Description should contain example usage")
	}

	if !strings.Contains(description, "⚠️  SAFETY: This operation modifies data") {
		t.Error("Description should contain safety warning for POST operation")
	}
}

func TestGenerateExampleValueFromSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   *jsonschema.Schema
		expected any
	}{
		{
			name:     "string type",
			schema:   &jsonschema.Schema{Type: "string"},
			expected: "example_string",
		},
		{
			name:     "integer type",
			schema:   &jsonschema.Schema{Type: "integer"},
			expected: 123,
		},
		{
			name:     "boolean type",
			schema:   &jsonschema.Schema{Type: "boolean"},
			expected: true,
		},
		{
			name:     "email format",
			schema:   &jsonschema.Schema{Type: "string", Format: "email"},
			expected: "user@example.com",
		},
		{
			name:     "enum values",
			schema:   &jsonschema.Schema{Type: "string", Enum: []any{"option1", "option2"}},
			expected: "option1",
		},
		{
			name:     "with examples",
			schema:   &jsonschema.Schema{Type: "string", Examples: []any{"custom_example"}},
			expected: "custom_example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateExampleValueFromSchema(tt.schema)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGenerateExampleValueFromSchema_Nil(t *testing.T) {
	result := generateExampleValueFromSchema(nil)
	if result != nil {
		t.Errorf("Expected nil for nil schema, got %v", result)
	}
}
