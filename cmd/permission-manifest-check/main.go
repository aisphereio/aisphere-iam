package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/aisphereio/aisphere-iam/internal/permissionmanifest"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("permission-manifest-check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String("manifest", "configs/resource/defaults.yaml", "permission manifest path")
	schemaPath := flags.String("schema", "configs/spicedb/aisphere.schema.zed", "SpiceDB schema path")
	if err := flags.Parse(args); err != nil {
		return err
	}

	manifest, err := permissionmanifest.Load(*manifestPath)
	if err != nil {
		return fmt.Errorf("load permission manifest: %w", err)
	}
	body, err := os.ReadFile(*schemaPath)
	if err != nil {
		return fmt.Errorf("read SpiceDB schema: %w", err)
	}
	schema, err := permissionmanifest.ParseSchema(string(body))
	if err != nil {
		return fmt.Errorf("parse SpiceDB schema: %w", err)
	}
	if err := permissionmanifest.Validate(manifest, schema); err != nil {
		return err
	}
	fmt.Fprintf(output, "permission manifest valid: %d resource types, %d role templates, %d schema definitions\n", len(manifest.ResourceTypes), len(manifest.RoleTemplates), len(schema.Definitions))
	return nil
}
