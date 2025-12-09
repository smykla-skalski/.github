package merge

import (
	"testing"
)

func TestMergeArrays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		base        []any
		override    []any
		strategy    string
		deduplicate bool
		want        []any
	}{
		{
			name:     "append strategy",
			base:     []any{float64(1), float64(2)},
			override: []any{float64(3), float64(4)},
			strategy: "append",
			want:     []any{float64(1), float64(2), float64(3), float64(4)},
		},
		{
			name:     "prepend strategy",
			base:     []any{float64(1), float64(2)},
			override: []any{float64(3), float64(4)},
			strategy: "prepend",
			want:     []any{float64(3), float64(4), float64(1), float64(2)},
		},
		{
			name:     "replace strategy",
			base:     []any{float64(1), float64(2)},
			override: []any{float64(3), float64(4)},
			strategy: "replace",
			want:     []any{float64(3), float64(4)},
		},
		{
			name:     "invalid strategy defaults to replace",
			base:     []any{float64(1), float64(2)},
			override: []any{float64(3), float64(4)},
			strategy: "invalid",
			want:     []any{float64(3), float64(4)},
		},
		{
			name:        "deduplicate primitives with append",
			base:        []any{float64(1), float64(2)},
			override:    []any{float64(2), float64(1), float64(3)},
			strategy:    "append",
			deduplicate: true,
			want:        []any{float64(1), float64(2), float64(3)},
		},
		{
			name: "deduplicate objects with append",
			base: []any{
				map[string]any{"id": float64(1), "name": "alice"},
				map[string]any{"id": float64(2), "name": "bob"},
			},
			override: []any{
				map[string]any{"id": float64(1), "name": "alice"},
				map[string]any{"id": float64(3), "name": "charlie"},
			},
			strategy:    "append",
			deduplicate: true,
			want: []any{
				map[string]any{"id": float64(1), "name": "alice"},
				map[string]any{"id": float64(2), "name": "bob"},
				map[string]any{"id": float64(3), "name": "charlie"},
			},
		},
		{
			name:     "empty base array",
			base:     []any{},
			override: []any{float64(1), float64(2)},
			strategy: "append",
			want:     []any{float64(1), float64(2)},
		},
		{
			name:     "empty override array",
			base:     []any{float64(1), float64(2)},
			override: []any{},
			strategy: "append",
			want:     []any{float64(1), float64(2)},
		},
		{
			name:     "nil base array",
			base:     nil,
			override: []any{float64(1), float64(2)},
			strategy: "append",
			want:     []any{float64(1), float64(2)},
		},
		{
			name:     "nil override array",
			base:     []any{float64(1), float64(2)},
			override: nil,
			strategy: "append",
			want:     []any{float64(1), float64(2)},
		},
		{
			name:     "both nil arrays",
			base:     nil,
			override: nil,
			strategy: "append",
			want:     []any{},
		},
		{
			name:     "strings append",
			base:     []any{"a", "b"},
			override: []any{"c", "d"},
			strategy: "append",
			want:     []any{"a", "b", "c", "d"},
		},
		{
			name: "mixed types append",
			base: []any{
				float64(1),
				"string",
				map[string]any{"key": "value"},
			},
			override: []any{
				true,
				[]any{"nested", "array"},
			},
			strategy: "append",
			want: []any{
				float64(1),
				"string",
				map[string]any{"key": "value"},
				true,
				[]any{"nested", "array"},
			},
		},
		{
			name:        "deduplicate with prepend",
			base:        []any{float64(1), float64(2), float64(3)},
			override:    []any{float64(3), float64(4), float64(1)},
			strategy:    "prepend",
			deduplicate: true,
			want:        []any{float64(3), float64(4), float64(1), float64(2)},
		},
		{
			name:        "deduplicate with replace",
			base:        []any{float64(1), float64(2)},
			override:    []any{float64(3), float64(3), float64(4)},
			strategy:    "replace",
			deduplicate: true,
			want:        []any{float64(3), float64(4)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mergeArrays(tt.base, tt.override, tt.strategy, tt.deduplicate)

			if !arraysEqual(got, tt.want) {
				t.Errorf("mergeArrays() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeduplicateArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		arr  []any
		want []any
	}{
		{
			name: "primitives with duplicates",
			arr:  []any{float64(1), float64(2), float64(1), float64(3), float64(2)},
			want: []any{float64(1), float64(2), float64(3)},
		},
		{
			name: "strings with duplicates",
			arr:  []any{"a", "b", "a", "c"},
			want: []any{"a", "b", "c"},
		},
		{
			name: "objects with duplicates",
			arr: []any{
				map[string]any{"id": float64(1)},
				map[string]any{"id": float64(2)},
				map[string]any{"id": float64(1)},
			},
			want: []any{
				map[string]any{"id": float64(1)},
				map[string]any{"id": float64(2)},
			},
		},
		{
			name: "no duplicates",
			arr:  []any{float64(1), float64(2), float64(3)},
			want: []any{float64(1), float64(2), float64(3)},
		},
		{
			name: "empty array",
			arr:  []any{},
			want: []any{},
		},
		{
			name: "single element",
			arr:  []any{float64(1)},
			want: []any{float64(1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := deduplicateArray(tt.arr)

			if !arraysEqual(got, tt.want) {
				t.Errorf("deduplicateArray() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectArrays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		obj    map[string]any
		prefix string
		want   map[string][]any
	}{
		{
			name: "top level array",
			obj: map[string]any{
				"items": []any{float64(1), float64(2)},
			},
			prefix: "$",
			want: map[string][]any{
				"$.items": {float64(1), float64(2)},
			},
		},
		{
			name: "nested array",
			obj: map[string]any{
				"config": map[string]any{
					"rules": []any{"a", "b"},
				},
			},
			prefix: "$",
			want: map[string][]any{
				"$.config.rules": {"a", "b"},
			},
		},
		{
			name: "multiple arrays",
			obj: map[string]any{
				"extends":      []any{"base"},
				"packageRules": []any{"rule1"},
				"config": map[string]any{
					"ignorePaths": []any{"/node_modules/"},
				},
			},
			prefix: "$",
			want: map[string][]any{
				"$.extends":            {"base"},
				"$.packageRules":       {"rule1"},
				"$.config.ignorePaths": {"/node_modules/"},
			},
		},
		{
			name: "no arrays",
			obj: map[string]any{
				"name": "test",
				"config": map[string]any{
					"timeout": float64(30),
				},
			},
			prefix: "$",
			want:   map[string][]any{},
		},
		{
			name:   "empty object",
			obj:    map[string]any{},
			prefix: "$",
			want:   map[string][]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := collectArrays(tt.obj, tt.prefix)

			if len(got) != len(tt.want) {
				t.Errorf("collectArrays() len = %d, want %d", len(got), len(tt.want))

				return
			}

			for path, wantArr := range tt.want {
				gotArr, ok := got[path]
				if !ok {
					t.Errorf("collectArrays() missing path %s", path)

					continue
				}

				if !arraysEqual(gotArr, wantArr) {
					t.Errorf("collectArrays() at %s = %v, want %v", path, gotArr, wantArr)
				}
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "single field",
			path: "field",
			want: []string{"field"},
		},
		{
			name: "nested path",
			path: "field.nested.deep",
			want: []string{"field", "nested", "deep"},
		},
		{
			name: "empty path",
			path: "",
			want: []string{},
		},
		{
			name: "path with trailing dot",
			path: "field.",
			want: []string{"field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := splitPath(tt.path)

			if len(got) != len(tt.want) {
				t.Errorf("splitPath() len = %d, want %d", len(got), len(tt.want))

				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitPath()[%d] = %s, want %s", i, got[i], tt.want[i])
				}
			}
		})
	}
}
