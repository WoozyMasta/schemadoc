// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestDetectDraftSupported(t *testing.T) {
	t.Parallel()

	cases := []string{
		"https://json-schema.org/draft/2020-12/schema",
		"https://json-schema.org/draft/2020-12/schema#",
		"2019-09",
		"http://json-schema.org/draft-07/schema",
		"https://json-schema.org/draft-06/schema/",
		"http://json-schema.org/draft-05/schema",
	}

	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			got := DetectDraft(input)
			if !got.Supported {
				t.Fatalf("draft %q should be supported: %+v", input, got)
			}
		})
	}
}

func TestDetectDraftUnsupported(t *testing.T) {
	t.Parallel()

	got := DetectDraft("https://json-schema.org/draft/2023-12/schema")
	if got.Supported {
		t.Fatalf("unexpected supported draft: %+v", got)
	}
}

func TestBuiltinTemplates(t *testing.T) {
	t.Parallel()

	names := BuiltinTemplateNames()
	if strings.Join(names, ",") != "html,list,table" {
		t.Fatalf("unexpected template names: %v", names)
	}

	if _, err := BuiltinTemplate("missing"); err == nil {
		t.Fatalf("expected error for unknown template")
	}
}

func TestRenderGeneratedFixtureSmoke(t *testing.T) {
	t.Parallel()

	rendered, err := RenderFile(
		filepath.Join("testdata", "generated", "app.schema.json"),
		Options{},
	)
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	if strings.TrimSpace(rendered) == "" {
		t.Fatalf("empty rendered output")
	}
}

func TestRenderListPathsAreContiguous(t *testing.T) {
	t.Parallel()

	output, err := RenderFile(
		filepath.Join("testdata", "generated", "app.schema.json"),
		Options{TemplateName: "list", ListMarker: "-"},
	)
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	want := "- [`bucket_groups`](#serviceconfigbucket_groups).`[]`.`[]`.`region`\n" +
		"- [`buckets_by_priority`](#serviceconfigbuckets_by_priority)`[\"^[0-9]+$\"]`.`region`\n" +
		"- [`named_buckets`](#serviceconfignamed_buckets).`[]`.`region`"
	if !strings.Contains(output, want) {
		t.Fatalf("Paths list contains unexpected spacing or presentation")
	}
}

func TestRenderXOrderControlsTOCAndPropertyHeadings(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Root",
		"$defs": map[string]any{
			"Root": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"first": map[string]any{
						"$ref":    "#/$defs/First",
						"x-order": 2,
					},
					"second": map[string]any{
						"$ref":    "#/$defs/Second",
						"x-order": 1,
					},
				},
			},
			"First":  map[string]any{"type": "object"},
			"Second": map[string]any{"type": "object"},
		},
	})

	for _, templateName := range []string{"list", "html"} {
		t.Run(templateName, func(t *testing.T) {
			t.Parallel()

			got, err := Render(schema, Options{TemplateName: templateName})
			if err != nil {
				t.Fatalf("Render(%s): %v", templateName, err)
			}

			assertBefore(t, got, "Second", "First")
			assertBefore(t, got, "Root.Second", "Root.First")
		})
	}
}

func TestRenderRootContentWithDefinitions(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"title": "Root configuration",
		"type":  "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Root content remains visible.",
			},
		},
		"$defs": map[string]any{
			"Unused": map[string]any{
				"type": "object",
			},
		},
	})

	output, err := Render(schema, Options{TemplateName: "list"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, marker := range []string{
		"## Root",
		"### Root.name",
		"Root content remains visible.",
		"## Unused",
	} {
		assertContains(t, output, marker)
	}
}

func TestRenderReferenceGraphIncludesSchemaBearingKeywords(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"allOf": []any{
			map[string]any{"$ref": "#/$defs/Composition"},
		},
		"items": map[string]any{
			"$ref": "#/$defs/ArrayItem",
		},
		"propertyNames": map[string]any{
			"$ref": "#/$defs/PropertyName",
		},
		"dependentSchemas": map[string]any{
			"enabled": map[string]any{"$ref": "#/$defs/Dependent"},
		},
		"dependencies": map[string]any{
			"legacy": map[string]any{"$ref": "#/$defs/Dependency"},
		},
		"$defs": map[string]any{
			"Composition":  map[string]any{"type": "object"},
			"ArrayItem":    map[string]any{"type": "object"},
			"PropertyName": map[string]any{"type": "string"},
			"Dependent":    map[string]any{"type": "object"},
			"Dependency":   map[string]any{"type": "object"},
		},
	})

	output, err := Render(schema, Options{TemplateName: "list"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, name := range []string{
		"Composition",
		"ArrayItem",
		"PropertyName",
		"Dependent",
		"Dependency",
	} {
		assertContains(t, output, "["+name+"](#"+strings.ToLower(name)+")")
	}
}

func TestRenderHidesInternalKeywordsByDefault(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{
				"type":    "string",
				"x-order": 1,
				"x-owner": "platform",
			},
		},
	})

	defaultOutput, err := Render(schema, Options{})
	if err != nil {
		t.Fatalf("Render default: %v", err)
	}
	assertNotContains(t, defaultOutput, "x-order=1")
	assertContains(t, defaultOutput, "x-owner=platform")

	internalOutput, err := Render(schema, Options{ShowInternalKeywords: true})
	if err != nil {
		t.Fatalf("Render with internal keywords: %v", err)
	}
	assertContains(t, internalOutput, "x-order=1")
}

func TestRenderSummarizesStructuredAnnotations(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{
		"$vocabulary": map[string]any{
			"https://json-schema.org/draft/2020-12/vocab/core": true,
		},
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{
				"type": "object",
				"default": map[string]any{
					"enabled": true,
				},
				"enum": []any{
					map[string]any{"enabled": false},
					"safe",
				},
				"examples": []any{
					map[string]any{"enabled": true},
					"demo",
				},
				"x-note": map[string]any{
					"details": []any{"structured"},
				},
			},
		},
	})

	output, err := Render(schema, Options{TemplateName: "list"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, output, "* Default: `structured object`")
	assertContains(t, output, "* Enum: `safe`, 1 structured value")
	assertContains(t, output, "* Examples: `demo`, 1 structured example")
	assertContains(t, output, "* Other keywords: x-note=structured object")
	assertNotContains(t, output, `{"enabled":`)
	assertNotContains(t, output, "$vocabulary=")
}

func assertBefore(t *testing.T, text, first, second string) {
	t.Helper()
	firstIndex := strings.Index(text, first)
	secondIndex := strings.Index(text, second)
	if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
		t.Fatalf("expected %q before %q", first, second)
	}
}

func TestRenderGeneratedFixturesByDraft(t *testing.T) {
	t.Parallel()

	fixtures, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.json"))
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}

	if len(fixtures) == 0 {
		t.Fatal("no fixtures found")
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			t.Parallel()

			output, err := RenderFile(fixture, Options{})
			if err != nil {
				t.Fatalf("RenderFile(%s): %v", fixture, err)
			}

			if strings.TrimSpace(output) == "" {
				t.Fatalf("empty output for %s", fixture)
			}
		})
	}
}

func TestRenderGoldenList(t *testing.T) {
	testRenderGoldenTemplate(t, "list", filepath.Join("testdata", "generated", "app.doc.list.md"))
}

func TestRenderGoldenTable(t *testing.T) {
	testRenderGoldenTemplate(t, "table", filepath.Join("testdata", "generated", "app.doc.table.md"))
}

func TestRenderGoldenHTML(t *testing.T) {
	testRenderGoldenTemplate(t, "html", filepath.Join("testdata", "generated", "app.doc.html"))
}

func testRenderGoldenTemplate(t *testing.T, templateName, goldenPath string) {
	t.Helper()

	schemaPath := filepath.Join("testdata", "generated", "app.schema.json")
	const sourcePath = "testdata/generated/app.schema.json"
	renderOptions := Options{
		SourcePath:   sourcePath,
		TemplateName: templateName,
	}

	switch templateName {
	case "list":
		renderOptions.Title = "Testdata Reference (List + YAML Example)"
		renderOptions.Description = "Golden fixture for list template with embedded required YAML example."
		renderOptions.ExampleMode = ExampleModeRequired
		renderOptions.ExampleFormat = ExampleFormatYAML
		renderOptions.ListMarker = "-"
		renderOptions.WrapWidth = 72
	case "table":
		renderOptions.Title = "Testdata Reference (Table + JSON Example)"
		renderOptions.Description = "Golden fixture for table template with embedded full JSON example."
		renderOptions.ExampleMode = ExampleModeAll
		renderOptions.ExampleFormat = ExampleFormatJSON
		renderOptions.WrapWidth = 88
	case "html":
		renderOptions.Title = "Testdata Reference (HTML + YAML Example)"
		renderOptions.Description = "Golden fixture for html template with embedded YAML example and rich comments."
		renderOptions.ExampleMode = ExampleModeAll
		renderOptions.ExampleFormat = ExampleFormatYAML
		renderOptions.WrapWidth = 90
	default:
		t.Fatalf("unsupported golden template %q", templateName)
	}

	got, err := RenderFile(schemaPath, Options{
		Title:          renderOptions.Title,
		Description:    renderOptions.Description,
		SourcePath:     renderOptions.SourcePath,
		TemplateName:   renderOptions.TemplateName,
		ListMarker:     renderOptions.ListMarker,
		WrapWidth:      renderOptions.WrapWidth,
		ExampleMode:    renderOptions.ExampleMode,
		ExampleFormat:  renderOptions.ExampleFormat,
		FooterToolName: "schemadoc",
		FooterToolURL:  "https://github.com/woozymasta/schemadoc",
		FooterVersion:  "dev",
		FooterCommit:   "unknown",
	})
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}

	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	want := string(wantBytes)
	if got != want {
		t.Fatalf("golden mismatch for %s; run `go test . -run TestRenderGolden -update`", templateName)
	}
}
