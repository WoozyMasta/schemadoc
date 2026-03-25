// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

/*
Package merge provides a reusable JSON Schema merge runtime.

Core library API is file-agnostic and works with in-memory documents.
File-oriented execution is provided as a convenience adapter.

Core entrypoint:

	merge.Apply(root, actions, options)

File helper:

	merge.File(sourcePath, actions, options)

Minimal in-memory example:

	root := map[string]any{
		"$defs": map[string]any{
			"Config": map[string]any{"type": "object"},
		},
	}
	source := map[string]any{
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}
	merged, err := merge.Apply(root, []merge.Action{
		{
			Type:          merge.NodeOpReplace,
			Source:        source,
			SourcePointer: "/$defs/Config",
			TargetPointer: "/$defs/Config",
		},
	}, merge.ApplyOptions{})
	if err != nil {
		log.Fatal(err)
	}
	_ = merged

File-oriented example:

	merged, err := merge.File(
		"warpbox.schema.json",
		[]merge.Action{
			{
				Type:          merge.NodeOpMergeDefs,
				SourcePath:    "linting.schema.json",
				SourcePointer: "/$defs",
				TargetPointer: "/$defs",
			},
		},
		merge.ApplyOptions{PruneUnreachableDefs: true},
	)
	if err != nil {
		log.Fatal(err)
	}

	encoded, err := merge.Encode(merged, merge.FormatJSON)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("warpbox.schema.json", encoded, 0o600); err != nil {
		log.Fatal(err)
	}
*/
package merge
