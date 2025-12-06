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
