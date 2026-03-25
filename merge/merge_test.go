// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import "testing"

func TestSetNodeAtPointer_CreatesNestedObjects(t *testing.T) {
	t.Parallel()

	root := any(map[string]any{})
	if err := setNodeAtPointer(&root, "/$defs/Config", map[string]any{
		"type": "object",
	}); err != nil {
		t.Fatalf("setNodeAtPointer() error = %v", err)
	}

	got, err := nodeAtPointer(root, "/$defs/Config/type")
	if err != nil {
		t.Fatalf("nodeAtPointer() error = %v", err)
	}

	if got != "object" {
		t.Fatalf("nodeAtPointer() = %v, want %v", got, "object")
	}
}

func TestSetNodeAtPointer_RootReplace(t *testing.T) {
	t.Parallel()

	root := any(map[string]any{"old": true})
	if err := setNodeAtPointer(&root, "", map[string]any{"new": true}); err != nil {
		t.Fatalf("setNodeAtPointer() error = %v", err)
	}

	object, ok := root.(map[string]any)
	if !ok {
		t.Fatalf("root type = %T, want map[string]any", root)
	}

	if _, exists := object["old"]; exists {
		t.Fatalf("old key still exists: %v", object)
	}

	if value, exists := object["new"]; !exists || value != true {
		t.Fatalf("new key missing or invalid: %v", object)
	}
}

func TestNodeAtPointer_ArrayIndexOutOfRange(t *testing.T) {
	t.Parallel()

	root := map[string]any{
		"items": []any{"a"},
	}

	if _, err := nodeAtPointer(root, "/items/1"); err == nil {
		t.Fatal("nodeAtPointer() error = nil, want out-of-range error")
	}
}

func TestPruneUnreachableDefs_Transitive(t *testing.T) {
	t.Parallel()

	root := any(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cfg": map[string]any{
				"$ref": "#/$defs/A",
			},
		},
		"$defs": map[string]any{
			"A": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"b": map[string]any{
						"$ref": "#/$defs/B",
					},
				},
			},
			"B": map[string]any{
				"type": "string",
			},
			"C": map[string]any{
				"type": "number",
			},
		},
	})

	if err := pruneUnreachableDefs(&root); err != nil {
		t.Fatalf("pruneUnreachableDefs() error = %v", err)
	}

	rootObject := root.(map[string]any)
	defs := rootObject["$defs"].(map[string]any)

	if _, exists := defs["A"]; !exists {
		t.Fatalf("defs[\"A\"] removed unexpectedly: %v", defs)
	}

	if _, exists := defs["B"]; !exists {
		t.Fatalf("defs[\"B\"] removed unexpectedly: %v", defs)
	}

	if _, exists := defs["C"]; exists {
		t.Fatalf("defs[\"C\"] should be pruned: %v", defs)
	}
}

func TestNormalizeAction_DefaultActionModes(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeAction(0, Action{
		Type:          NodeOpReplace,
		SourcePath:    "overlay.schema.json",
		SourcePointer: "/$defs/Config",
		TargetPointer: "/$defs/Config",
	})
	if err != nil {
		t.Fatalf("normalizeAction() error = %v", err)
	}

	if normalized.ObjectOp != ObjectOpMerge {
		t.Fatalf("object op = %q, want %q", normalized.ObjectOp, ObjectOpMerge)
	}

	if normalized.ArrayOp != ArrayOpReplace {
		t.Fatalf("array op = %q, want %q", normalized.ArrayOp, ArrayOpReplace)
	}
}

func TestApply_InMemorySource(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
			},
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

	merged, err := Apply(base, []Action{
		{
			Type:          NodeOpReplace,
			Source:        source,
			SourcePointer: "/$defs/Config",
			TargetPointer: "/$defs/Config",
		},
	}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	got, err := nodeAtPointer(merged, "/$defs/Config/properties/name/type")
	if err != nil {
		t.Fatalf("nodeAtPointer() error = %v", err)
	}

	if got != "string" {
		t.Fatalf("node value = %v, want %v", got, "string")
	}
}
