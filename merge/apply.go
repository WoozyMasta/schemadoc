// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import (
	"encoding/json"
	"fmt"
)

// cloneNode deep-copies dynamic node tree.
func cloneNode(node any) (any, error) {
	raw, err := json.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("clone encode: %w", err)
	}

	var cloned any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, fmt.Errorf("clone decode: %w", err)
	}

	return cloned, nil
}

// mergeObjects recursively merges source object into destination object.
func mergeObjects(
	destination, source map[string]any,
	arrayMode string,
	objectMode string,
) error {
	for key, sourceValue := range source {
		destinationValue, exists := destination[key]
		if !exists {
			cloned, err := cloneNode(sourceValue)
			if err != nil {
				return err
			}

			destination[key] = cloned
			continue
		}

		switch sourceTyped := sourceValue.(type) {
		case map[string]any:
			destinationTyped, ok := destinationValue.(map[string]any)
			if !ok {
				cloned, err := cloneNode(sourceValue)
				if err != nil {
					return err
				}

				destination[key] = cloned
				continue
			}

			switch objectMode {
			case ObjectOpReplace:
				cloned, err := cloneNode(sourceValue)
				if err != nil {
					return err
				}

				destination[key] = cloned
			case ObjectOpMerge:
				if err := mergeObjects(
					destinationTyped,
					sourceTyped,
					arrayMode,
					objectMode,
				); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported object merge mode %q", objectMode)
			}
		case []any:
			destinationSlice, ok := destinationValue.([]any)
			if !ok {
				cloned, err := cloneNode(sourceValue)
				if err != nil {
					return err
				}

				destination[key] = cloned
				continue
			}

			mergedSlice, err := mergeArrays(destinationSlice, sourceTyped, arrayMode)
			if err != nil {
				return err
			}

			destination[key] = mergedSlice
		default:
			cloned, err := cloneNode(sourceValue)
			if err != nil {
				return err
			}

			destination[key] = cloned
		}
	}

	return nil
}

// mergeArrays merges source and destination arrays by selected mode.
func mergeArrays(destination, source []any, mode string) ([]any, error) {
	switch mode {
	case ArrayOpReplace, "":
		cloned, err := cloneNode(source)
		if err != nil {
			return nil, err
		}

		result, ok := cloned.([]any)
		if !ok {
			return nil, errCloneArrayExpectedSlice
		}

		return result, nil
	case ArrayOpAppend:
		result := make([]any, 0, len(destination)+len(source))
		result = append(result, destination...)
		for _, item := range source {
			cloned, err := cloneNode(item)
			if err != nil {
				return nil, err
			}

			result = append(result, cloned)
		}

		return result, nil
	case ArrayOpAppendUnique:
		result := make([]any, 0, len(destination)+len(source))
		result = append(result, destination...)
		seen := make(map[string]struct{}, len(result))
		for _, item := range result {
			key, err := uniqueNodeKey(item)
			if err != nil {
				return nil, err
			}

			seen[key] = struct{}{}
		}

		for _, item := range source {
			key, err := uniqueNodeKey(item)
			if err != nil {
				return nil, err
			}

			if _, exists := seen[key]; exists {
				continue
			}

			cloned, err := cloneNode(item)
			if err != nil {
				return nil, err
			}

			result = append(result, cloned)
			seen[key] = struct{}{}
		}

		return result, nil
	default:
		return nil, fmt.Errorf("unsupported array merge mode %q", mode)
	}
}

// uniqueNodeKey creates stable key for unique array merge mode.
func uniqueNodeKey(node any) (string, error) {
	encoded, err := json.Marshal(node)
	if err != nil {
		return "", fmt.Errorf("encode node key: %w", err)
	}

	return string(encoded), nil
}

// applyAction applies one prepared merge action.
func applyAction(root *any, action Action, loader SourceLoader) error {
	switch action.Type {
	case NodeOpReplace:
		return applyReplace(root, action, loader)
	case NodeOpMerge:
		return applyMerge(root, action, loader)
	case NodeOpMergeDefs:
		return applyMergeDefs(root, action, loader)
	default:
		return fmt.Errorf("unsupported operation kind %q", action.Type)
	}
}

// applyReplace applies replace action.
func applyReplace(root *any, action Action, loader SourceLoader) error {
	sourceNode, err := resolveSourceNode(action, loader)
	if err != nil {
		return fmt.Errorf("replace %q: %w", action.TargetPointer, err)
	}

	cloned, err := cloneNode(sourceNode)
	if err != nil {
		return err
	}

	if err := setNodeAtPointer(
		root,
		normalizeTargetPointer(action.TargetPointer),
		cloned,
	); err != nil {
		return fmt.Errorf("replace %q: %w", action.TargetPointer, err)
	}

	return nil
}

// applyMerge applies deep merge action.
func applyMerge(root *any, action Action, loader SourceLoader) error {
	targetPointer := normalizeTargetPointer(action.TargetPointer)
	sourceNode, err := resolveSourceNode(action, loader)
	if err != nil {
		return fmt.Errorf("merge %q: %w", targetPointer, err)
	}

	sourceObject, ok := sourceNode.(map[string]any)
	if !ok {
		return fmt.Errorf(
			"merge %q: %w, got %T",
			targetPointer,
			ErrExpectedObject,
			sourceNode,
		)
	}

	targetNode, err := nodeAtPointer(*root, targetPointer)
	if err != nil {
		empty := make(map[string]any)
		if setErr := setNodeAtPointer(root, targetPointer, empty); setErr != nil {
			return fmt.Errorf("merge %q: create target: %w", targetPointer, setErr)
		}

		targetNode = empty
	}

	targetObject, ok := targetNode.(map[string]any)
	if !ok {
		return fmt.Errorf(
			"merge %q: %w, got %T",
			targetPointer,
			ErrExpectedObject,
			targetNode,
		)
	}

	clonedSource, err := cloneNode(sourceObject)
	if err != nil {
		return err
	}

	sourceForMerge, ok := clonedSource.(map[string]any)
	if !ok {
		return fmt.Errorf("merge %q: clone source: expected object", targetPointer)
	}

	if err := mergeObjects(
		targetObject,
		sourceForMerge,
		action.ArrayOp,
		action.ObjectOp,
	); err != nil {
		return fmt.Errorf("merge %q: %w", targetPointer, err)
	}

	return nil
}

// applyMergeDefs merges source object fields into target defs object.
func applyMergeDefs(root *any, action Action, loader SourceLoader) error {
	targetPointer := normalizeDefsTargetPointer(action.TargetPointer)
	sourceNode, err := resolveSourceNode(action, loader)
	if err != nil {
		return fmt.Errorf("merge-defs %q: %w", targetPointer, err)
	}

	sourceObject, ok := sourceNode.(map[string]any)
	if !ok {
		return fmt.Errorf(
			"merge-defs %q: %w, got %T",
			targetPointer,
			ErrExpectedObject,
			sourceNode,
		)
	}

	targetNode, err := nodeAtPointer(*root, targetPointer)
	if err != nil {
		empty := make(map[string]any)
		if setErr := setNodeAtPointer(root, targetPointer, empty); setErr != nil {
			return fmt.Errorf("merge-defs %q: create target: %w", targetPointer, setErr)
		}

		targetNode = empty
	}

	targetObject, ok := targetNode.(map[string]any)
	if !ok {
		return fmt.Errorf(
			"merge-defs %q: %w, got %T",
			targetPointer,
			ErrExpectedObject,
			targetNode,
		)
	}

	if err := mergeObjects(targetObject, sourceObject, action.ArrayOp, action.ObjectOp); err != nil {
		return fmt.Errorf("merge-defs %q: %w", targetPointer, err)
	}

	return nil
}

// resolveSourceNode loads source schema and resolves optional pointer.
func resolveSourceNode(action Action, loader SourceLoader) (any, error) {
	rootNode, err := resolveSourceRoot(action, loader)
	if err != nil {
		return nil, err
	}

	pointer := action.SourcePointer
	if pointer == "" {
		return rootNode, nil
	}

	node, err := nodeAtPointer(rootNode, pointer)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// resolveSourceRoot resolves source root from in-memory value or loader.
func resolveSourceRoot(action Action, loader SourceLoader) (any, error) {
	if action.Source != nil {
		return action.Source, nil
	}

	if loader == nil {
		loader = FileLoader{}
	}

	return loader.Load(action.SourcePath)
}
