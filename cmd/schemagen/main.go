// Package main provides a simple CLI to generate JSON Schema for sync configuration.
// This binary is not released; it's used via `go run` in workflows.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"

	"github.com/smykla-labs/.github/pkg/schema"
)

const (
	modulePath    = "github.com/smykla-labs/.github"
	configPkgPath = "./internal/configtypes"
	// schemaFileMode is the permission mode for generated schema files.
	// Schemas need to be world-readable for CI verification and external tooling.
	schemaFileMode = 0o644
)

func main() {
	var (
		generateAll bool
		outputDir   string
		schemaType  string
	)

	flag.BoolVar(&generateAll, "all", false, "Generate all schemas (sync-config and settings)")
	flag.StringVar(&outputDir, "output-dir", "", "Output directory for generated schemas (required with --all)")
	flag.StringVar(&schemaType, "type", "sync-config", "Schema type to generate: sync-config or settings")
	flag.Parse()

	if generateAll {
		if outputDir == "" {
			fatalf("--output-dir is required when using --all")
		}

		if err := generateAllSchemas(outputDir); err != nil {
			fatalf("generating schemas: %v", err)
		}

		return
	}

	// Single schema generation (original behavior, outputs to stdout)
	if err := generateSingleSchema(schemaType); err != nil {
		fatalf("%v", err)
	}
}

func generateAllSchemas(outputDir string) error {
	outputs, err := schema.GenerateAllSchemas(modulePath, configPkgPath)
	if err != nil {
		return err
	}

	for _, output := range outputs {
		outputPath := filepath.Join(outputDir, output.Filename)

		if err := os.WriteFile(outputPath, output.Content, schemaFileMode); err != nil {
			return errors.Wrapf(err, "writing %s", output.Filename)
		}

		fmt.Printf("Generated %s\n", outputPath)
	}

	return nil
}

func generateSingleSchema(schemaType string) error {
	st := schema.SchemaType(schemaType)

	output, err := schema.GenerateSchemaForType(modulePath, configPkgPath, st)
	if err != nil {
		return err
	}

	fmt.Print(string(output.Content))

	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
