// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

const schemaCorpusManifestPath = "testdata/cases/manifest.json"

// schemaCorpusCase describes one schema fixture
// and the checks that can be activated without changing the corpus loader.
type schemaCorpusCase struct {
	Name         string                         `json:"name"`
	Group        string                         `json:"group"`
	Schema       string                         `json:"schema"`
	Dialect      string                         `json:"dialect,omitempty"`
	Status       string                         `json:"status"`
	Description  string                         `json:"description,omitempty"`
	JSONExpected string                         `json:"json_expected,omitempty"`
	YAMLExpected string                         `json:"yaml_expected,omitempty"`
	Error        *schemaCorpusErrorExpectation  `json:"error,omitempty"`
	Render       *schemaCorpusRenderExpectation `json:"render,omitempty"`
}

// schemaCorpusErrorExpectation stores stable error assertions for negative cases.
// Human-readable error text is intentionally not part of the corpus.
type schemaCorpusErrorExpectation struct {
	Category string `json:"category"`
	Code     string `json:"code,omitempty"`
	Path     string `json:"path,omitempty"`
}

// schemaCorpusRenderExpectation stores template assertions
// for rendering cases without coupling the loader to a particular output format.
type schemaCorpusRenderExpectation struct {
	Template string `json:"template"`
	Golden   string `json:"golden,omitempty"`
}

// loadSchemaCorpus loads and validates the fixture manifest.
func loadSchemaCorpus(t *testing.T) []schemaCorpusCase {
	t.Helper()

	data, err := os.ReadFile(schemaCorpusManifestPath)
	if err != nil {
		t.Fatalf("read schema corpus manifest: %v", err)
	}

	var cases []schemaCorpusCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("decode schema corpus manifest: %v", err)
	}

	if len(cases) == 0 {
		t.Fatal("schema corpus manifest is empty")
	}

	seenNames := make(map[string]struct{}, len(cases))
	seenSchemas := make(map[string]struct{}, len(cases))
	for index := range cases {
		fixture := &cases[index]
		if fixture.Name == "" || fixture.Schema == "" {
			t.Fatalf("corpus case %d must have name and schema", index)
		}
		if _, exists := seenNames[fixture.Name]; exists {
			t.Fatalf("duplicate corpus case name %q", fixture.Name)
		}
		seenNames[fixture.Name] = struct{}{}
		if _, exists := seenSchemas[fixture.Schema]; exists {
			t.Fatalf("duplicate corpus schema %q", fixture.Schema)
		}
		seenSchemas[fixture.Schema] = struct{}{}

		schemaPath := filepath.Join("testdata", "cases", filepath.FromSlash(fixture.Schema))
		if _, err := os.Stat(schemaPath); err != nil {
			t.Fatalf("corpus case %q schema %q: %v", fixture.Name, schemaPath, err)
		}
	}

	return cases
}

// readSchemaCorpusSchema returns the raw schema bytes for one fixture.
func (fixture schemaCorpusCase) readSchemaCorpusSchema(t *testing.T) []byte {
	t.Helper()

	path := filepath.Join("testdata", "cases", filepath.FromSlash(fixture.Schema))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read corpus schema %q: %v", fixture.Name, err)
	}

	return data
}

// validateDecodedInstance independently validates a decoded instance against
// an in-memory schema without enabling a network-backed resource loader.
func validateDecodedInstance(schemaBytes []byte, instance any) error {
	var schema any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return fmt.Errorf("decode schema: %w", err)
	}

	const resourceURL = "urn:schemadoc:test-schema"
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(resourceURL, schema); err != nil {
		return fmt.Errorf("register schema resource: %w", err)
	}

	compiled, err := compiler.Compile(resourceURL)
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}

	if err := compiled.Validate(instance); err != nil {
		return fmt.Errorf("validate instance: %w", err)
	}

	return nil
}

// validateGeneratedJSON decodes generated JSON before independent validation.
func validateGeneratedJSON(schemaBytes, generated []byte) error {
	var instance any
	if err := json.Unmarshal(generated, &instance); err != nil {
		return fmt.Errorf("decode generated json: %w", err)
	}

	return validateDecodedInstance(schemaBytes, instance)
}

// validateGeneratedYAML decodes YAML and normalizes it before validation.
func validateGeneratedYAML(schemaBytes, generated []byte) error {
	var value any
	if err := yaml.Unmarshal(generated, &value); err != nil {
		return fmt.Errorf("decode generated yaml: %w", err)
	}

	normalized := normalizeYAMLMap(value)
	return validateDecodedInstance(schemaBytes, normalized)
}

func minimalSchemaBytes(t *testing.T, doc map[string]any) []byte {
	t.Helper()

	if _, ok := doc["$schema"]; !ok {
		doc["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	}

	if _, ok := doc["$id"]; !ok {
		doc["$id"] = "urn:test"
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal schema fixture: %v", err)
	}

	return data
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if !strings.Contains(haystack, needle) {
		t.Fatalf("missing substring %q in:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if strings.Contains(haystack, needle) {
		t.Fatalf("unexpected substring %q in:\n%s", needle, haystack)
	}
}
