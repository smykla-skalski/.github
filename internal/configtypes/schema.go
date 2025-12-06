package configtypes

import "github.com/invopop/jsonschema"

// JSONSchemaExtend adds example values to the LabelsConfig schema.
func (LabelsConfig) JSONSchemaExtend(schema *jsonschema.Schema) {
	if excludeProp, ok := schema.Properties.Get("exclude"); ok {
		excludeProp.Examples = []any{
			[]string{"ci/skip-tests", "ci/force-full"},
			[]string{"release/major", "release/minor", "release/patch"},
		}
	}
}

// JSONSchemaExtend adds example values to the FilesConfig schema.
func (FilesConfig) JSONSchemaExtend(schema *jsonschema.Schema) {
	if excludeProp, ok := schema.Properties.Get("exclude"); ok {
		excludeProp.Examples = []any{
			[]string{"CONTRIBUTING.md", "CODE_OF_CONDUCT.md"},
			[]string{".github/PULL_REQUEST_TEMPLATE.md", "SECURITY.md"},
		}
	}
}

// JSONSchemaExtend adds example values to the FileMergeConfig schema.
func (FileMergeConfig) JSONSchemaExtend(schema *jsonschema.Schema) {
	if pathProp, ok := schema.Properties.Get("path"); ok {
		pathProp.Examples = []any{
			"renovate.json",
			".github/dependabot.yml",
		}
	}
}
