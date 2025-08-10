// summary.go
package openapi2mcp

import "fmt"

// PrintToolSummary prints a summary of the generated tools (count, tags, etc).
func PrintToolSummary(ops []OpenAPIOperation) {
	tagCount := map[string]int{}
	for _, op := range ops {
		for _, tag := range op.Tags {
			tagCount[tag]++
		}
	}
	fmt.Printf("Total tools: %d\n", len(ops))
	if len(tagCount) > 0 {
		fmt.Println("Tags:")
		for tag, count := range tagCount {
			fmt.Printf("  %s: %d\n", tag, count)
		}
	}
}

// Example usage for PrintToolSummary:
//
//   doc, _ := openapi2mcp.LoadOpenAPISpec("petstore.yaml")
//   ops := openapi2mcp.ExtractOpenAPIOperations(doc)
//   openapi2mcp.PrintToolSummary(ops)
