// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import (
	"fmt"
)

// Apply executes actions over one in-memory schema root and returns merged root.
func Apply(root any, actions []Action, options ApplyOptions) (any, error) {
	workingRoot, err := cloneNode(root)
	if err != nil {
		return nil, fmt.Errorf("clone root: %w", err)
	}

	loader := options.Loader
	if loader == nil {
		loader = FileLoader{}
	}

	for index := range actions {
		action, err := normalizeAction(index, actions[index])
		if err != nil {
			return nil, err
		}

		if err := applyAction(&workingRoot, action, loader); err != nil {
			return nil, err
		}
	}

	if options.PruneUnreachableDefs {
		if err := pruneUnreachableDefs(&workingRoot); err != nil {
			return nil, err
		}
	}

	return workingRoot, nil
}

// File loads one source schema file and executes actions over it.
func File(
	sourcePath string,
	actions []Action,
	options ApplyOptions,
) (any, error) {
	loader := options.Loader
	if loader == nil {
		loader = FileLoader{}
	}

	if sourcePath == "" {
		return nil, ErrSourcePathRequired
	}

	root, err := loader.Load(sourcePath)
	if err != nil {
		return nil, err
	}

	options.Loader = loader
	return Apply(root, actions, options)
}

// Decode parses schema bytes by selected format.
func Decode(content []byte, format string) (any, error) {
	return decodeByFormat(content, format)
}

// DecodeFile parses schema file by extension with JSON/YAML fallback.
func DecodeFile(path string) (any, error) {
	return decodeFile(path)
}

// Encode serializes schema node by selected format.
func Encode(node any, format string) ([]byte, error) {
	return encodeNode(node, format)
}

// FileLoader loads source schemas from filesystem.
type FileLoader struct{}

// Load reads and decodes one schema from file path.
func (loader FileLoader) Load(path string) (any, error) {
	return decodeFile(path)
}
