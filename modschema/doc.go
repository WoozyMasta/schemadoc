// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

/*
Package modschema reflects Go types from a target module into JSON Schema.

The package creates a temporary helper module, wires module replacements from
the source workspace, executes reflection, and returns generated schema bytes.

Use [NewGenerator] to reuse prepared helper modules for multiple requests.
Call [Generator.Close] when the session ends to remove its temporary directories.
Generator methods serialize requests;
source dependencies and replacement directives must remain stable
for the lifetime of the session.
[Generate] retains the one-request convenience API and performs its own cleanup.

Target package must be importable.
Types declared in package main are not supported.

Go comments are preserved by default [CommentModeRaw].
Set [Options.Comments] to [CommentModePlain] to normalize extracted comments
into portable plain text before reflection.
Descriptions from struct tags and custom schemas are unchanged.

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

Reuse a session for multiple types from the same package:

	generator := modschema.NewGenerator()
	defer generator.Close()

	for _, typeName := range []string{"Config", "Options"} {
		schemaBytes, _, err := generator.Generate(modschema.Options{
			Module: ".",
			Type: typeName,
		})
		if err != nil {
			return err
		}

		fmt.Printf("type=%s bytes=%d\n", typeName, len(schemaBytes))
	}

Normalize options before custom orchestration:

	options := modschema.NormalizeOptions(modschema.Options{
		Module: "github.com/acme/project@v1.2.3",
		Type:   "Config",
	})

	fmt.Println(options.Module) // defaults to "."
*/
package modschema
