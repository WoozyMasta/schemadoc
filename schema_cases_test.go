// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestSchemaCorpusManifest(t *testing.T) {
	t.Parallel()

	cases := loadSchemaCorpus(t)
	if len(cases) < 20 {
		t.Fatalf("schema corpus has %d cases, want at least 20", len(cases))
	}

	groups := make(map[string]int, len(cases))
	for _, fixture := range cases {
		if fixture.Status != "characterization" {
			t.Fatalf("case %q has unexpected status %q", fixture.Name, fixture.Status)
		}

		groups[fixture.Group]++
	}

	for _, group := range []string{"positive", "negative", "integration"} {
		if groups[group] == 0 {
			t.Fatalf("schema corpus has no %s cases", group)
		}
	}
}

func TestSchemaCorpusDocumentsAreJSON(t *testing.T) {
	t.Parallel()

	for _, fixture := range loadSchemaCorpus(t) {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			var document any
			if err := json.Unmarshal(fixture.readSchemaCorpusSchema(t), &document); err != nil {
				t.Fatalf("decode schema: %v", err)
			}

			switch document.(type) {
			case bool, map[string]any:
			default:
				t.Fatalf("schema root has unsupported type %T", document)
			}
		})
	}
}

func TestValidateDecodedInstance(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"required": []any{
			"name",
		},
		"properties": map[string]any{
			"name": map[string]any{
				"type": "string",
			},
		},
	})

	if err := validateDecodedInstance(schema, map[string]any{"name": "demo"}); err != nil {
		t.Fatalf("validate decoded instance: %v", err)
	}

	if err := validateDecodedInstance(schema, map[string]any{"name": 42}); err == nil {
		t.Fatal("validate decoded instance accepted an invalid value")
	}
}

func TestSchemaCorpusGeneratedExamplesValidate(t *testing.T) {
	t.Parallel()

	for _, fixture := range loadSchemaCorpus(t) {
		if fixture.Group != "positive" && fixture.Group != "integration" {
			continue
		}

		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			schema := fixture.readSchemaCorpusSchema(t)
			jsonExample, err := GenerateExampleJSON(schema, ExampleModeAll)
			if err != nil {
				t.Fatalf("generate json example: %v", err)
			}
			if err := validateGeneratedJSON(schema, jsonExample); err != nil {
				t.Fatalf("validate generated json example: %v", err)
			}

			yamlExample, err := GenerateExampleYAML(schema, ExampleModeAll)
			if err != nil {
				t.Fatalf("generate yaml example: %v", err)
			}
			if err := validateGeneratedYAML(schema, yamlExample); err != nil {
				t.Fatalf("validate generated yaml example: %v", err)
			}
		})
	}
}

func TestIntegrationCorpusOutputsAreDeterministic(t *testing.T) {
	t.Parallel()

	for _, fixture := range loadSchemaCorpus(t) {
		if fixture.Group != "integration" {
			continue
		}

		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			schema := fixture.readSchemaCorpusSchema(t)
			jsonFirst := generateCorpusExample(t, schema, ExampleFormatJSON)
			jsonSecond := generateCorpusExample(t, schema, ExampleFormatJSON)
			if !bytes.Equal(jsonFirst, jsonSecond) {
				t.Fatal("JSON examples are not deterministic")
			}

			yamlFirst := generateCorpusExample(t, schema, ExampleFormatYAML)
			yamlSecond := generateCorpusExample(t, schema, ExampleFormatYAML)
			if !bytes.Equal(yamlFirst, yamlSecond) {
				t.Fatal("YAML examples are not deterministic")
			}

			jsonValue := decodeCorpusInstance(t, jsonFirst, false)
			yamlValue := decodeCorpusInstance(t, yamlFirst, true)
			if !reflect.DeepEqual(jsonValue, yamlValue) {
				t.Fatalf("JSON and YAML examples differ: JSON=%#v YAML=%#v", jsonValue, yamlValue)
			}

			for _, templateName := range []string{"list", "table", "html"} {
				options := Options{TemplateName: templateName}
				first, err := Render(schema, options)
				if err != nil {
					t.Fatalf("Render(%s): %v", templateName, err)
				}
				second, err := Render(schema, options)
				if err != nil {
					t.Fatalf("Render(%s) second run: %v", templateName, err)
				}
				if first != second {
					t.Fatalf("%s document is not deterministic", templateName)
				}
			}
		})
	}
}

func generateCorpusExample(t *testing.T, schema []byte, format ExampleFormat) []byte {
	t.Helper()

	example, err := GenerateExample(schema, ExampleModeAll, format)
	if err != nil {
		t.Fatalf("GenerateExample(%s): %v", format, err)
	}

	return example
}

func decodeCorpusInstance(t *testing.T, content []byte, isYAML bool) any {
	t.Helper()

	var value any
	var err error
	if isYAML {
		err = yaml.Unmarshal(content, &value)
	} else {
		err = json.Unmarshal(content, &value)
	}
	if err != nil {
		t.Fatalf("decode %s instance: %v", map[bool]string{true: "YAML", false: "JSON"}[isYAML], err)
	}

	canonical, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("canonicalize instance: %v", err)
	}
	if err := json.Unmarshal(canonical, &value); err != nil {
		t.Fatalf("decode canonical instance: %v", err)
	}

	return value
}

func TestSchemaCorpusNegativeMaterialization(t *testing.T) {
	t.Parallel()

	for _, fixture := range loadSchemaCorpus(t) {
		if fixture.Group != "negative" {
			continue
		}

		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			_, err := GenerateExampleJSON(fixture.readSchemaCorpusSchema(t), ExampleModeAll)
			if err == nil {
				t.Fatal("GenerateExampleJSON unexpectedly succeeded")
			}

			var materialization *MaterializationError
			if !errors.As(err, &materialization) {
				t.Fatalf("error = %T %v, want MaterializationError", err, err)
			}
			if fixture.Error == nil {
				t.Fatalf("negative case has no error expectation")
			}
			if string(materialization.Category) != fixture.Error.Category {
				t.Fatalf("category = %q, want %q", materialization.Category, fixture.Error.Category)
			}
			if fixture.Error.Code != "" && string(materialization.Code) != fixture.Error.Code {
				t.Fatalf("code = %q, want %q", materialization.Code, fixture.Error.Code)
			}
			if fixture.Error.Path != "" && materialization.Path != fixture.Error.Path {
				t.Fatalf("path = %q, want %q", materialization.Path, fixture.Error.Path)
			}
		})
	}
}
