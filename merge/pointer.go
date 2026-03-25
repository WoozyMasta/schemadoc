// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import (
	"fmt"
	"strconv"
	"strings"
)

// validatePointer validates JSON pointer syntax.
func validatePointer(pointer string) error {
	if pointer == "" {
		return nil
	}

	if !strings.HasPrefix(pointer, "/") {
		return fmt.Errorf("%w: %q", ErrInvalidJSONPointer, pointer)
	}

	tokens := strings.Split(pointer, "/")[1:]
	for _, token := range tokens {
		if !strings.Contains(token, "~") {
			continue
		}

		for index := 0; index < len(token); index++ {
			if token[index] != '~' {
				continue
			}

			if index+1 >= len(token) {
				return fmt.Errorf("%w: %q", ErrInvalidJSONPointer, pointer)
			}

			next := token[index+1]
			if next != '0' && next != '1' {
				return fmt.Errorf("%w: %q", ErrInvalidJSONPointer, pointer)
			}
		}
	}

	return nil
}

// pointerTokens returns decoded JSON pointer tokens.
func pointerTokens(pointer string) ([]string, error) {
	if err := validatePointer(pointer); err != nil {
		return nil, err
	}

	if pointer == "" {
		return nil, nil
	}

	rawTokens := strings.Split(pointer, "/")[1:]
	decoded := make([]string, len(rawTokens))
	for index := range rawTokens {
		decoded[index] = strings.ReplaceAll(
			strings.ReplaceAll(rawTokens[index], "~1", "/"),
			"~0",
			"~",
		)
	}

	return decoded, nil
}

// nodeAtPointer resolves one node by JSON pointer.
func nodeAtPointer(root any, pointer string) (any, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, err
	}

	current := root
	for _, token := range tokens {
		switch typed := current.(type) {
		case map[string]any:
			next, exists := typed[token]
			if !exists {
				return nil, fmt.Errorf("pointer %q: key %q not found", pointer, token)
			}

			current = next
		case []any:
			index, parseErr := parseIndexToken(token, len(typed))
			if parseErr != nil {
				return nil, fmt.Errorf("pointer %q: %w", pointer, parseErr)
			}

			current = typed[index]
		default:
			return nil, fmt.Errorf("pointer %q: cannot step into %T", pointer, current)
		}
	}

	return current, nil
}

// setNodeAtPointer sets value by JSON pointer.
func setNodeAtPointer(root any, pointer string, value any) error {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return err
	}

	if len(tokens) == 0 {
		target, ok := root.(*any)
		if !ok {
			return errSetRootExpectedAny
		}

		*target = value
		return nil
	}

	current := root
	if typed, ok := current.(*any); ok {
		current = *typed
	}

	for index := 0; index < len(tokens)-1; index++ {
		token := tokens[index]

		switch typed := current.(type) {
		case *any:
			current = *typed
		case map[string]any:
			next, exists := typed[token]
			if !exists {
				created := make(map[string]any)
				typed[token] = created
				current = created
				continue
			}

			current = next
		case []any:
			itemIndex, parseErr := parseIndexToken(token, len(typed))
			if parseErr != nil {
				return fmt.Errorf("pointer %q: %w", pointer, parseErr)
			}

			current = typed[itemIndex]
		default:
			return fmt.Errorf("pointer %q: cannot step into %T", pointer, current)
		}
	}

	lastToken := tokens[len(tokens)-1]
	switch typed := current.(type) {
	case *any:
		object, ok := (*typed).(map[string]any)
		if !ok {
			return fmt.Errorf("pointer %q: cannot set key on %T", pointer, *typed)
		}

		object[lastToken] = value
	case map[string]any:
		typed[lastToken] = value
	case []any:
		itemIndex, parseErr := parseIndexToken(lastToken, len(typed))
		if parseErr != nil {
			return fmt.Errorf("pointer %q: %w", pointer, parseErr)
		}

		typed[itemIndex] = value
	default:
		return fmt.Errorf("pointer %q: cannot set value on %T", pointer, current)
	}

	return nil
}

// parseIndexToken parses array index token.
func parseIndexToken(token string, length int) (int, error) {
	if token == "-" {
		return 0, errAppendTokenNotSupported
	}

	index, err := strconv.Atoi(token)
	if err != nil {
		return 0, fmt.Errorf("invalid array index %q", token)
	}

	if index < 0 || index >= length {
		return 0, fmt.Errorf("array index %d out of range", index)
	}

	return index, nil
}

// normalizeTargetPointer normalizes target pointer for replace/merge ops.
func normalizeTargetPointer(pointer string) string {
	trimmed := strings.TrimSpace(pointer)
	if trimmed == "" {
		return ""
	}

	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}

	return trimmed
}

// normalizeDefsTargetPointer normalizes target pointer for defs merge.
func normalizeDefsTargetPointer(pointer string) string {
	trimmed := strings.TrimSpace(pointer)
	if trimmed == "" {
		return "/$defs"
	}

	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}

	return trimmed
}
