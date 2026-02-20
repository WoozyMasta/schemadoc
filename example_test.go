// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateExampleJSONAllMode(t *testing.T) {
	t.Parallel()

	schema := buildExampleSchemaFixture(t)
	gotBytes, err := GenerateExampleJSON(schema, ExampleModeAll)
	if err != nil {
		t.Fatalf("GenerateExampleJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatalf("unmarshal generated json: %v", err)
	}

	want := map[string]any{
		"name":     "demo",
		"mode":     "safe",
		"count":    float64(0),
		"features": []any{"<string>"},
		"settings": map[string]any{
			"enabled": true,
			"note":    "<string>",
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("all mode mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestGenerateExampleJSONRequiredMode(t *testing.T) {
	t.Parallel()

	schema := buildExampleSchemaFixture(t)
	gotBytes, err := GenerateExampleJSON(schema, ExampleModeRequired)
	if err != nil {
		t.Fatalf("GenerateExampleJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatalf("unmarshal generated json: %v", err)
	}

	want := map[string]any{
		"name": "demo",
		"settings": map[string]any{
			"enabled": true,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("required mode mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestGenerateExampleYAMLRequiredMode(t *testing.T) {
	t.Parallel()

	schema := buildExampleSchemaFixture(t)
	gotBytes, err := GenerateExampleYAML(schema, ExampleModeRequired)
	if err != nil {
		t.Fatalf("GenerateExampleYAML: %v", err)
	}

	got := string(gotBytes)
	assertContains(t, got, "# Service Name")
	assertContains(t, got, "# Human-readable service name.")
	assertContains(t, got, "name: demo")
	assertContains(t, got, "settings:")
	assertContains(t, got, "# Enabled")
	assertContains(t, got, "# Enables processing pipeline.")
	assertContains(t, got, "enabled: true")
	assertNotContains(t, got, "mode:")
	assertNotContains(t, got, "count:")
}

func TestGenerateExampleJSONModeValidation(t *testing.T) {
	t.Parallel()

	schema := buildExampleSchemaFixture(t)
	_, err := GenerateExampleJSON(schema, "broken")
	if !errors.Is(err, ErrUnknownExampleMode) {
		t.Fatalf("expected ErrUnknownExampleMode, got: %v", err)
	}
}

func TestGenerateExampleJSONSupportsLocalDefinitionRefs(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type":     "object",
				"required": []any{"name"},
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
			},
		},
	})

	data, err := GenerateExampleJSON(schema, ExampleModeRequired)
	if err != nil {
		t.Fatalf("GenerateExampleJSON: %v", err)
	}

	if strings.TrimSpace(string(data)) != "{\n  \"name\": \"<string>\"\n}" {
		t.Fatalf("unexpected generated json:\n%s", string(data))
	}
}

// buildExampleSchemaFixture returns schema used across example generation tests.
func buildExampleSchemaFixture(t *testing.T) []byte {
	t.Helper()

	return minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type":     "object",
				"required": []any{"name", "settings"},
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"default":     "demo",
						"title":       "Service Name",
						"description": "Human-readable service name.",
					},
					"mode": map[string]any{
						"type":     "string",
						"examples": []any{"safe"},
					},
					"count": map[string]any{
						"type": "integer",
					},
					"features": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
					"settings": map[string]any{
						"type":     "object",
						"required": []any{"enabled"},
						"properties": map[string]any{
							"enabled": map[string]any{
								"type":        "boolean",
								"default":     true,
								"title":       "Enabled",
								"description": "Enables processing pipeline.",
							},
							"note": map[string]any{
								"type": "string",
							},
						},
					},
				},
			},
		},
	})
}
