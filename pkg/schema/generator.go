// Package schema provides JSON Schema generation for sync configuration.
package schema

import (
	"encoding/json"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/invopop/jsonschema"

	"github.com/smykla-labs/.github/internal/configtypes"
)

// GenerateSchema generates JSON Schema for sync configuration.
// Returns the schema as JSON bytes.
func GenerateSchema(modulePath, configPkgPath string) ([]byte, error) {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties:  false,
		RequiredFromJSONSchemaTags: true,
	}

	// Load Go comments as descriptions
	if err := reflector.AddGoComments(modulePath, configPkgPath); err != nil {
		return nil, errors.Wrap(err, "loading Go comments for schema descriptions")
	}

	schema := reflector.Reflect(&configtypes.SyncConfig{})
	schema.Version = "https://json-schema.org/draft/2020-12/schema"
	schema.ID = "https://raw.githubusercontent.com/smykla-labs/.github/main/schemas/sync-config.schema.json"
	schema.Title = "Sync Configuration"
	schema.Description = "Configuration for organization-wide label, file, and smyklot version synchronization. Place at .github/sync-config.yml in your repository."

	// Convert to JSON and back to map for post-processing
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling schema to bytes")
	}

	var schemaMap map[string]any
	if err = json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, errors.Wrap(err, "unmarshaling schema to map")
	}

	// Add examples to specific fields
	addExamples(schemaMap)

	// Normalize descriptions (replace newlines with spaces)
	normalizeDescriptions(schemaMap)

	output, err := json.MarshalIndent(schemaMap, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "marshaling final schema")
	}

	// Add trailing newline for better git diffs
	output = append(output, '\n')

	return output, nil
}

// addExamples adds examples to specific fields in the schema.
func addExamples(schemaMap map[string]any) {
	defs, ok := schemaMap["$defs"].(map[string]any)
	if !ok {
		return
	}

	// Add examples to LabelsConfig.exclude
	addExcludeExamples(defs, "LabelsConfig", []any{
		[]string{"ci/skip-tests", "ci/force-full"},
		[]string{"release/major", "release/minor", "release/patch"},
	})

	// Add examples to FilesConfig.exclude
	addExcludeExamples(defs, "FilesConfig", []any{
		[]string{"CONTRIBUTING.md", "CODE_OF_CONDUCT.md"},
		[]string{".github/PULL_REQUEST_TEMPLATE.md", "SECURITY.md"},
	})
}

// addExcludeExamples adds examples to the exclude field of a config type.
func addExcludeExamples(defs map[string]any, configName string, examples []any) {
	configDef, ok := defs[configName].(map[string]any)
	if !ok {
		return
	}

	props, ok := configDef["properties"].(map[string]any)
	if !ok {
		return
	}

	exclude, ok := props["exclude"].(map[string]any)
	if !ok {
		return
	}

	exclude["examples"] = examples
}

// normalizeDescriptions recursively replaces newlines in description fields with spaces.
func normalizeDescriptions(v any) {
	switch val := v.(type) {
	case map[string]any:
		for key, value := range val {
			if key == "description" {
				if desc, ok := value.(string); ok {
					val[key] = strings.ReplaceAll(desc, "\n", " ")
				}
			} else {
				normalizeDescriptions(value)
			}
		}
	case []any:
		for _, item := range val {
			normalizeDescriptions(item)
		}
	}
}
