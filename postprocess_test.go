package openapi2mcp

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/jsonschema-go/jsonschema"
)

func TestPostProcessSchema_Integration(t *testing.T) {
	// Create a simple test operation
	params := openapi3.Parameters{
		&openapi3.ParameterRef{Value: &openapi3.Parameter{
			Name:     "testParam",
			In:       "query",
			Required: true,
			Schema:   &openapi3.SchemaRef{Value: &openapi3.Schema{Type: typesPtr("string")}},
		}},
	}

	// Create ToolGenOptions with PostProcessSchema that adds a description
	opts := &ToolGenOptions{
		PostProcessSchema: func(toolName string, schema jsonschema.Schema) jsonschema.Schema {
			// Add a custom description to demonstrate the schema can be modified
			schema.Description = "Modified by PostProcessSchema for tool: " + toolName
			return schema
		},
	}

	// Build the initial schema
	originalSchema := BuildInputSchema(params, nil)

	// Verify original schema doesn't have the custom description
	if originalSchema.Description != "" {
		t.Errorf("Original schema should not have description, got: %s", originalSchema.Description)
	}

	// Apply PostProcessSchema
	processedSchema := opts.PostProcessSchema("testTool", originalSchema)

	// Verify the schema was modified
	expectedDesc := "Modified by PostProcessSchema for tool: testTool"
	if processedSchema.Description != expectedDesc {
		t.Errorf("Expected description %q, got %q", expectedDesc, processedSchema.Description)
	}

	// Verify other properties are preserved
	if processedSchema.Type != "object" {
		t.Errorf("Expected type 'object', got %q", processedSchema.Type)
	}

	if processedSchema.Properties == nil {
		t.Error("Properties should be preserved")
	}

	if _, ok := processedSchema.Properties["testParam"]; !ok {
		t.Error("testParam property should be preserved")
	}

	if len(processedSchema.Required) != 1 || processedSchema.Required[0] != "testParam" {
		t.Errorf("Expected required field 'testParam', got %v", processedSchema.Required)
	}
}

func TestPostProcessSchema_TypesIntegrity(t *testing.T) {
	// Test that the function signature change maintains type safety
	postProcessor := func(toolName string, schema jsonschema.Schema) jsonschema.Schema {
		// This demonstrates the function signature is correct
		return schema
	}

	opts := &ToolGenOptions{
		PostProcessSchema: postProcessor,
	}

	// Verify the assignment works
	if opts.PostProcessSchema == nil {
		t.Error("PostProcessSchema should not be nil")
	}

	// Test that it can be called
	testSchema := jsonschema.Schema{
		Type: "object",
	}

	result := opts.PostProcessSchema("test", testSchema)
	if result.Type != "object" {
		t.Error("Function should return the schema")
	}
}
