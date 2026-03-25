// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import (
	"fmt"
	"strings"
)

// pruneUnreachableDefs removes entries in $defs not reachable by refs.
func pruneUnreachableDefs(root *any) error {
	rootObject, ok := (*root).(map[string]any)
	if !ok {
		return fmt.Errorf("prune defs: %w, got %T", ErrExpectedObject, *root)
	}

	defsNode, exists := rootObject["$defs"]
	if !exists {
		return nil
	}

	defsObject, ok := defsNode.(map[string]any)
	if !ok {
		return fmt.Errorf("prune defs: $defs: %w, got %T", ErrExpectedObject, defsNode)
	}

	reachable := collectReachableDefs(rootObject, defsObject)
	for name := range defsObject {
		if _, keep := reachable[name]; keep {
			continue
		}

		delete(defsObject, name)
	}

	return nil
}

// collectReachableDefs computes transitive closure for local #/$defs refs.
func collectReachableDefs(root, defs map[string]any) map[string]struct{} {
	reachable := make(map[string]struct{}, len(defs))
	pending := make([]string, 0, len(defs))

	collectRefs(root, true, func(defName string) {
		if _, known := reachable[defName]; known {
			return
		}

		reachable[defName] = struct{}{}
		pending = append(pending, defName)
	})

	for len(pending) > 0 {
		currentName := pending[0]
		pending = pending[1:]

		currentDef, exists := defs[currentName]
		if !exists {
			continue
		}

		collectRefs(currentDef, false, func(defName string) {
			if _, known := reachable[defName]; known {
				return
			}

			reachable[defName] = struct{}{}
			pending = append(pending, defName)
		})
	}

	return reachable
}

// collectRefs walks node tree and emits local defs ref names.
func collectRefs(node any, skipDefsBranch bool, emit func(defName string)) {
	switch typed := node.(type) {
	case map[string]any:
		if refValue, ok := typed["$ref"].(string); ok {
			if defName, resolved := localRefDefName(refValue); resolved {
				emit(defName)
			}
		}

		for key, value := range typed {
			if skipDefsBranch && key == "$defs" {
				continue
			}

			collectRefs(value, false, emit)
		}
	case []any:
		for _, item := range typed {
			collectRefs(item, false, emit)
		}
	}
}

// localRefDefName extracts defs entry name from local $ref pointer.
func localRefDefName(refValue string) (string, bool) {
	if !strings.HasPrefix(refValue, "#/$defs/") {
		return "", false
	}

	tail := strings.TrimPrefix(refValue, "#/$defs/")
	if tail == "" {
		return "", false
	}

	parts := strings.SplitN(tail, "/", 2)
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", false
	}

	return name, true
}
