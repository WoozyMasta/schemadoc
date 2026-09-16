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

func TestMaterializationScalarConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema map[string]any
		want   string
	}{
		{
			name: "bounded string",
			schema: map[string]any{
				"type":      "string",
				"minLength": 3,
				"maxLength": 3,
			},
			want: `"aaa"`,
		},
		{
			name: "unicode length",
			schema: map[string]any{
				"type":      "string",
				"examples":  []any{"é"},
				"minLength": 2,
				"maxLength": 2,
			},
			want: `"aa"`,
		},
		{
			name: "simple pattern",
			schema: map[string]any{
				"type":    "string",
				"pattern": "^[0-9]+$",
			},
			want: `"0"`,
		},
		{
			name: "format example",
			schema: map[string]any{
				"type":   "string",
				"format": "email",
			},
			want: `"user@example.com"`,
		},
		{
			name: "integer multiple",
			schema: map[string]any{
				"type":       "integer",
				"minimum":    1,
				"maximum":    5,
				"multipleOf": 2,
			},
			want: "2",
		},
		{
			name: "exclusive integer bounds",
			schema: map[string]any{
				"type":             "integer",
				"exclusiveMinimum": 1,
				"exclusiveMaximum": 3,
			},
			want: "2",
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

func TestMaterializationLargeStringConstraintDoesNotAllocate(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type":      "string",
		"minLength": maxGeneratedStringLength + 1,
		"maxLength": maxGeneratedStringLength + 2,
	})

	_, err := GenerateExampleJSON(schema, ExampleModeAll)
	if !errors.Is(err, ErrUnsupportedMaterialization) {
		t.Fatalf("error = %v, want ErrUnsupportedMaterialization", err)
	}
}

func TestMaterializationImpossibleMultipleOf(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type":       "number",
		"minimum":    0.1,
		"maximum":    0.2,
		"multipleOf": 0.3,
	})

	_, err := GenerateExampleJSON(schema, ExampleModeAll)
	if !errors.Is(err, ErrMaterializationUnsatisfiable) {
		t.Fatalf("error = %v, want ErrMaterializationUnsatisfiable", err)
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

func TestMaterializationObjectsAndDynamicMaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema map[string]any
		want   string
	}{
		{
			name: "nested map",
			schema: map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type":     "object",
					"required": []any{"enabled"},
					"properties": map[string]any{
						"enabled": map[string]any{"type": "boolean"},
					},
				},
			},
			want: "{\n  \"example\": {\n    \"enabled\": false\n  }\n}",
		},
		{
			name: "property name example",
			schema: map[string]any{
				"type": "object",
				"propertyNames": map[string]any{
					"type":     "string",
					"examples": []any{"tenant"},
				},
				"additionalProperties": map[string]any{"type": "string"},
			},
			want: "{\n  \"tenant\": \"<string>\"\n}",
		},
		{
			name: "forbidden pattern key is not invented",
			schema: map[string]any{
				"type": "object",
				"propertyNames": map[string]any{
					"type":    "string",
					"pattern": "^x-",
				},
				"patternProperties": map[string]any{
					"^x-": map[string]any{"type": "string"},
				},
				"additionalProperties": map[string]any{"type": "string"},
			},
			want: "{}",
		},
		{
			name: "additional properties false",
			schema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			},
			want: "{}",
		},
		{
			name: "overlapping object terms",
			schema: map[string]any{
				"allOf": []any{
					map[string]any{
						"type": "object",
						"properties": map[string]any{
							"declared": map[string]any{"type": "string"},
						},
					},
					map[string]any{
						"type":                 "object",
						"additionalProperties": false,
					},
				},
			},
			want: "{}",
		},
		{
			name: "optional false property is omitted",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"forbidden": false,
					"visible":   map[string]any{"type": "string"},
				},
			},
			want: "{\n  \"visible\": \"<string>\"\n}",
		},
		{
			name: "dynamic entry respects bounds",
			schema: map[string]any{
				"type":          "object",
				"minProperties": 1,
				"maxProperties": 1,
				"propertyNames": map[string]any{
					"type":     "string",
					"examples": []any{"tenant"},
				},
				"additionalProperties": map[string]any{"type": "integer"},
			},
			want: "{\n  \"tenant\": 0\n}",
		},
	}

	for _, test := range tests {
		test := test
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

func TestMaterializationRejectsForbiddenRequiredProperty(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type":     "object",
		"required": []any{"forbidden"},
		"properties": map[string]any{
			"forbidden": false,
		},
	})

	_, err := GenerateExampleJSON(schema, ExampleModeAll)
	if !errors.Is(err, ErrMaterializationUnsatisfiable) {
		t.Fatalf("GenerateExampleJSON error = %v, want unsatisfiable", err)
	}
}

func TestMaterializationRejectsUnmaterializableNestedMap(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type":          "object",
		"minProperties": 1,
		"additionalProperties": map[string]any{
			"type":     "object",
			"required": []any{"forbidden"},
			"properties": map[string]any{
				"forbidden": false,
			},
		},
	})

	_, err := GenerateExampleJSON(schema, ExampleModeAll)
	if !errors.Is(err, ErrUnsupportedMaterialization) {
		t.Fatalf("GenerateExampleJSON error = %v, want unsupported materialization", err)
	}
}

func TestMaterializationRejectsContradictoryObjectBounds(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type":          "object",
		"minProperties": 2,
		"maxProperties": 1,
	})

	_, err := GenerateExampleJSON(schema, ExampleModeAll)
	if !errors.Is(err, ErrMaterializationUnsatisfiable) {
		t.Fatalf("GenerateExampleJSON error = %v, want unsatisfiable", err)
	}
}

func TestMaterializationArraysAndTupleSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema map[string]any
		want   string
	}{
		{
			name: "modern heterogeneous tuple",
			schema: map[string]any{
				"type": "array",
				"prefixItems": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "integer"},
				},
				"items":    map[string]any{"type": "boolean"},
				"minItems": 2,
			},
			want: "[\n  \"<string>\",\n  0\n]",
		},
		{
			name: "legacy heterogeneous tuple",
			schema: map[string]any{
				"$schema": "http://json-schema.org/draft-04/schema#",
				"type":    "array",
				"items": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "integer"},
				},
				"additionalItems": false,
				"minItems":        2,
				"maxItems":        2,
			},
			want: "[\n  \"<string>\",\n  0\n]",
		},
		{
			name: "modern tuple may be shorter than prefix",
			schema: map[string]any{
				"type": "array",
				"prefixItems": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "integer"},
				},
				"maxItems": 1,
			},
			want: "[\n  \"<string>\"\n]",
		},
		{
			name: "legacy tuple may be shorter than tuple",
			schema: map[string]any{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type":    "array",
				"items": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "integer"},
				},
				"maxItems": 1,
			},
			want: "[\n  \"<string>\"\n]",
		},
		{
			name: "tuple minimum does not require unmaterializable positions",
			schema: map[string]any{
				"type": "array",
				"prefixItems": []any{
					map[string]any{"type": "string"},
					false,
				},
				"items":    false,
				"minItems": 1,
			},
			want: "[\n  \"<string>\"\n]",
		},
		{
			name: "tuple minimum uses valid trailing schema",
			schema: map[string]any{
				"type": "array",
				"prefixItems": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "integer"},
				},
				"items":    map[string]any{"type": "boolean"},
				"minItems": 3,
				"maxItems": 3,
			},
			want: "[\n  \"<string>\",\n  0,\n  false\n]",
		},
		{
			name: "contains replaces compatible tuple position",
			schema: map[string]any{
				"type": "array",
				"prefixItems": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "integer"},
				},
				"contains": map[string]any{"const": "match"},
				"maxItems": 2,
			},
			want: "[\n  \"match\",\n  0\n]",
		},
		{
			name: "unique items meet minimum",
			schema: map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "integer"},
				"minItems":    3,
				"uniqueItems": true,
			},
			want: "[\n  0,\n  1,\n  2\n]",
		},
		{
			name: "contains is satisfied",
			schema: map[string]any{
				"type":     "array",
				"contains": map[string]any{"const": "match"},
				"items":    map[string]any{"type": "string"},
				"maxItems": 1,
			},
			want: "[\n  \"match\"\n]",
		},
		{
			name: "zero contains maximum permits empty array",
			schema: map[string]any{
				"type":        "array",
				"contains":    map[string]any{},
				"items":       map[string]any{"type": "string"},
				"minContains": 0,
				"maxContains": 0,
			},
			want: "[]",
		},
		{
			name: "zero maximum does not materialize trailing items",
			schema: map[string]any{
				"type":     "array",
				"items":    false,
				"maxItems": 0,
			},
			want: "[]",
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

func TestMaterializationRejectsTupleMinimumWithoutTrailingItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema map[string]any
	}{
		{
			name: "modern items false",
			schema: map[string]any{
				"type": "array",
				"prefixItems": []any{
					map[string]any{"type": "string"},
				},
				"items":    false,
				"minItems": 2,
			},
		},
		{
			name: "draft 7 additional items false",
			schema: map[string]any{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type":    "array",
				"items": []any{
					map[string]any{"type": "string"},
				},
				"additionalItems": false,
				"minItems":        2,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := GenerateExampleJSON(minimalSchemaBytes(t, test.schema), ExampleModeAll)
			if !errors.Is(err, ErrMaterializationUnsatisfiable) {
				t.Fatalf("GenerateExampleJSON error = %v, want unsatisfiable", err)
			}
		})
	}
}

func TestMaterializationRejectsUnevaluatedItemsViolation(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "array",
		"prefixItems": []any{
			map[string]any{"type": "string"},
		},
		"unevaluatedItems": false,
		"minItems":         2,
	})

	_, err := GenerateExampleJSON(schema, ExampleModeAll)
	if !errors.Is(err, ErrMaterializationUnsatisfiable) {
		t.Fatalf("GenerateExampleJSON error = %v, want unsatisfiable", err)
	}
}
