// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import (
	"fmt"
	"strings"
)

// normalizeAction validates and applies defaults to one action.
func normalizeAction(index int, action Action) (Action, error) {
	action.Type = firstNonEmpty(strings.TrimSpace(action.Type), NodeOpReplace)
	action.SourcePath = strings.TrimSpace(action.SourcePath)
	action.SourcePointer = strings.TrimSpace(action.SourcePointer)
	action.TargetPointer = strings.TrimSpace(action.TargetPointer)
	action.ObjectOp = firstNonEmpty(strings.TrimSpace(action.ObjectOp), ObjectOpMerge)
	action.ArrayOp = firstNonEmpty(strings.TrimSpace(action.ArrayOp), ArrayOpReplace)

	if action.Source == nil && action.SourcePath == "" {
		return Action{}, fmt.Errorf("action[%d]: source path or source value is required", index)
	}

	if err := validateNodeOp(action.Type); err != nil {
		return Action{}, fmt.Errorf("action[%d]: %w", index, err)
	}

	if err := validatePointer(action.SourcePointer); err != nil {
		return Action{}, fmt.Errorf("action[%d]: invalid source pointer: %w", index, err)
	}

	if err := validatePointer(action.TargetPointer); err != nil {
		return Action{}, fmt.Errorf("action[%d]: invalid target pointer: %w", index, err)
	}

	if err := validateObjectOp(action.ObjectOp); err != nil {
		return Action{}, fmt.Errorf("action[%d]: %w", index, err)
	}

	if err := validateArrayOp(action.ArrayOp); err != nil {
		return Action{}, fmt.Errorf("action[%d]: %w", index, err)
	}

	return action, nil
}

// validateArrayOp validates supported array mode value.
func validateArrayOp(mode string) error {
	switch mode {
	case ArrayOpReplace, ArrayOpAppend, ArrayOpAppendUnique:
		return nil
	default:
		return fmt.Errorf("unsupported array merge mode %q", mode)
	}
}

// validateNodeOp validates supported node operation value.
func validateNodeOp(operation string) error {
	switch operation {
	case NodeOpReplace, NodeOpMerge, NodeOpMergeDefs:
		return nil
	default:
		return fmt.Errorf("unsupported node operation %q", operation)
	}
}

// firstNonEmpty returns first non-empty value.
func firstNonEmpty(values ...string) string {
	for index := range values {
		if strings.TrimSpace(values[index]) != "" {
			return values[index]
		}
	}

	return ""
}

// validateObjectOp validates supported object mode value.
func validateObjectOp(mode string) error {
	switch mode {
	case ObjectOpMerge, ObjectOpReplace:
		return nil
	default:
		return fmt.Errorf("unsupported object merge mode %q", mode)
	}
}
