// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

/*
Package modschema reflects Go types from a target module into JSON Schema.

The package creates a temporary helper module, wires module replacements from
the source workspace, executes reflection, and returns generated schema bytes.

Target package must be importable.
Types declared in package main are not supported.

Generate schema for one type:

	schemaBytes, sourcePath, err := modschema.Generate(modschema.Options{
		Module:  ".",
		Package: "github.com/acme/project/internal/config",
		Type:    "Config",
		KeyNamer: "none",
	})
	if err != nil {
		return err
	}

	fmt.Printf("source=%s bytes=%d\n", sourcePath, len(schemaBytes))

Normalize options before custom orchestration:

	options := modschema.NormalizeOptions(modschema.Options{
		Module: "github.com/acme/project@v1.2.3",
		Type:   "Config",
	})

	fmt.Println(options.Module) // defaults to "."
*/
package modschema
