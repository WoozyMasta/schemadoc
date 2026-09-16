// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"encoding/json"
	"testing"
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
