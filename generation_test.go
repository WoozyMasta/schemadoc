// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestGenerationChildValueBacktracking(t *testing.T) {
	tests := []struct {
		name, schema, property string
		want                   any
	}{
		{
			"forced_else",
			`{
	"type": "object",
	"properties": {
		"kind": {
			"enum": ["a", "b"]
		}
	},
	"required": ["kind"],
	"if": {
		"properties": {
			"kind": {
				"const": "a"
			}
		},
		"required": ["kind"]
	},
	"then": false,
	"else": true,
	"additionalProperties": false
}`,
			"kind",
			"b",
		},
		{
			"nested_oneof",
			`{
	"type": "object",
	"properties": {
		"x": {
			"type": "integer"
		}
	},
	"required": ["x"],
	"oneOf": [{
			"properties": {
				"x": {
					"minimum": 0
				}
			}
		},
		{
			"properties": {
				"x": {
					"const": 0
				}
			}
		}
	]
}`,
			"x",
			float64(1),
		},
		{
			"property_subset",
			`{
	"type": "object",
	"properties": {
		"kind": {
			"enum": ["a", "b"]
		},
		"flag": {
			"type": "boolean"
		}
	},
	"required": ["kind"],
	"minProperties": 2,
	"not": {
		"properties": {
			"kind": {
				"const": "a"
			}
		}
	}
}`,
			"kind",
			"b",
		},
		{
			"independent_budgets",
			`{
	"type": "object",
	"properties": {
		"kind": {
			"enum": ["a", "b", "c"]
		},
		"f1": {
			"type": "boolean"
		},
		"f2": {
			"type": "boolean"
		},
		"f3": {
			"type": "boolean"
		}
	},
	"required": ["kind"],
	"minProperties": 3,
	"allOf": [{
			"not": {
				"properties": {
					"kind": {
						"enum": ["a", "b"]
					}
				}
			}
		},
		{
			"not": {
				"required": ["f1"]
			}
		}
	]
}`,
			"kind",
			"c",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var object map[string]any
			if err := json.Unmarshal([]byte(test.schema), &object); err != nil {
				t.Fatal(err)
			}
			schema := minimalSchemaBytes(t, object)
			for _, mode := range []ExampleMode{ExampleModeRequired, ExampleModeAll} {
				first, err := GenerateExampleJSON(schema, mode)
				if err != nil {
					t.Fatalf("%s: %v", mode, err)
				}
				if err := validateGeneratedJSON(schema, first); err != nil {
					t.Fatal(err)
				}
				second, err := GenerateExampleJSON(schema, mode)
				if err != nil || !bytes.Equal(first, second) {
					t.Fatalf("nondeterministic: %s / %s: %v", first, second, err)
				}
				var result map[string]any
				if err := json.Unmarshal(first, &result); err != nil {
					t.Fatal(err)
				}
				if result[test.property] != test.want {
					t.Fatalf("%s: %s", mode, first)
				}
				if test.name == "property_subset" {
					if _, ok := result["flag"]; !ok {
						t.Fatalf("%s: missing flag: %s", mode, first)
					}
				}
			}
		})
	}
}

func TestChildValueRetriesDoNotExhaustPropertySets(t *testing.T) {
	t.Parallel()
	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object", "required": []any{"kind"},
		"properties": map[string]any{
			"kind": map[string]any{"enum": []any{"a", "b"}},
			"flag": map[string]any{"const": true},
		},
		"not": map[string]any{"anyOf": []any{
			map[string]any{"properties": map[string]any{"kind": map[string]any{"const": "a"}}},
			map[string]any{"not": map[string]any{"required": []any{"flag"}}},
		}},
	})
	doc, err := parseDocument(schema)
	if err != nil {
		t.Fatal(err)
	}
	m := newExampleMaterializer(doc, ExampleModeRequired)
	m.budget.childValues = 1
	m.budget.propertySets = 1
	value, err := m.materialize(doc.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDecodedInstance(schema, value); err != nil {
		t.Fatal(err)
	}
	object := value.(map[string]any)
	if object["kind"] != "b" || len(object) != 2 {
		t.Fatalf("unexpected value: %v", value)
	}
}

func TestGenerationDeepDeterministicSchemas(t *testing.T) {
	t.Parallel()
	var nested any = map[string]any{"type": "string"}
	for range 50 {
		nested = map[string]any{"type": "object", "required": []any{"x"},
			"properties": map[string]any{"x": nested}}
	}
	allOf := make([]any, 70)
	for i := range allOf {
		allOf[i] = map[string]any{"anyOf": []any{map[string]any{}}}
	}
	for _, root := range []any{nested, map[string]any{"type": "string", "allOf": allOf}} {
		bytes, err := json.Marshal(root)
		if err != nil {
			t.Fatal(err)
		}
		generated, err := GenerateExampleJSON(bytes, ExampleModeRequired)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateGeneratedJSON(bytes, generated); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGenerationValidatorSetupError(t *testing.T) {
	t.Parallel()
	doc, err := parseDocument([]byte(`{"type":"string","pattern":"["}`))
	if err != nil {
		t.Fatal(err)
	}
	m := newExampleMaterializer(doc, ExampleModeRequired)
	if m.validatorError == nil {
		t.Fatal("expected validator compilation error")
	}
	_, err = m.materialize(doc.Root)
	if !errors.Is(err, m.validatorError) {
		t.Fatalf("validator error lost: %v", err)
	}
}

func TestGenerationPlanRegressions(t *testing.T) {
	t.Parallel()
	names := []string{
		"composition/common-object-one-of",
		"composition/optional-discriminator-one-of",
		"composition/common-scalar-one-of",
		"composition/common-array-one-of",
		"composition/multiple-one-of-groups",
		"composition/mixed-alternatives",
		"composition/nested-object-alternatives",
		"composition/ref-sibling-alternatives",
		"objects/transitive-dependent-required",
		"objects/dependent-schema-constraint",
		"objects/transitive-dependent-schemas",
		"objects/draft7-dependencies",
		"objects/conditional-constraints",
		"objects/required-unconstrained",
		"objects/required-via-additional-properties",
		"objects/required-via-pattern-properties",
		"objects/one-of-unevaluated-properties",
		"arrays/one-of-unevaluated-items",
		"objects/minimal-required",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			schema, err := os.ReadFile(filepath.Join("testdata", "cases", "positive", name+".json"))
			if err != nil {
				t.Fatal(err)
			}

			for _, mode := range []ExampleMode{ExampleModeRequired, ExampleModeAll} {
				t.Run(string(mode), func(t *testing.T) {
					first, err := GenerateExampleJSON(schema, mode)
					if err != nil {
						t.Fatal(err)
					}
					if err := validateGeneratedJSON(schema, first); err != nil {
						t.Fatal(err)
					}

					second, err := GenerateExampleJSON(schema, mode)
					if err != nil || !bytes.Equal(first, second) {
						t.Fatalf("nondeterministic output: %s / %s: %v", first, second, err)
					}

					yaml, err := GenerateExampleYAML(schema, mode)
					if err != nil {
						t.Fatal(err)
					}
					if err := validateGeneratedYAML(schema, yaml); err != nil {
						t.Fatal(err)
					}

					if name == "composition/common-object-one-of" {
						var object map[string]any
						if err := json.Unmarshal(first, &object); err != nil {
							t.Fatal(err)
						}

						_, hasName := object["name"]
						wantLength := 2
						if mode == ExampleModeAll {
							wantLength = 3
						}
						if hasName != (mode == ExampleModeAll) || len(object) != wantLength {
							t.Fatalf("unexpected property selection: %s", first)
						}
					}

					if name == "objects/minimal-required" && mode == ExampleModeRequired {
						var object map[string]any
						if err := json.Unmarshal(first, &object); err != nil {
							t.Fatal(err)
						}
						if len(object) != 1 {
							t.Fatalf("unnecessary fields: %s", first)
						}
					}
				})
			}
		})
	}
}

func TestGenerationRootRejectionContinuesSearch(t *testing.T) {
	t.Parallel()
	schemas := []string{
		`{
		  "type": "object",
		  "properties": {
		    "a": {
		      "type": "string"
		    },
		    "b": {
		      "type": "string"
		    }
		  },
		  "allOf": [
		    {
		      "properties": {
		        "a": {}
		      },
		      "unevaluatedProperties": false
		    },
		    {
		      "properties": {
		        "b": {}
		      }
		    }
		  ]
		}`,
		`{
		  "type": "object",
		  "required": [
		    "a",
		    "b"
		  ],
		  "oneOf": [
		    {
		      "properties": {
		        "a": {}
		      },
		      "unevaluatedProperties": false
		    },
		    {
		      "properties": {
		        "a": {},
		        "b": {}
		      }
		    }
		  ]
		}`,
	}
	for _, raw := range schemas {
		var object map[string]any
		if err := json.Unmarshal([]byte(raw), &object); err != nil {
			t.Fatal(err)
		}
		schema := minimalSchemaBytes(t, object)
		generated, err := GenerateExampleJSON(schema, ExampleModeAll)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateGeneratedJSON(schema, generated); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGenerationSearchBudget(t *testing.T) {
	t.Parallel()
	schema := minimalSchemaBytes(t, map[string]any{
		"type":       "object",
		"properties": map[string]any{"x": map[string]any{"type": "string"}},
		"not":        map[string]any{},
	})
	_, err := GenerateExampleJSON(schema, ExampleModeAll)
	if !errors.Is(err, ErrUnsupportedMaterialization) || errors.Is(err, ErrMaterializationUnsatisfiable) {
		t.Fatalf("search failure misclassified: %v", err)
	}
}

func TestRequiredPropertyWithUnevaluatedProperties(t *testing.T) {
	t.Parallel()
	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object", "required": []any{"x"},
		"unevaluatedProperties": map[string]any{"type": "string"},
	})
	for _, mode := range []ExampleMode{ExampleModeRequired, ExampleModeAll} {
		generated, err := GenerateExampleJSON(schema, mode)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateGeneratedJSON(schema, generated); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGenerationPlansOrderAndBound(t *testing.T) {
	t.Parallel()
	doc, err := parseDocument([]byte(`{
		  "type": "object",
		  "required": [
		    "base"
		  ],
		  "allOf": [
		    {
		      "oneOf": [
		        {
		          "required": [
		            "a"
		          ]
		        },
		        {
		          "required": [
		            "b"
		          ]
		        }
		      ]
		    },
		    {
		      "anyOf": [
		        {
		          "required": [
		            "x"
		          ]
		        },
		        {
		          "required": [
		            "y"
		          ]
		        }
		      ]
		    }
		  ]
		}`))
	if err != nil {
		t.Fatal(err)
	}

	m := newExampleMaterializer(doc, ExampleModeAll)
	whole, err := m.semantic.effective(doc.Root)
	if err != nil {
		t.Fatal(err)
	}

	var plans [][]string
	_, err = m.generationPlans(whole, schemaPath{}, func(plan effectiveSchema) (bool, error) {
		plans = append(plans, plan.required())
		return false, nil
	})
	if !errors.Is(err, ErrUnsupportedMaterialization) {
		t.Fatalf("search rejection: %v", err)
	}

	want := [][]string{{"base", "a", "x"}, {"base", "a", "y"}, {"base", "b", "x"}, {"base", "b", "y"}}
	if len(plans) != len(want) {
		t.Fatalf("plans: %v", plans)
	}
	for i := range want {
		if !slices.Equal(plans[i], want[i]) {
			t.Fatalf("plans: %v", plans)
		}
	}

	m.budget.plans = 0
	planCount := 0
	_, err = m.generationPlans(whole, schemaPath{}, func(effectiveSchema) (bool, error) {
		planCount++
		return false, nil
	})
	if !errors.Is(err, ErrUnsupportedMaterialization) || planCount != 1 {
		t.Fatalf("budget exhaustion: %d plans, %v", planCount, err)
	}

	m.budget.propertySets = 3
	visits := 0
	_, err = m.visitSubsets(100, 50, func([]int) (bool, error) {
		visits++
		m.budget.propertySets--
		return false, nil
	})

	if err != nil || visits != 3 {
		t.Fatalf("unbounded subsets: %d, %v", visits, err)
	}
}

func TestGenerationActivatedAlternatives(t *testing.T) {
	t.Parallel()
	schemas := []string{
		`{
		  "type": "object",
		  "required": [
		    "enabled"
		  ],
		  "properties": {
		    "enabled": {
		      "const": true
		    }
		  },
		  "dependentSchemas": {
		    "enabled": {
		      "oneOf": [
		        false,
		        {
		          "properties": {
		            "kind": {
		              "const": "a"
		            }
		          }
		        },
		        {
		          "properties": {
		            "kind": {
		              "const": "b"
		            }
		          }
		        }
		      ]
		    }
		  }
		}`,
		`{
		  "type": "object",
		  "required": [
		    "enabled"
		  ],
		  "properties": {
		    "enabled": {
		      "const": true
		    }
		  },
		  "dependentSchemas": {
		    "enabled": {
		      "properties": {
		        "extra": {
		          "const": "optional"
		        }
		      }
		    }
		  }
		}`,
		`{
		  "type": "object",
		  "properties": {
		    "kind": {
		      "type": "string"
		    },
		    "extra": {
		      "type": "integer"
		    }
		  },
		  "oneOf": [
		    {
		      "anyOf": [
		        {
		          "properties": {
		            "kind": {
		              "const": "a"
		            }
		          },
		          "required": [
		            "kind"
		          ]
		        },
		        {
		          "properties": {
		            "kind": {
		              "const": "b"
		            }
		          },
		          "required": [
		            "kind"
		          ]
		        }
		      ]
		    },
		    false
		  ]
		}`,
		`{"type":"string","if":{"const":"trigger"},"then":{"pattern":"^(?:[A-Z]{3}-){2}[0-9]{4}$"},"else":false}`,
	}
	for i, raw := range schemas {
		var object map[string]any
		if err := json.Unmarshal([]byte(raw), &object); err != nil {
			t.Fatal(err)
		}

		schema := minimalSchemaBytes(t, object)
		for _, mode := range []ExampleMode{ExampleModeRequired, ExampleModeAll} {
			generated, err := GenerateExampleJSON(schema, mode)
			if i == 3 {
				if !errors.Is(err, ErrUnsupportedMaterialization) || errors.Is(err, ErrMaterializationUnsatisfiable) {
					t.Fatalf("conditional search misclassified: %v", err)
				}
				continue
			}

			if err != nil {
				t.Fatalf("case %d, %s: %v", i, mode, err)
			}
			if err := validateGeneratedJSON(schema, generated); err != nil {
				t.Fatal(err)
			}

			if i == 1 {
				var result map[string]any
				if err := json.Unmarshal(generated, &result); err != nil {
					t.Fatal(err)
				}
				_, extra := result["extra"]
				if extra != (mode == ExampleModeAll) {
					t.Fatalf("dependent optional property preference: %s", generated)
				}
			}
		}
	}
}

func BenchmarkGenerateExampleSearch(b *testing.B) {
	for _, name := range []string{
		"composition/common-object-one-of",
		"composition/multiple-one-of-groups",
		"objects/transitive-dependent-required",
	} {
		for _, mode := range []ExampleMode{ExampleModeRequired, ExampleModeAll} {
			b.Run(name+"/"+string(mode), func(b *testing.B) {
				schema, err := os.ReadFile(filepath.Join("testdata", "cases", "positive", name+".json"))
				if err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				for b.Loop() {
					if _, err := GenerateExampleJSON(schema, mode); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func TestGenerationConjunctiveTypeSemantics(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"type":["string","integer"],"oneOf":[{"type":"integer","minimum":10},false]}`,
		`{"type":"number","allOf":[{"type":"integer","minimum":2}]}`,
		`{"type":"integer","properties":{"ignored":false},"required":["ignored"],"oneOf":[{"maximum":0},{"minimum":10}]}`,
		`{
		  "$schema": "http://json-schema.org/draft-07/schema#",
		  "type": "object",
		  "required": [
		    "a"
		  ],
		  "dependentSchemas": {
		    "a": false
		  },
		  "dependentRequired": {
		    "a": [
		      "ignored"
		    ]
		  }
		}`,
	} {
		var object map[string]any
		if err := json.Unmarshal([]byte(raw), &object); err != nil {
			t.Fatal(err)
		}

		if _, exists := object["$schema"]; !exists {
			object["$schema"] = "https://json-schema.org/draft/2020-12/schema"
		}
		schema, err := json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}

		generated, err := GenerateExampleJSON(schema, ExampleModeRequired)
		if err != nil {
			t.Fatalf("%s: %v", schema, err)
		}
		if err := validateGeneratedJSON(schema, generated); err != nil {
			t.Fatal(err)
		}
	}
}
