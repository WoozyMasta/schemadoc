// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGeneratedExampleJSONGolden(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join("testdata", "generated", "app.schema.json")
	wantPath := filepath.Join("testdata", "generated", "app.config.json")

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}

	got, err := GenerateExample(schemaBytes, ExampleModeAll, ExampleFormatJSON)
	if err != nil {
		t.Fatalf("GenerateExample(json): %v", err)
	}

	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read golden example json: %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf("generated json example differs from golden %q", wantPath)
	}
}

func TestGeneratedExampleYAMLGolden(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join("testdata", "generated", "app.schema.json")
	wantPath := filepath.Join("testdata", "generated", "app.config.yaml")

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}

	got, err := GenerateExample(schemaBytes, ExampleModeAll, ExampleFormatYAML)
	if err != nil {
		t.Fatalf("GenerateExample(yaml): %v", err)
	}

	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read golden example yaml: %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf("generated yaml example differs from golden %q", wantPath)
	}
}

func TestGeneratedSchemaYAMLGolden(t *testing.T) {
	t.Parallel()

	jsonPath := filepath.Join("testdata", "generated", "base.schema.json")
	yamlPath := filepath.Join("testdata", "generated", "base.schema.yaml")

	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read schema json fixture: %v", err)
	}

	yamlBytes, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read schema yaml fixture: %v", err)
	}

	var want any
	if err := json.Unmarshal(jsonBytes, &want); err != nil {
		t.Fatalf("decode schema json fixture: %v", err)
	}

	var yamlValue any
	if err := yaml.Unmarshal(yamlBytes, &yamlValue); err != nil {
		t.Fatalf("decode schema yaml fixture: %v", err)
	}

	normalizedYAML := normalizeYAMLMap(yamlValue)
	normalizedYAMLBytes, err := json.Marshal(normalizedYAML)
	if err != nil {
		t.Fatalf("normalize schema yaml fixture: %v", err)
	}

	var got any
	if err := json.Unmarshal(normalizedYAMLBytes, &got); err != nil {
		t.Fatalf("decode normalized schema yaml fixture: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schema yaml fixture differs from schema json fixture")
	}
}

// normalizeYAMLMap converts yaml.v3 decoded maps into JSON-compatible maps.
func normalizeYAMLMap(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[key] = normalizeYAMLMap(item)
		}
		return normalized
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized[keyToString(key)] = normalizeYAMLMap(item)
		}
		return normalized
	case []any:
		out := make([]any, len(typed))
		for index := range typed {
			out[index] = normalizeYAMLMap(typed[index])
		}
		return out
	default:
		return typed
	}
}

// keyToString converts YAML map keys to string values for JSON normalization.
func keyToString(value any) string {
	return fmt.Sprint(value)
}
