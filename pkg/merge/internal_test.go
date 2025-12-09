package merge

import "testing"

func TestIsNestedPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "top level",
			path: "$.field",
			want: false,
		},
		{
			name: "nested",
			path: "$.field.nested",
			want: true,
		},
		{
			name: "deeply nested",
			path: "$.a.b.c",
			want: true,
		},
		{
			name: "invalid - no dollar",
			path: "field",
			want: false,
		},
		{
			name: "invalid - no dot after dollar",
			path: "$field",
			want: false,
		},
		{
			name: "empty",
			path: "",
			want: false,
		},
		{
			name: "just dollar",
			path: "$",
			want: false,
		},
		{
			name: "just dollar dot",
			path: "$.",
			want: false,
		},
		{
			name: "single character field",
			path: "$.a",
			want: false,
		},
		{
			name: "multiple dots",
			path: "$.a.b.c.d.e",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isNestedPath(tt.path)
			if got != tt.want {
				t.Errorf("isNestedPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSetValueAtPath(t *testing.T) {
	t.Parallel()

	t.Run("success cases", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			obj   map[string]any
			path  string
			value any
			want  map[string]any
		}{
			{
				name:  "top level field",
				obj:   map[string]any{"existing": "value"},
				path:  "$.field",
				value: "new",
				want:  map[string]any{"existing": "value", "field": "new"},
			},
			{
				name:  "nested field",
				obj:   map[string]any{"parent": map[string]any{"existing": "value"}},
				path:  "$.parent.child",
				value: "new",
				want:  map[string]any{"parent": map[string]any{"existing": "value", "child": "new"}},
			},
			{
				name:  "deeply nested",
				obj:   map[string]any{"a": map[string]any{"b": map[string]any{"c": "old"}}},
				path:  "$.a.b.d",
				value: "new",
				want:  map[string]any{"a": map[string]any{"b": map[string]any{"c": "old", "d": "new"}}},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := setValueAtPath(tt.obj, tt.path, tt.value)
				if err != nil {
					t.Fatalf("setValueAtPath() error = %v", err)
				}

				if !mapsEqual(tt.obj, tt.want) {
					t.Errorf("setValueAtPath() = %v, want %v", tt.obj, tt.want)
				}
			})
		}
	})

	t.Run("error cases", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			obj     map[string]any
			path    string
			value   any
			wantErr string
		}{
			{
				name:    "invalid path - no dollar",
				obj:     map[string]any{},
				path:    "field",
				value:   "value",
				wantErr: "invalid path format",
			},
			{
				name:    "invalid path - no dot after dollar",
				obj:     map[string]any{},
				path:    "$field",
				value:   "value",
				wantErr: "invalid path format",
			},
			{
				name:    "invalid path - too short",
				obj:     map[string]any{},
				path:    "$",
				value:   "value",
				wantErr: "invalid path format",
			},
			{
				name:    "empty path after dollar dot",
				obj:     map[string]any{},
				path:    "$.",
				value:   "value",
				wantErr: "empty path",
			},
			{
				name:    "path segment is not a map",
				obj:     map[string]any{"parent": "not-a-map"},
				path:    "$.parent.child",
				value:   "value",
				wantErr: "path segment parent is not a map",
			},
			{
				name:    "deeply nested non-map",
				obj:     map[string]any{"a": map[string]any{"b": []any{1, 2, 3}}},
				path:    "$.a.b.c",
				value:   "value",
				wantErr: "path segment b is not a map",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := setValueAtPath(tt.obj, tt.path, tt.value)
				if err == nil {
					t.Fatal("setValueAtPath() expected error, got nil")
				}

				if !contains(err.Error(), tt.wantErr) {
					t.Errorf("setValueAtPath() error = %q, want error containing %q", err.Error(), tt.wantErr)
				}
			})
		}
	})
}

// contains checks if the string s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
