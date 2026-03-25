// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	// ImportConflictReplace replaces existing target definition.
	ImportConflictReplace = "replace"
	// ImportConflictMerge merges source definition into existing target definition.
	ImportConflictMerge = "merge"
	// ImportConflictKeep keeps existing target definition and skips source.
	ImportConflictKeep = "keep"
	// ImportConflictError fails when target definition already exists.
	ImportConflictError = "error"
)

// DefsImportRename configures imported definition key rename strategy.
type DefsImportRename struct {
	// Mode selects rename strategy: none, prefix, suffix.
	Mode string

	// Value is prefix/suffix value for selected rename mode.
	Value string
}

// DefsImport describes one high-level `$defs` import plan.
type DefsImport struct {
	// SourcePath is source schema file path.
	SourcePath string

	// SourceDefs points to source defs object. Default is /$defs.
	SourceDefs string

	// TargetDefs points to target defs object. Default is /$defs.
	TargetDefs string

	// Rename configures imported definition name transformation.
	Rename DefsImportRename

	// Conflict selects conflict mode:
	// replace, merge, keep, error.
	Conflict string
}

// PlanImportsFile loads target schema file and expands imports into actions.
func PlanImportsFile(targetPath string, imports []DefsImport) ([]Action, error) {
	targetRoot, err := decodeFile(strings.TrimSpace(targetPath))
	if err != nil {
		return nil, fmt.Errorf("decode target schema %q: %w", targetPath, err)
	}

	return PlanImports(targetRoot, imports, FileLoader{})
}

// PlanImports expands high-level defs imports into low-level merge actions.
func PlanImports(targetRoot any, imports []DefsImport, loader SourceLoader) ([]Action, error) {
	if len(imports) == 0 {
		return nil, nil
	}

	if loader == nil {
		loader = FileLoader{}
	}

	actions := make([]Action, 0, len(imports)*8)
	planned := make(map[string]struct{}, len(imports)*8)
	for importIndex, item := range imports {
		sourcePath := strings.TrimSpace(item.SourcePath)
		if sourcePath == "" {
			return nil, fmt.Errorf("imports[%d].source_path is required", importIndex)
		}

		sourceDefsPointer := firstNonEmpty(
			strings.TrimSpace(item.SourceDefs),
			"/$defs",
		)
		targetDefsPointer := firstNonEmpty(
			strings.TrimSpace(item.TargetDefs),
			"/$defs",
		)
		conflictMode := firstNonEmpty(
			strings.TrimSpace(item.Conflict),
			ImportConflictError,
		)

		sourceRoot, err := loader.Load(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("imports[%d]: load source %q: %w", importIndex, sourcePath, err)
		}

		sourceDefsNode, err := nodeAtPointer(sourceRoot, sourceDefsPointer)
		if err != nil {
			return nil, fmt.Errorf("imports[%d]: source_defs %q: %w", importIndex, sourceDefsPointer, err)
		}

		sourceDefs, ok := sourceDefsNode.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("imports[%d]: source_defs %q must point to object", importIndex, sourceDefsPointer)
		}

		defNames := make([]string, 0, len(sourceDefs))
		for defName := range sourceDefs {
			defNames = append(defNames, defName)
		}
		sort.Strings(defNames)

		for _, sourceDefName := range defNames {
			targetDefName, err := applyImportRename(sourceDefName, item.Rename)
			if err != nil {
				return nil, fmt.Errorf("imports[%d]: rename %q: %w", importIndex, sourceDefName, err)
			}

			sourceDefPointer := joinPointer(sourceDefsPointer, sourceDefName)
			targetDefPointer := joinPointer(targetDefsPointer, targetDefName)
			targetExists := pointerExists(targetRoot, targetDefPointer)
			if _, ok := planned[targetDefPointer]; ok {
				targetExists = true
			}

			switch conflictMode {
			case ImportConflictError:
				if targetExists {
					return nil, fmt.Errorf(
						"imports[%d]: target %q already exists (definition %q)",
						importIndex,
						targetDefPointer,
						sourceDefName,
					)
				}

				actions = append(actions, Action{
					Type:          NodeOpReplace,
					SourcePath:    sourcePath,
					SourcePointer: sourceDefPointer,
					TargetPointer: targetDefPointer,
				})

			case ImportConflictKeep:
				if targetExists {
					continue
				}

				actions = append(actions, Action{
					Type:          NodeOpReplace,
					SourcePath:    sourcePath,
					SourcePointer: sourceDefPointer,
					TargetPointer: targetDefPointer,
				})

			case ImportConflictReplace:
				actions = append(actions, Action{
					Type:          NodeOpReplace,
					SourcePath:    sourcePath,
					SourcePointer: sourceDefPointer,
					TargetPointer: targetDefPointer,
				})

			case ImportConflictMerge:
				actions = append(actions, Action{
					Type:          NodeOpMerge,
					SourcePath:    sourcePath,
					SourcePointer: sourceDefPointer,
					TargetPointer: targetDefPointer,
					ObjectOp:      ObjectOpMerge,
					ArrayOp:       ArrayOpReplace,
				})

			default:
				return nil, fmt.Errorf(
					"imports[%d]: unsupported conflict mode %q",
					importIndex,
					conflictMode,
				)
			}

			planned[targetDefPointer] = struct{}{}
		}
	}

	return actions, nil
}

// applyImportRename applies one import rename strategy to source definition key.
func applyImportRename(sourceName string, rename DefsImportRename) (string, error) {
	sourceName = strings.TrimSpace(sourceName)
	if sourceName == "" {
		return "", errors.New("source definition name is empty")
	}

	mode := firstNonEmpty(strings.TrimSpace(rename.Mode), "none")
	value := strings.TrimSpace(rename.Value)
	switch mode {
	case "none":
		return sourceName, nil
	case "prefix":
		return value + sourceName, nil
	case "suffix":
		return sourceName + value, nil
	default:
		return "", fmt.Errorf("unsupported rename mode %q", mode)
	}
}

// pointerExists reports whether node exists at selected JSON pointer.
func pointerExists(root any, pointer string) bool {
	_, err := nodeAtPointer(root, pointer)
	return err == nil
}

// joinPointer appends escaped key to base pointer.
func joinPointer(base, key string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "/"
	}

	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}

	if base != "/" {
		base = strings.TrimSuffix(base, "/")
	}

	escaped := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
	if base == "/" {
		return "/" + escaped
	}

	return base + "/" + escaped
}
