// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// decodeByFormat decodes schema bytes in selected format.
func decodeByFormat(content []byte, format string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatJSON:
		return decodeJSON(content)
	case FormatYAML:
		return decodeYAML(content)
	default:
		return nil, fmt.Errorf("unsupported input format %q", format)
	}
}

// decodeFile decodes JSON or YAML schema file into dynamic node tree.
func decodeFile(path string) (any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema %q: %w", path, err)
	}

	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".json":
		return decodeJSON(content)

	case ".yaml", ".yml":
		return decodeYAML(content)

	default:
		node, decodeErr := decodeJSON(content)
		if decodeErr == nil {
			return node, nil
		}

		node, decodeErr = decodeYAML(content)
		if decodeErr == nil {
			return node, nil
		}

		return nil, fmt.Errorf("decode schema %q: unsupported format", path)
	}
}

// decodeJSON decodes JSON bytes to dynamic node tree.
func decodeJSON(content []byte) (any, error) {
	var node any
	if err := json.Unmarshal(content, &node); err != nil {
		return nil, err
	}

	return node, nil
}

// decodeYAML decodes YAML bytes and normalizes map keys to string.
func decodeYAML(content []byte) (any, error) {
	var node any
	if err := yaml.Unmarshal(content, &node); err != nil {
		return nil, err
	}

	return normalizeYAMLNode(node), nil
}

// normalizeYAMLNode recursively converts YAML map keys to strings.
func normalizeYAMLNode(node any) any {
	switch typed := node.(type) {
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, value := range typed {
			normalized[fmt.Sprint(key)] = normalizeYAMLNode(value)
		}

		return normalized
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, value := range typed {
			normalized[key] = normalizeYAMLNode(value)
		}

		return normalized
	case []any:
		for index := range typed {
			typed[index] = normalizeYAMLNode(typed[index])
		}

		return typed
	default:
		return node
	}
}

// encodeNode encodes schema node in selected output format.
func encodeNode(node any, format string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatYAML:
		encoded, err := yaml.Marshal(node)
		if err != nil {
			return nil, fmt.Errorf("encode yaml: %w", err)
		}

		return encoded, nil
	case "", FormatJSON:
		encoded, err := json.MarshalIndent(node, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode json: %w", err)
		}

		return append(encoded, '\n'), nil
	default:
		return nil, fmt.Errorf("unsupported output format %q", format)
	}
}
