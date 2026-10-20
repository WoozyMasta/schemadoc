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

Reflect only trusted local or remote modules.
Reflection imports and executes the target package and its transitive runtime imports,
including their init functions, with the caller's permissions.
Target code is not sandboxed.
Go module checksums verify integrity, not whether the target code is trustworthy.
Remote @latest references are non-deterministic and are rejected in build check mode.

Use [GenerateContext] or [Generator.GenerateContext] to cancel module setup,
helper execution, or waiting for a busy session.
Errors preserve [context.Canceled] or [context.DeadlineExceeded] for errors.Is checks.
Cancellation removes the affected workspace;
the generator and unrelated prepared workspaces remain usable.
Failed cleanup is reported; cached workspaces stay owned for retries by Close.
Subprocess shutdown and workspace cleanup may delay return after a deadline.
Cancellable commands use Windows job objects or Unix process groups;
deliberately detached Unix processes are outside this lifecycle control.
Other platforms use standard direct-process cancellation.
The existing Generate methods use context.Background and have no mandatory timeout.

Go comments are preserved by default [CommentModeRaw].
Set [Options.Comments] to [CommentModePlain] to normalize extracted comments
into portable plain text before reflection.
Descriptions from struct tags and custom schemas are unchanged.

Definition names default to [TypeNamerModePackage]:
target-package types keep their names and external types use package-base prefixes.
Naming collisions fail generation before any schema output.
Set [Options.TypeNamer] to [TypeNamerModeFull]
for full import-path prefixes with stable SHA-256 hashes.
Target-package names stay unchanged. Both modes share prepared workspaces.

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
