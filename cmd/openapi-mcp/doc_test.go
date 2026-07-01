package main

import "testing"

func TestExampleValue(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want any
	}{
		{"example wins", map[string]any{"type": "string", "examples": []any{"15m"}, "default": "1h", "enum": []any{"json"}}, "15m"},
		{"default over enum", map[string]any{"type": "string", "default": "json", "enum": []any{"csv"}}, "json"},
		{"enum over placeholder", map[string]any{"type": "string", "enum": []any{"csv", "json"}}, "csv"},
		{"boolean default false", map[string]any{"type": "boolean", "default": false}, false},
		{"type placeholder fallback", map[string]any{"type": "string"}, "example"},
		{"integer-in-string desc", map[string]any{"type": "string", "description": "an integer id"}, "123"},
	}
	for _, c := range cases {
		if got := exampleValue(c.in); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
