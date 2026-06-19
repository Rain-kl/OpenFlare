// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		body     map[string]any
		expected string
	}{
		{
			name:     "simple replacement",
			template: "hello {{name}}",
			body:     map[string]any{"name": "world"},
			expected: "hello world",
		},
		{
			name:     "multiple replacements",
			template: "{{greeting}} {{name}}!",
			body:     map[string]any{"greeting": "Hello", "name": "Alice"},
			expected: "Hello Alice!",
		},
		{
			name:     "missing key preserves placeholder",
			template: "hello {{name}} and {{other}}",
			body:     map[string]any{"name": "world"},
			expected: "hello world and {{other}}",
		},
		{
			name:     "unbalanced placeholders",
			template: "hello {{name",
			body:     map[string]any{"name": "world"},
			expected: "hello {{name",
		},
		{
			name:     "nil value",
			template: "val: {{val}}",
			body:     map[string]any{"val": nil},
			expected: "val: ",
		},
		{
			name:     "basic types",
			template: "int: {{i}}, float: {{f}}, bool: {{b}}",
			body:     map[string]any{"i": 123, "f": 45.67, "b": true},
			expected: "int: 123, float: 45.67, bool: true",
		},
		{
			name:     "complex type slice",
			template: "items: {{items}}",
			body:     map[string]any{"items": []string{"a", "b"}},
			expected: `items: ["a","b"]`,
		},
		{
			name:     "complex type map",
			template: "obj: {{obj}}",
			body:     map[string]any{"obj": map[string]any{"key": "value"}},
			expected: `obj: {"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTemplate(tt.template, tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}
