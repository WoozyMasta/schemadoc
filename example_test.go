// SPDX-License-Identifier: MIT
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
	assertContains(t, got, "# Enables processing flow.")
	assertContains(t, got, "enabled: true")
	assertNotContains(t, got, "mode:")
	assertNotContains(t, got, "count:")
}

func TestGenerateExampleYAMLCommentsIncludeEnumValues(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"type": "string",
				"enum": []any{"safe", "fast", "debug"},
			},
		},
	})

	gotBytes, err := GenerateExampleYAML(schema, ExampleModeAll)
	if err != nil {
		t.Fatalf("GenerateExampleYAML: %v", err)
	}

	assertContains(t, string(gotBytes), "# Allowed values: safe, fast, debug")
}

func TestGenerateExampleYAMLDisableCommentsKeepsXOrder(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"first": map[string]any{
				"type":    "string",
				"x-order": 2,
			},
			"second": map[string]any{
				"type":    "string",
				"x-order": 1,
			},
		},
	})

	gotBytes, err := GenerateExampleYAMLWithOptions(schema, ExampleModeAll, ExampleOptions{
		YAMLComments: YAMLCommentPolicy{
			Titles:       boolPointer(false),
			Descriptions: boolPointer(false),
			Defaults:     boolPointer(false),
			Enums:        boolPointer(false),
			Examples:     YAMLCommentExamplesNone,
		},
	})
	if err != nil {
		t.Fatalf("GenerateExampleYAMLWithOptions: %v", err)
	}

	got := strings.TrimSpace(string(gotBytes))
	want := "second: <string>\nfirst: <string>"
	if got != want {
		t.Fatalf("unexpected generated yaml:\n%s\nwant:\n%s", got, want)
	}
	assertNotContains(t, got, "#")
}

func TestGenerateExampleYAMLCommentPolicy(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"settings": map[string]any{
				"type":        "object",
				"title":       "Settings",
				"description": "Runtime settings.",
				"default": map[string]any{
					"enabled": true,
					"labels":  []any{"demo"},
				},
				"examples": []any{
					map[string]any{
						"enabled": false,
						"labels":  []any{"example"},
					},
				},
			},
			"mode": map[string]any{
				"type": "string",
				"enum": []any{"safe", "fast"},
			},
		},
	})

	gotBytes, err := GenerateExampleYAMLWithOptions(schema, ExampleModeAll, ExampleOptions{
		YAMLComments: YAMLCommentPolicy{
			Examples:      YAMLCommentExamplesAll,
			ExampleFormat: YAMLCommentFormatBlock,
		},
	})
	if err != nil {
		t.Fatalf("GenerateExampleYAML: %v", err)
	}
	got := string(gotBytes)
	for _, marker := range []string{
		"# Settings",
		"# Runtime settings.",
		"# Default:",
		"#   enabled: true",
		"# Example:",
		"#   enabled: false",
		"# Allowed values: safe, fast",
		"settings:",
	} {
		assertContains(t, got, marker)
	}

	withoutExamples, err := GenerateExampleYAMLWithOptions(schema, ExampleModeAll, ExampleOptions{
		YAMLComments: YAMLCommentPolicy{
			Titles:       boolPointer(false),
			Descriptions: boolPointer(true),
			Defaults:     boolPointer(false),
			Enums:        boolPointer(true),
			Examples:     YAMLCommentExamplesNone,
		},
	})
	if err != nil {
		t.Fatalf("GenerateExampleYAMLWithOptions: %v", err)
	}
	withoutExamplesText := string(withoutExamples)
	assertNotContains(t, withoutExamplesText, "# Settings")
	assertContains(t, withoutExamplesText, "# Runtime settings.")
	assertNotContains(t, withoutExamplesText, "# Default:")
	assertNotContains(t, withoutExamplesText, "# Example:")
	assertContains(t, withoutExamplesText, "# Allowed values: safe, fast")
	assertContains(t, withoutExamplesText, "enabled: false")
}

func TestGenerateExampleYAMLCommentPolicyExamplesAndValues(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{
				"type":    "string",
				"example": "legacy",
				"examples": []any{
					"",
				},
				"enum": []any{"", false, 0},
			},
		},
	})

	gotBytes, err := GenerateExampleYAMLWithOptions(schema, ExampleModeAll, ExampleOptions{
		YAMLComments: YAMLCommentPolicy{
			Examples:      YAMLCommentExamplesAll,
			ExampleFormat: YAMLCommentFormatInline,
		},
	})
	if err != nil {
		t.Fatalf("GenerateExampleYAMLWithOptions: %v", err)
	}
	got := string(gotBytes)
	assertContains(t, got, `# Example: ""`)
	assertContains(t, got, `# Allowed values: "", false, 0`)
	assertNotContains(t, got, "legacy")
	assertContains(t, got, "value: \"\"")
}

func TestGenerateExampleYAMLTraversesDynamicMapsAndArrayTuples(t *testing.T) {
	t.Parallel()

	objectWithValueTitle := func(title string) map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{
					"type":  "string",
					"title": title,
				},
			},
		}
	}
	objectWithNameTitle := func(title string) map[string]any {
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":  "string",
					"title": title,
				},
			},
		}
	}

	modernSchema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dynamic": map[string]any{
				"type": "object",
				"patternProperties": map[string]any{
					"^x-": objectWithValueTitle("Pattern value"),
				},
				"additionalProperties": objectWithValueTitle("Additional value"),
				"examples": []any{
					map[string]any{
						"x-item": map[string]any{"value": "pattern"},
						"other":  map[string]any{"value": "additional"},
					},
				},
			},
			"items": map[string]any{
				"type": "array",
				"prefixItems": []any{
					objectWithNameTitle("First item name"),
					objectWithNameTitle("Second item name"),
				},
				"items": objectWithNameTitle("Trailing item name"),
				"examples": []any{
					[]any{
						map[string]any{"name": "first"},
						map[string]any{"name": "second"},
						map[string]any{"name": "trailing"},
					},
				},
			},
		},
	})

	gotBytes, err := GenerateExampleYAMLWithOptions(modernSchema, ExampleModeAll, ExampleOptions{
		YAMLComments: YAMLCommentPolicy{
			Examples:      YAMLCommentExamplesNone,
			ExampleFormat: YAMLCommentFormatBlock,
		},
	})
	if err != nil {
		t.Fatalf("GenerateExampleYAMLWithOptions: %v", err)
	}
	got := string(gotBytes)
	for _, marker := range []string{
		"# Pattern value",
		"# Additional value",
		"# First item name",
		"# Second item name",
		"# Trailing item name",
	} {
		assertContains(t, got, marker)
	}

	legacySchema := minimalSchemaBytes(t, map[string]any{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": []any{
					objectWithNameTitle("Legacy first name"),
					objectWithNameTitle("Legacy second name"),
				},
				"additionalItems": objectWithNameTitle("Legacy additional name"),
				"examples": []any{
					[]any{
						map[string]any{"name": "first"},
						map[string]any{"name": "second"},
						map[string]any{"name": "additional"},
					},
				},
			},
		},
	})

	gotBytes, err = GenerateExampleYAMLWithOptions(legacySchema, ExampleModeAll, ExampleOptions{
		YAMLComments: YAMLCommentPolicy{Examples: YAMLCommentExamplesNone},
	})
	if err != nil {
		t.Fatalf("GenerateExampleYAMLWithOptions legacy: %v", err)
	}
	got = string(gotBytes)
	for _, marker := range []string{
		"# Legacy first name",
		"# Legacy second name",
		"# Legacy additional name",
	} {
		assertContains(t, got, marker)
	}
}

func TestGenerateExampleYAMLCommentValuesPreserveText(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{
				"type":    "string",
				"default": "# literal\nfalse: 0\n\"quoted\": yes",
			},
		},
	})

	gotBytes, err := GenerateExampleYAMLWithOptions(schema, ExampleModeAll, ExampleOptions{})
	if err != nil {
		t.Fatalf("GenerateExampleYAMLWithOptions: %v", err)
	}
	got := string(gotBytes)
	assertContains(t, got, `# Default: # literal false: 0 "quoted": yes`)
	for _, marker := range []string{
		"value: |-",
		"  # literal",
		"  false: 0",
		"  \"quoted\": yes",
	} {
		assertContains(t, got, marker)
	}
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

func TestGenerateExampleJSONArrayUsesItemsExamples(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"languages": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":     "string",
							"examples": []any{"czech", "russian"},
						},
					},
				},
			},
		},
	})

	gotBytes, err := GenerateExampleJSON(schema, ExampleModeAll)
	if err != nil {
		t.Fatalf("GenerateExampleJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatalf("unmarshal generated json: %v", err)
	}

	want := []any{"czech", "russian"}
	if !reflect.DeepEqual(got["languages"], want) {
		t.Fatalf("languages example mismatch\ngot:  %#v\nwant: %#v", got["languages"], want)
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
								"description": "Enables processing flow.",
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
