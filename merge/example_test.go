// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/woozymasta/schemadoc/merge"
)

func ExampleApply() {
	root := map[string]any{"$defs": map[string]any{"Config": map[string]any{"type": "object"}}}
	merged, err := merge.Apply(root, []merge.Action{{
		Type:   merge.NodeOpReplace,
		Source: map[string]any{"type": "string"}, TargetPointer: "/$defs/Config",
	}}, merge.ApplyOptions{})
	if err != nil {
		panic(err)
	}

	encoded, err := json.Marshal(merged)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
	// Output: {"$defs":{"Config":{"type":"string"}}}
}

func ExampleFile() {
	dir, err := os.MkdirTemp("", "schemadoc-merge-example-*")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			panic(err)
		}
	}()
	basePath, sourcePath := filepath.Join(dir, "base.json"), filepath.Join(dir, "shared.json")
	if err := os.WriteFile(basePath, []byte(`{"$ref":"#/$defs/Config","$defs":{}}`), 0o600); err != nil {
		panic(err)
	}
	if err := os.WriteFile(sourcePath, []byte(`{"$defs":{"Config":{"type":"object"}}}`), 0o600); err != nil {
		panic(err)
	}

	merged, err := merge.File(basePath, []merge.Action{{
		Type: merge.NodeOpMergeDefs, SourcePath: sourcePath,
		SourcePointer: "/$defs", TargetPointer: "/$defs",
	}}, merge.ApplyOptions{PruneUnreachableDefs: true})
	if err != nil {
		panic(err)
	}

	encoded, err := merge.Encode(merged, merge.FormatYAML)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
	// Output:
	// $defs:
	//     Config:
	//         type: object
	// $ref: '#/$defs/Config'
}
