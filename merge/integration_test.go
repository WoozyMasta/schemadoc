// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import (
	"path/filepath"
	"testing"
)

func TestApply_WithGeneratedImportsAndPatch(t *testing.T) {
	t.Parallel()

	targetPath := filepath.Join("..", "testdata", "generated", "app.schema.json")
	sourcePath := filepath.Join("..", "testdata", "generated", "base.schema.json")

	root, err := DecodeFile(targetPath)
	if err != nil {
		t.Fatalf("DecodeFile(target): %v", err)
	}

	// Remove one imported node to ensure merge integration restores it.
	rootObject, ok := root.(map[string]any)
	if !ok {
		t.Fatalf("target root type = %T, want map[string]any", root)
	}

	defs, ok := rootObject["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("target root $defs type = %T, want map[string]any", rootObject["$defs"])
	}

	delete(defs, "model_EndpointMeta")

	plannedImports, err := PlanImports(root, []DefsImport{
		{
			SourcePath: sourcePath,
			Conflict:   ImportConflictKeep,
		},
		{
			SourcePath: sourcePath,
			Rename: DefsImportRename{
				Mode:  "prefix",
				Value: "model_",
			},
			Conflict: ImportConflictReplace,
		},
	}, FileLoader{})
	if err != nil {
		t.Fatalf("PlanImports(): %v", err)
	}

	actions := make([]Action, 0, len(plannedImports)+1)
	actions = append(actions, Action{
		Type:          NodeOpReplace,
		SourcePath:    sourcePath,
		SourcePointer: "/$defs/EndpointMeta",
		TargetPointer: "/$defs/model_EndpointMeta",
	})
	actions = append(actions, plannedImports...)

	merged, err := Apply(root, actions, ApplyOptions{
		PruneUnreachableDefs: false,
	})
	if err != nil {
		t.Fatalf("Apply(): %v", err)
	}

	checks := []string{
		"/$defs/model_SharedOptions",
		"/$defs/model_EndpointMeta/properties/region/type",
	}

	for _, pointer := range checks {
		if _, err := nodeAtPointer(merged, pointer); err != nil {
			t.Fatalf("nodeAtPointer(%q): %v", pointer, err)
		}
	}
}
