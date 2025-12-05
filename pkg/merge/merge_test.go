package merge_test

import (
	"testing"

	"github.com/smykla-labs/.github/pkg/config"
	"github.com/smykla-labs/.github/pkg/merge"
)

func TestDeepMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     map[string]any
		override map[string]any
		want     map[string]any
		wantErr  bool
	}{
		{
			name: "basic object merge",
			base: map[string]any{
				"name": "org",
				"age":  30,
			},
			override: map[string]any{
				"age":  35,
				"city": "NYC",
			},
			want: map[string]any{
				"name": "org",
				"age":  float64(35),
				"city": "NYC",
			},
		},
		{
			name: "nested object merge",
			base: map[string]any{
				"config": map[string]any{
					"timeout": 30,
					"retries": 3,
				},
			},
			override: map[string]any{
				"config": map[string]any{
					"timeout": 60,
					"verbose": true,
				},
			},
			want: map[string]any{
				"config": map[string]any{
					"timeout": float64(60),
					"retries": float64(3),
					"verbose": true,
				},
			},
		},
		{
			name: "array replacement not merge",
			base: map[string]any{
				"items": []any{"a", "b", "c"},
			},
			override: map[string]any{
				"items": []any{"x", "y"},
			},
			want: map[string]any{
				"items": []any{"x", "y"},
			},
		},
		{
			name: "null value deletes key",
			base: map[string]any{
				"keep":   "value",
				"remove": "value",
			},
			override: map[string]any{
				"remove": nil,
			},
			want: map[string]any{
				"keep": "value",
			},
		},
		{
			name: "nested null deletion",
			base: map[string]any{
				"config": map[string]any{
					"keep":   "value",
					"remove": "value",
				},
			},
			override: map[string]any{
				"config": map[string]any{
					"remove": nil,
				},
			},
			want: map[string]any{
				"config": map[string]any{
					"keep": "value",
				},
			},
		},
		{
			name:     "nil base map",
			base:     nil,
			override: map[string]any{"key": "value"},
			want:     map[string]any{"key": "value"},
		},
		{
			name:     "nil override map",
			base:     map[string]any{"key": "value"},
			override: nil,
			want:     map[string]any{"key": "value"},
		},
		{
			name:     "both nil maps",
			base:     nil,
			override: nil,
			want:     map[string]any{},
		},
		{
			name: "type override",
			base: map[string]any{
				"value": "string",
			},
			override: map[string]any{
				"value": float64(123),
			},
			want: map[string]any{
				"value": float64(123),
			},
		},
		{
			name: "deep nested merge",
			base: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"keep": "base",
						},
					},
				},
			},
			override: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"add": "override",
						},
					},
				},
			},
			want: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"keep": "base",
							"add":  "override",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge.DeepMerge(tt.base, tt.override)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeepMerge() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !mapsEqual(got, tt.want) {
				t.Errorf("DeepMerge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShallowMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     map[string]any
		override map[string]any
		want     map[string]any
		wantErr  bool
	}{
		{
			name: "top level merge only",
			base: map[string]any{
				"name": "org",
				"age":  30,
			},
			override: map[string]any{
				"age":  35,
				"city": "NYC",
			},
			want: map[string]any{
				"name": "org",
				"age":  35,
				"city": "NYC",
			},
		},
		{
			name: "nested object replaced not merged",
			base: map[string]any{
				"config": map[string]any{
					"timeout": 30,
					"retries": 3,
				},
			},
			override: map[string]any{
				"config": map[string]any{
					"timeout": 60,
				},
			},
			want: map[string]any{
				"config": map[string]any{
					"timeout": 60,
				},
			},
		},
		{
			name: "null value deletes top level key",
			base: map[string]any{
				"keep":   "value",
				"remove": "value",
			},
			override: map[string]any{
				"remove": nil,
			},
			want: map[string]any{
				"keep": "value",
			},
		},
		{
			name:     "nil base map",
			base:     nil,
			override: map[string]any{"key": "value"},
			want:     map[string]any{"key": "value"},
		},
		{
			name:     "nil override map",
			base:     map[string]any{"key": "value"},
			override: nil,
			want:     map[string]any{"key": "value"},
		},
		{
			name: "array replacement",
			base: map[string]any{
				"items": []any{"a", "b"},
			},
			override: map[string]any{
				"items": []any{"x"},
			},
			want: map[string]any{
				"items": []any{"x"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge.ShallowMerge(tt.base, tt.override)
			if (err != nil) != tt.wantErr {
				t.Errorf("ShallowMerge() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !mapsEqual(got, tt.want) {
				t.Errorf("ShallowMerge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeJSON(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"name": "org",
		"config": map[string]any{
			"timeout": 30,
		},
	}

	override := map[string]any{
		"config": map[string]any{
			"retries": 3,
		},
	}

	tests := []struct {
		name     string
		strategy config.MergeStrategy
		want     map[string]any
		wantErr  bool
	}{
		{
			name:     "deep merge strategy",
			strategy: config.MergeStrategyDeep,
			want: map[string]any{
				"name": "org",
				"config": map[string]any{
					"timeout": float64(30),
					"retries": float64(3),
				},
			},
		},
		{
			name:     "overlay strategy alias",
			strategy: config.MergeStrategyOverlay,
			want: map[string]any{
				"name": "org",
				"config": map[string]any{
					"timeout": float64(30),
					"retries": float64(3),
				},
			},
		},
		{
			name:     "shallow merge strategy",
			strategy: config.MergeStrategyShallow,
			want: map[string]any{
				"name": "org",
				"config": map[string]any{
					"retries": 3,
				},
			},
		},
		{
			name:     "unknown strategy",
			strategy: config.MergeStrategy("invalid"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge.MergeJSON(base, override, tt.strategy)
			if (err != nil) != tt.wantErr {
				t.Errorf("MergeJSON() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && !mapsEqual(got, tt.want) {
				t.Errorf("MergeJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeYAML(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"name": "org",
	}

	override := map[string]any{
		"age": 30,
	}

	got, err := merge.MergeYAML(base, override, config.MergeStrategyDeep)
	if err != nil {
		t.Fatalf("MergeYAML() error = %v", err)
	}

	want := map[string]any{
		"name": "org",
		"age":  float64(30),
	}

	if !mapsEqual(got, want) {
		t.Errorf("MergeYAML() = %v, want %v", got, want)
	}
}

func TestParseJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		want    map[string]any
		wantErr bool
	}{
		{
			name: "valid json",
			data: `{"name":"test","age":30}`,
			want: map[string]any{
				"name": "test",
				"age":  float64(30),
			},
		},
		{
			name:    "invalid json",
			data:    `{invalid}`,
			wantErr: true,
		},
		{
			name: "empty json object",
			data: `{}`,
			want: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge.ParseJSON([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSON() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && !mapsEqual(got, tt.want) {
				t.Errorf("ParseJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		want    map[string]any
		wantErr bool
	}{
		{
			name: "valid yaml",
			data: "name: test\nage: 30\n",
			want: map[string]any{
				"name": "test",
				"age":  30,
			},
		},
		{
			name:    "invalid yaml",
			data:    "name: test\n  invalid: indent\n",
			wantErr: true,
		},
		{
			name: "empty yaml",
			data: "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge.ParseYAML([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseYAML() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && !mapsEqual(got, tt.want) {
				t.Errorf("ParseYAML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"name": "test",
		"age":  float64(30),
	}

	result, err := merge.MarshalJSON(data)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	// Parse back to verify
	parsed, err := merge.ParseJSON(result)
	if err != nil {
		t.Fatalf("ParseJSON() error = %v", err)
	}

	if !mapsEqual(parsed, data) {
		t.Errorf("Round trip failed: got %v, want %v", parsed, data)
	}
}

func TestMarshalYAML(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"name": "test",
		"age":  30,
	}

	result, err := merge.MarshalYAML(data)
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}

	// Parse back to verify
	parsed, err := merge.ParseYAML(result)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	if !mapsEqual(parsed, data) {
		t.Errorf("Round trip failed: got %v, want %v", parsed, data)
	}
}

// mapsEqual compares two maps deeply.
func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}

	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}

		if !valuesEqual(va, vb) {
			return false
		}
	}

	return true
}

// valuesEqual compares two values deeply.
func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	switch va := a.(type) {
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok {
			return false
		}

		return mapsEqual(va, vb)
	case []any:
		vb, ok := b.([]any)
		if !ok {
			return false
		}

		if len(va) != len(vb) {
			return false
		}

		for i := range va {
			if !valuesEqual(va[i], vb[i]) {
				return false
			}
		}

		return true
	default:
		return a == b
	}
}
