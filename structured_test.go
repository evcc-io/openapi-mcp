package openapi2mcp

import (
	"encoding/json"
	"testing"
)

func TestStructuredResponse(t *testing.T) {
	op := OpenAPIOperation{OperationID: "getState", Method: "GET", Path: "/state"}

	t.Run("object payload", func(t *testing.T) {
		res := structuredResponse(op, 200, []byte(`{"soc":42}`))

		if res["http_status"] != 200 {
			t.Errorf("http_status = %v", res["http_status"])
		}

		body, ok := res["body"].(map[string]any)
		if !ok {
			t.Fatalf("body is %T, want object", res["body"])
		}
		if body["soc"] != float64(42) {
			t.Errorf("soc = %v", body["soc"])
		}

		operation, ok := res["operation"].(map[string]any)
		if !ok || operation["id"] != "getState" {
			t.Errorf("operation = %v", res["operation"])
		}
	})

	// the protocol requires an object, arrays and scalars ride in body
	t.Run("array payload", func(t *testing.T) {
		res := structuredResponse(op, 200, []byte(`[1,2]`))

		body, ok := res["body"].([]any)
		if !ok || len(body) != 2 {
			t.Fatalf("body is %T, want array of 2", res["body"])
		}
	})

	t.Run("null payload", func(t *testing.T) {
		res := structuredResponse(op, 200, []byte(`null`))

		if body, ok := res["body"]; !ok || body != nil {
			t.Errorf("body = %v, want null", body)
		}
	})

	t.Run("non json payload", func(t *testing.T) {
		res := structuredResponse(op, 200, []byte("plain text"))

		if res["body"] != "plain text" {
			t.Errorf("body = %v", res["body"])
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		res := structuredResponse(op, 204, nil)

		if _, ok := res["body"]; ok {
			t.Errorf("body should be absent, got %v", res["body"])
		}
	})

	// whatever we build has to survive the wire as a JSON object
	t.Run("marshals to an object", func(t *testing.T) {
		b, err := json.Marshal(structuredResponse(op, 200, []byte(`[1,2]`)))
		if err != nil {
			t.Fatal(err)
		}
		if b[0] != '{' {
			t.Errorf("structuredContent must be an object, got %s", b)
		}
	})
}
