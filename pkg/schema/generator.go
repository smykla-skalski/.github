// Package schema provides JSON Schema generation for sync configuration.
package schema

import (
	"encoding/json"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/invopop/jsonschema"

	"github.com/smykla-skalski/.github/internal/configtypes"
	"github.com/smykla-skalski/.github/pkg/github"
)

// SchemaOutput represents a generated schema with its metadata.
type SchemaOutput struct {
	// Name is the short identifier for this schema (e.g., "sync-config", "settings", "smyklot")
	Name string
	// Filename is the output filename (e.g., "sync-config.schema.json")
	Filename string
	// Content is the generated JSON schema bytes
	Content []byte
}

// SchemaType identifies the type of schema to generate.
type SchemaType string

const (
	// SchemaSyncConfig generates schema for .github/sync-config.yml
	SchemaSyncConfig SchemaType = "sync-config"
	// SchemaSettings generates schema for .github/settings.yml
	SchemaSettings SchemaType = "settings"
	// SchemaSmyklot generates schema for .github/smyklot.yml
	SchemaSmyklot SchemaType = "smyklot"
)

// commentPaths lists all source directories containing types used in schemas.
// These paths are loaded to extract Go doc comments as JSON Schema descriptions.
var commentPaths = []string{
	"./internal/configtypes",
	"./pkg/github",
}

// GenerateSchema generates JSON Schema for sync configuration.
// Returns the schema as JSON bytes.
//
// Deprecated: Use GenerateSchemaForType instead.
func GenerateSchema(modulePath, configPkgPath string) ([]byte, error) {
	output, err := GenerateSchemaForType(modulePath, configPkgPath, SchemaSyncConfig)
	if err != nil {
		return nil, err
	}

	return output.Content, nil
}

// GenerateSchemaForType generates JSON Schema for the specified schema type.
func GenerateSchemaForType(
	modulePath, _ string, // configPkgPath deprecated, using commentPaths instead
	schemaType SchemaType,
) (*SchemaOutput, error) {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties:  false,
		RequiredFromJSONSchemaTags: true,
	}

	// Load Go comments as descriptions from all type source directories
	for _, path := range commentPaths {
		if err := reflector.AddGoComments(modulePath, path); err != nil {
			return nil, errors.Wrapf(err, "loading Go comments from %s", path)
		}
	}

	var schema *jsonschema.Schema

	var output SchemaOutput

	switch schemaType {
	case SchemaSyncConfig:
		schema = reflector.Reflect(&configtypes.SyncConfig{})
		schema.ID = "https://raw.githubusercontent.com/smykla-skalski/.github/main/schemas/sync-config.schema.json"
		schema.Title = "Sync Configuration"
		schema.Description = "Configuration for organization-wide label, file, and smyklot version synchronization. Place at .github/sync-config.yml in your repository."

		// Inject settings type definitions for SettingsMergeConfig.overrides
		injectSettingsDefinitions(schema, &reflector)

		output.Name = "sync-config"
		output.Filename = "sync-config.schema.json"

	case SchemaSettings:
		schema = reflector.Reflect(&github.SettingsFile{})
		schema.ID = "https://raw.githubusercontent.com/smykla-skalski/.github/main/schemas/settings.schema.json"
		schema.Title = "Repository Settings"
		schema.Description = "Repository settings definition for organization-wide synchronization. Place at .github/settings.yml in your repository."

		output.Name = "settings"
		output.Filename = "settings.schema.json"

	case SchemaSmyklot:
		schema = reflector.Reflect(&configtypes.SmyklotFile{})
		schema.ID = "https://raw.githubusercontent.com/smykla-skalski/.github/main/schemas/smyklot.schema.json"
		schema.Title = "Smyklot Configuration"
		schema.Description = "Organization-wide smyklot configuration controlling version sync and workflow installation. Place at .github/smyklot.yml in the .github repository."

		output.Name = "smyklot"
		output.Filename = "smyklot.schema.json"

	default:
		return nil, errors.Newf("unknown schema type: %s", schemaType)
	}

	schema.Version = "https://json-schema.org/draft/2020-12/schema"

	content, err := finalizeSchema(schema)
	if err != nil {
		return nil, err
	}

	output.Content = content

	return &output, nil
}

// GenerateAllSchemas generates all available schemas.
func GenerateAllSchemas(modulePath, configPkgPath string) ([]*SchemaOutput, error) {
	schemaTypes := []SchemaType{SchemaSyncConfig, SchemaSettings, SchemaSmyklot}
	outputs := make([]*SchemaOutput, 0, len(schemaTypes))

	for _, schemaType := range schemaTypes {
		output, err := GenerateSchemaForType(modulePath, configPkgPath, schemaType)
		if err != nil {
			return nil, errors.Wrapf(err, "generating %s schema", schemaType)
		}

		outputs = append(outputs, output)
	}

	return outputs, nil
}

// finalizeSchema converts a schema to JSON and applies post-processing.
// Note: Run `jsonschema fmt` on output for canonical key ordering.
func finalizeSchema(schema *jsonschema.Schema) ([]byte, error) {
	// Convert to JSON and back to map for post-processing
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling schema to bytes")
	}

	var schemaMap map[string]any
	if err = json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, errors.Wrap(err, "unmarshaling schema to map")
	}

	// Apply lint-fixing transformations
	applyLintFixes(schemaMap)

	output, err := json.MarshalIndent(schemaMap, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "marshaling final schema")
	}

	// Add trailing newline for better git diffs
	output = append(output, '\n')

	return output, nil
}

// applyLintFixes applies all lint-fixing transformations to the schema.
func applyLintFixes(schemaMap map[string]any) {
	// Normalize descriptions (replace newlines with spaces)
	normalizeDescriptions(schemaMap)

	// Remove type when enum is present (enum_with_type lint rule)
	removeTypeWithEnum(schemaMap)
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

// removeTypeWithEnum recursively removes "type" when "enum" is present.
// This fixes the enum_with_type lint rule: enum values already imply their type.
func removeTypeWithEnum(v any) {
	switch val := v.(type) {
	case map[string]any:
		// If this object has both "enum" and "type", remove "type"
		if _, hasEnum := val["enum"]; hasEnum {
			delete(val, "type")
		}

		// Recurse into all values
		for _, value := range val {
			removeTypeWithEnum(value)
		}

	case []any:
		for _, item := range val {
			removeTypeWithEnum(item)
		}
	}
}

// settingsOverrideTypes maps section names to their corresponding config types.
// These are used to provide proper type definitions for SettingsMergeConfig.overrides.
var settingsOverrideTypes = []struct {
	name    string
	typeRef any
}{
	{"RepositorySettingsConfig", &configtypes.RepositorySettingsConfig{}},
	{"FeaturesConfig", &configtypes.FeaturesConfig{}},
	{"SecurityConfig", &configtypes.SecurityConfig{}},
	{"BranchProtectionRuleConfig", &configtypes.BranchProtectionRuleConfig{}},
	{"RulesetConfig", &configtypes.RulesetConfig{}},
}

// injectSettingsDefinitions adds settings type definitions to the sync-config schema
// and updates SettingsMergeConfig.overrides to reference them with anyOf.
func injectSettingsDefinitions(schema *jsonschema.Schema, reflector *jsonschema.Reflector) {
	if schema.Definitions == nil {
		schema.Definitions = make(jsonschema.Definitions)
	}

	anyOfRefs := buildSettingsOverrideDefinitions(schema, reflector)
	updateOverridesSchema(schema, anyOfRefs)
}

// buildSettingsOverrideDefinitions generates override type definitions and returns anyOf refs.
func buildSettingsOverrideDefinitions(
	schema *jsonschema.Schema,
	reflector *jsonschema.Reflector,
) []*jsonschema.Schema {
	anyOfRefs := make([]*jsonschema.Schema, 0, len(settingsOverrideTypes)+1)

	for _, st := range settingsOverrideTypes {
		typeSchema := reflector.Reflect(st.typeRef)
		copyDefinitions(schema, typeSchema)

		defName := "SettingsOverride_" + st.name
		actualSchema := resolveTypeSchema(typeSchema)

		// Only add the reference when actualSchema is successfully resolved
		if actualSchema != nil {
			schema.Definitions[defName] = createOverrideSchema(actualSchema, st.name)
			anyOfRefs = append(anyOfRefs, &jsonschema.Schema{
				Ref: "#/$defs/" + defName,
			})
		}
	}

	// Also allow plain object for flexibility (advanced use cases)
	anyOfRefs = append(anyOfRefs, &jsonschema.Schema{
		Type:        "object",
		Description: "Custom override object for advanced use cases",
	})

	return anyOfRefs
}

// copyDefinitions copies type definitions from source schema to target.
func copyDefinitions(target *jsonschema.Schema, source *jsonschema.Schema) {
	if source.Definitions == nil {
		return
	}

	for key, value := range source.Definitions {
		if _, exists := target.Definitions[key]; !exists {
			target.Definitions[key] = value
		}
	}
}

// resolveTypeSchema extracts the actual schema, dereferencing if needed.
func resolveTypeSchema(typeSchema *jsonschema.Schema) *jsonschema.Schema {
	if typeSchema.Ref == "" {
		return typeSchema
	}

	refName := strings.TrimPrefix(typeSchema.Ref, "#/$defs/")
	if refSchema, ok := typeSchema.Definitions[refName]; ok {
		return refSchema
	}

	return nil
}

// createOverrideSchema creates a permissive schema for merge overrides.
func createOverrideSchema(actualSchema *jsonschema.Schema, typeName string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:                 actualSchema.Type,
		Properties:           actualSchema.Properties,
		AdditionalProperties: actualSchema.AdditionalProperties,
		Description:          "Partial " + typeName + " for merge overrides. Only specified fields will override org defaults.",
		// Don't require any fields since this is for partial overrides
	}
}

// updateOverridesSchema updates SettingsMergeConfig.overrides to use anyOf.
func updateOverridesSchema(schema *jsonschema.Schema, anyOfRefs []*jsonschema.Schema) {
	settingsMergeConfig, ok := schema.Definitions["SettingsMergeConfig"]
	if !ok || settingsMergeConfig.Properties == nil {
		return
	}

	overridesProp, ok := settingsMergeConfig.Properties.Get("overrides")
	if !ok {
		return
	}

	overridesProp.AnyOf = anyOfRefs
	overridesProp.Type = "" // Clear type when using anyOf
	overridesProp.Description = "Override values to merge with org settings. " +
		"Structure should match the section type (repository, features, security, " +
		"branch protection rule, or ruleset). Only specified fields will override org defaults."
}
