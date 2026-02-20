// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
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

func TestDefinitionOrderRootFallbackConfig(t *testing.T) {
	t.Parallel()

	order := definitionOrder(map[string]schemaValue{
		"Zulu":   {},
		"Config": {},
		"Alpha":  {},
	}, "")

	got := strings.Join(order, ",")
	want := "Config,Alpha,Zulu"
	if got != want {
		t.Fatalf("definition order = %q, want %q", got, want)
	}
}

func TestPropertyOrderRequiredThenOptionalSorted(t *testing.T) {
	t.Parallel()

	order := propertyOrder([]string{"b", "a"}, map[string]schemaValue{
		"d": {},
		"c": {},
		"b": {},
		"a": {},
	})

	got := strings.Join(order, ",")
	want := "b,a,c,d"
	if got != want {
		t.Fatalf("property order = %q, want %q", got, want)
	}
}

func TestRenderSupportsDefinitionsKeyword(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/definitions/Config",
		"definitions": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "## Config")
	assertContains(t, rendered, "### Config.name")
}

func TestRenderSupportsRootWithoutDefinitions(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "## Root")
	assertContains(t, rendered, "### Root.name")
}

func TestRenderIncludesBooleanAndReferences(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled": true,
					"target": map[string]any{
						"$ref":          "#/$defs/Target",
						"$dynamicRef":   "#/$defs/Dyn",
						"$recursiveRef": "#/$defs/Rec",
					},
				},
			},
			"Target": map[string]any{
				"type": "string",
			},
		},
	}), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "Boolean schema: true")
	assertContains(t, rendered, "Reference: `#/$defs/Target`")
	assertContains(t, rendered, "Dynamic reference: `#/$defs/Dyn`")
	assertContains(t, rendered, "Recursive reference: `#/$defs/Rec`")
}

func TestRenderUsesObjectNameInReferencedPropertyHeading(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/SchemaModel",
		"$defs": map[string]any{
			"SchemaModel": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"draft_info": map[string]any{
						"$ref": "#/$defs/DraftInfo",
					},
				},
			},
			"DraftInfo": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"supported": map[string]any{"type": "boolean"},
				},
			},
		},
	}), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "### SchemaModel.DraftInfo")
	assertContains(t, rendered, "Key: `draft_info`")
	assertNotContains(t, rendered, "Path: `SchemaModel.draft_info`")
	assertNotContains(t, rendered, "Path: `draft_info`")
}

func TestRenderShowsResolvedPathsForReusedDefinitions(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"spec": map[string]any{
						"$ref": "#/$defs/BuildSpec",
					},
				},
			},
			"BuildSpec": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"projects": map[string]any{
						"type": "object",
						"additionalProperties": map[string]any{
							"$ref": "#/$defs/ProjectConfig",
						},
					},
					"settings": map[string]any{
						"$ref": "#/$defs/BuildSettings",
					},
				},
			},
			"ProjectConfig": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"settings": map[string]any{
						"$ref": "#/$defs/BuildSettings",
					},
				},
			},
			"BuildSettings": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sign": map[string]any{
						"$ref": "#/$defs/SignOptions",
					},
				},
			},
			"SignOptions": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled": map[string]any{
						"type": "boolean",
					},
				},
			},
		},
	}), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "### SignOptions.enabled")
	assertContains(t, rendered, "Paths:")
	assertContains(t, rendered, "* `spec.settings.sign.enabled`")
	assertContains(t, rendered, "* `spec.projects.[].settings.sign.enabled`")
}

func TestRenderIncludesKeywordCoverageSummaries(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"value": map[string]any{
						"type":                 "number",
						"minimum":              1,
						"maximum":              10,
						"exclusiveMinimum":     0,
						"exclusiveMaximum":     11,
						"multipleOf":           2,
						"minLength":            1,
						"maxLength":            64,
						"pattern":              "^[a-z]+$",
						"minItems":             1,
						"maxItems":             3,
						"uniqueItems":          true,
						"minContains":          1,
						"maxContains":          2,
						"minProperties":        1,
						"maxProperties":        4,
						"deprecated":           true,
						"readOnly":             true,
						"writeOnly":            false,
						"contentEncoding":      "base64",
						"contentMediaType":     "application/json",
						"contentSchema":        map[string]any{"type": "string"},
						"if":                   map[string]any{"type": "number"},
						"then":                 map[string]any{"minimum": 10},
						"else":                 map[string]any{"maximum": 0},
						"not":                  map[string]any{"const": 3},
						"oneOf":                []any{map[string]any{"type": "string"}},
						"anyOf":                []any{map[string]any{"type": "number"}},
						"allOf":                []any{map[string]any{"minimum": 1}},
						"prefixItems":          []any{map[string]any{"type": "integer"}},
						"additionalItems":      false,
						"contains":             map[string]any{"type": "integer"},
						"unevaluatedItems":     true,
						"propertyNames":        map[string]any{"pattern": "^[a-z]+$"},
						"additionalProperties": false,
						"unevaluatedProperties": map[string]any{
							"type": "string",
						},
						"dependentRequired": map[string]any{
							"a": []any{"b"},
						},
						"dependentSchemas": map[string]any{
							"a": map[string]any{"type": "string"},
						},
						"dependencies": map[string]any{
							"a": []any{"b"},
						},
						"x-unknown-keyword": "value",
					},
				},
			},
		},
	}), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "Composition: oneOf=1; anyOf=1; allOf=1")
	assertContains(t, rendered, "Conditional: if, then, else")
	assertContains(t, rendered, "Not: inline schema")
	assertContains(t, rendered, "Read only: yes")
	assertContains(t, rendered, "Write only: no")
	assertContains(t, rendered, "Deprecated: yes")
	assertContains(t, rendered, "Content encoding: `base64`")
	assertContains(t, rendered, "Other keywords: x-unknown-keyword=\"value\"")
}

func TestRenderPreservesMarkdownDescription(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"notes": map[string]any{
						"type":        "string",
						"description": "Paragraph before list.\n\n- first item\n- second item\n\n> quoted text",
					},
				},
			},
		},
	}), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "Paragraph before list.")
	assertContains(t, rendered, "* first item")
	assertContains(t, rendered, "> quoted text")
	assertNotContains(t, rendered, "&gt;")
}

func TestRenderNormalizesListIndentInDescription(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"template_name": map[string]any{
						"type":        "string",
						"description": "TemplateName selects one built-in template.\n\nSupported values:\n\n - `list`\n - `table`",
					},
				},
			},
		},
	}), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "Supported values:\n\n* `list`\n* `table`")
	assertNotContains(t, rendered, "\n - `list`")
}

func TestRenderInsertsBlankLineBeforeListWhenMissing(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"list_marker": map[string]any{
						"type":        "string",
						"description": "Supported values:\n - `-`\n - `*`",
					},
				},
			},
		},
	}), Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "Supported values:\n\n* `-`\n* `*`")
}

func TestRenderNormalizesListMarkerAndPreservesGodocMarkdown(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"notes": map[string]any{
						"type":        "string",
						"description": "# Usage\n\n - first\n  - second\n\n```bash\n echo from fence\n```\n\n    go test ./...",
					},
				},
			},
		},
	}), Options{ListMarker: "*"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "# Usage")
	assertContains(t, rendered, "* first")
	assertContains(t, rendered, "  * second")
	assertContains(t, rendered, "```bash")
	assertContains(t, rendered, " echo from fence")
	assertContains(t, rendered, "    go test ./...")
}

func TestRenderWrapWidth(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"notes": map[string]any{
						"type":        "string",
						"description": "This paragraph should be wrapped by words for predictable line length in markdown output.",
					},
				},
			},
		},
	}), Options{WrapWidth: 32})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "This paragraph should be wrapped")
	assertContains(t, rendered, "by words for predictable line")
	assertContains(t, rendered, "length in markdown output.")
}

func TestRenderNoMultipleBlankLinesAfterPropertyHeading(t *testing.T) {
	t.Parallel()

	rendered, err := RenderFile(filepath.Join("testdata", "schema.fixture.json"), Options{})
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	headingGapPattern := regexp.MustCompile(`(?m)^### .*\n\n\n+`)
	if headingGapPattern.MatchString(rendered) {
		t.Fatalf("rendered markdown contains multiple blank lines after ### heading")
	}
}

func TestRenderTableTemplate(t *testing.T) {
	t.Parallel()

	rendered, err := RenderFile(filepath.Join("testdata", "schema.fixture.json"), Options{
		TemplateName: "table",
	})
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	assertContains(t, rendered, "| Attribute | Value |")
}

func TestRenderIncludesDefinitionContents(t *testing.T) {
	t.Parallel()

	rendered, err := RenderFile(filepath.Join("testdata", "schema.fixture.json"), Options{})
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	assertContains(t, rendered, "## Contents")
	assertContains(t, rendered, "* [Config](#config)")
	assertContains(t, rendered, "* [Settings](#settings)")
}

func TestRenderCustomTemplate(t *testing.T) {
	t.Parallel()

	rendered, err := RenderFile(filepath.Join("testdata", "schema.fixture.json"), Options{
		TemplateText: "# {{ .Title }}\n{{ range .Definitions }}- {{ .Name }}\n{{ end }}\n",
	})
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	assertContains(t, rendered, "- Config")
	assertContains(t, rendered, "- Settings")
}

func TestRenderEmbedsExampleDocumentJSON(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
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
	}), Options{
		ExampleFormat: ExampleFormatJSON,
		ExampleMode:   ExampleModeRequired,
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "## Example json document")
	assertContains(t, rendered, "```json")
	assertContains(t, rendered, `"name": "<string>"`)
}

func TestRenderEmbedsExampleDocumentYAMLRequiredMode(t *testing.T) {
	t.Parallel()

	rendered, err := Render(minimalSchemaBytes(t, map[string]any{
		"$ref": "#/$defs/Config",
		"$defs": map[string]any{
			"Config": map[string]any{
				"type":     "object",
				"required": []any{"name"},
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
					"mode": map[string]any{
						"type":     "string",
						"examples": []any{"safe"},
					},
				},
			},
		},
	}), Options{
		ExampleFormat: ExampleFormatYAML,
		ExampleMode:   ExampleModeRequired,
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertContains(t, rendered, "## Example yaml document")
	assertContains(t, rendered, "```yaml")
	assertContains(t, rendered, "name: <string>")
	assertNotContains(t, rendered, "mode: safe")
}

func TestRenderFixturesFromExamples(t *testing.T) {
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

func TestBuiltinTemplates(t *testing.T) {
	t.Parallel()

	names := BuiltinTemplateNames()
	if strings.Join(names, ",") != "list,table" {
		t.Fatalf("unexpected template names: %v", names)
	}

	if _, err := BuiltinTemplate("missing"); err == nil {
		t.Fatalf("expected error for unknown template")
	}
}

func TestRenderOutputHasNoHTML(t *testing.T) {
	t.Parallel()

	rendered, err := RenderFile(filepath.Join("testdata", "schema.fixture.json"), Options{})
	if err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	htmlPattern := regexp.MustCompile(`<[A-Za-z/][^>]*>`)
	if htmlPattern.MatchString(rendered) {
		t.Fatalf("rendered markdown contains html tags")
	}
}

func TestRenderGoldenList(t *testing.T) {
	testRenderGoldenTemplate(t, "list", filepath.Join("testdata", "schema.golden.list.md"))
}

func TestRenderGoldenTable(t *testing.T) {
	testRenderGoldenTemplate(t, "table", filepath.Join("testdata", "schema.golden.table.md"))
}

func testRenderGoldenTemplate(t *testing.T, templateName, goldenPath string) {
	t.Helper()

	schemaPath := filepath.Join("testdata", "schema.fixture.json")
	const sourcePath = "testdata/schema.fixture.json"
	got, err := RenderFile(schemaPath, Options{
		Title:        "schema reference",
		SourcePath:   sourcePath,
		TemplateName: templateName,
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
