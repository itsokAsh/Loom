package nodes

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTransformExecutor_Execute(t *testing.T) {
	executor, err := Get("TRANSFORM")
	if err != nil {
		t.Fatalf("expected to get TRANSFORM executor, got error: %v", err)
	}

	tests := []struct {
		name        string
		config      string
		expectError bool
		expected    string
	}{
		{
			name: "Map mapping",
			config: `{
				"mapping": {
					"field1": "value1",
					"field2": 123,
					"nested": {
						"inner": true
					}
				}
			}`,
			expectError: false,
			expected:    `{"field1":"value1","field2":123,"nested":{"inner":true}}`,
		},
		{
			name: "Array mapping",
			config: `{
				"mapping": ["item1", "item2", 3]
			}`,
			expectError: false,
			expected:    `["item1","item2",3]`,
		},
		{
			name: "String mapping",
			config: `{
				"mapping": "just a string"
			}`,
			expectError: false,
			expected:    `"just a string"`,
		},
		{
			name: "Null mapping",
			config: `{
				"mapping": null
			}`,
			expectError: false,
			expected:    `{}`,
		},
		{
			name: "Missing mapping",
			config: `{}`,
			expectError: false,
			expected:    `{}`,
		},
		{
			name: "Invalid JSON config",
			config: `{ "mapping": `,
			expectError: true,
			expected:    ``,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executor.Execute(context.Background(), json.RawMessage(tc.config))
			
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if string(result) != tc.expected {
				t.Errorf("expected result %s, got %s", tc.expected, string(result))
			}
		})
	}
}
