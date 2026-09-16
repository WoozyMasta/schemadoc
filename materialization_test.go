// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"errors"
	"strings"
	"testing"
)

func TestMaterializationCandidateOrderAndValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema map[string]any
		want   string
	}{
		{
			name: "const before examples",
			schema: map[string]any{
				"type":     "integer",
				"const":    3,
				"examples": []any{4},
				"default":  5,
				"enum":     []any{3, 6},
			},
			want: "3",
		},
		{
			name: "valid example before default",
			schema: map[string]any{
				"type":     "string",
				"examples": []any{"valid", "later"},
				"default":  "default",
			},
			want: `"valid"`,
		},
		{
			name: "invalid example falls through",
			schema: map[string]any{
				"type":      "string",
				"examples":  []any{"too long"},
				"default":   "ok",
				"maxLength": 2,
			},
			want: `"ok"`,
		},
		{
			name: "singular example is ignored",
			schema: map[string]any{
				"type":    "string",
				"example": "legacy",
			},
			want: `"<string>"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := GenerateExampleJSON(minimalSchemaBytes(t, test.schema), ExampleModeAll)
			if err != nil {
				t.Fatalf("GenerateExampleJSON: %v", err)
			}
			if strings.TrimSpace(string(got)) != test.want {
				t.Fatalf("example = %s, want %s", got, test.want)
			}
		})
	}
}

func TestMaterializationWholeValueCandidates(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"default": map[string]any{
			"name": "from default",
		},
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	})

	got, err := GenerateExampleJSON(schema, ExampleModeRequired)
	if err != nil {
		t.Fatalf("GenerateExampleJSON: %v", err)
	}
	if strings.TrimSpace(string(got)) != "{\n  \"name\": \"from default\"\n}" {
		t.Fatalf("example = %s", got)
	}
}

func TestMaterializationStructuredErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   []byte
		category MaterializationErrorCategory
		code     MaterializationErrorCode
		path     string
		is       error
	}{
		{
			name:     "false schema",
			schema:   []byte(`false`),
			category: MaterializationCategoryUnsatisfiableSchema,
			code:     MaterializationCodeUnsatisfiable,
			path:     "#",
			is:       ErrMaterializationUnsatisfiable,
		},
		{
			name:     "external reference",
			schema:   minimalSchemaBytes(t, map[string]any{"$ref": "https://example.invalid/schema.json"}),
			category: MaterializationCategoryUnsupportedExternalReference,
			code:     MaterializationCodeExternalReference,
			path:     "#/$ref",
			is:       ErrExternalSchemaReference,
		},
		{
			name:     "missing local reference",
			schema:   minimalSchemaBytes(t, map[string]any{"$ref": "#/$defs/Missing"}),
			category: MaterializationCategoryUnresolvedReference,
			code:     MaterializationCodeUnresolvedReference,
			path:     "#/$ref",
			is:       ErrUnresolvedSchemaReference,
		},
		{
			name: "recursive reference",
			schema: minimalSchemaBytes(t, map[string]any{
				"$ref": "#/$defs/Node",
				"$defs": map[string]any{
					"Node": map[string]any{
						"type":     "object",
						"required": []any{"next"},
						"properties": map[string]any{
							"next": map[string]any{"$ref": "#/$defs/Node"},
						},
					},
				},
			}),
			category: MaterializationCategoryReferenceCycle,
			code:     MaterializationCodeReferenceRecursion,
			path:     "#/next/$ref",
			is:       ErrSchemaReferenceCycle,
		},
		{
			name: "contradictory allOf",
			schema: minimalSchemaBytes(t, map[string]any{
				"allOf": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "number"},
				},
			}),
			category: MaterializationCategoryUnsatisfiableSchema,
			code:     MaterializationCodeUnsatisfiable,
			path:     "#/type",
			is:       ErrMaterializationUnsatisfiable,
		},
		{
			name: "unsupported pattern synthesis",
			schema: minimalSchemaBytes(t, map[string]any{
				"type":    "string",
				"pattern": "^(?:[A-Z]{3}-){2}[0-9]{4}$",
			}),
			category: MaterializationCategoryUnsupportedMaterialization,
			code:     MaterializationCodeUnsupported,
			path:     "#/pattern",
			is:       ErrUnsupportedMaterialization,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := GenerateExampleJSON(test.schema, ExampleModeAll)
			if err == nil {
				t.Fatal("GenerateExampleJSON unexpectedly succeeded")
			}
			var materialization *MaterializationError
			if !errors.As(err, &materialization) {
				t.Fatalf("error = %T %v, want MaterializationError", err, err)
			}
			if materialization.Category != test.category || materialization.Code != test.code || materialization.Path != test.path {
				t.Fatalf("error = %+v, want category=%q code=%q path=%q", materialization, test.category, test.code, test.path)
			}
			if !errors.Is(err, test.is) {
				t.Fatalf("errors.Is(%v, %v) = false", err, test.is)
			}
		})
	}
}

func TestMaterializationEvaluatesCompositionBranches(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"anyOf": []any{
			map[string]any{"type": "number", "const": "invalid"},
			map[string]any{"type": "string", "default": "selected"},
		},
	})
	got, err := GenerateExampleJSON(schema, ExampleModeAll)
	if err != nil {
		t.Fatalf("GenerateExampleJSON: %v", err)
	}
	if strings.TrimSpace(string(got)) != `"selected"` {
		t.Fatalf("example = %s, want selected branch", got)
	}
}
