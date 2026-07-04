// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkParseDocument measures schema decoding and normalization cost.
func BenchmarkParseDocument(b *testing.B) {
	schemaPath := filepath.Join("testdata", "app.schema.json")
	schemaBytes := readBenchmarkFile(b, schemaPath)

	b.ReportAllocs()
	b.SetBytes(int64(len(schemaBytes)))

	for i := 0; i < b.N; i++ {
		if _, err := parseDocument(schemaBytes); err != nil {
			b.Fatalf("parseDocument: %v", err)
		}
	}
}

// BenchmarkParseDocumentScale measures parse cost for different schema shapes.
func BenchmarkParseDocumentScale(b *testing.B) {
	schemaPath := filepath.Join("testdata", "app.schema.json")
	fixtureSchemaBytes := readBenchmarkFile(b, schemaPath)

	benchmarks := []struct {
		name   string
		schema []byte
	}{
		{
			name:   "bool_root",
			schema: []byte(`true`),
		},
		{
			name: "minimal_object",
			schema: []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "name": { "type": "string" }
  }
}`),
		},
		{
			name:   "fixture",
			schema: fixtureSchemaBytes,
		},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(benchmark.schema)))

			for i := 0; i < b.N; i++ {
				if _, err := parseDocument(benchmark.schema); err != nil {
					b.Fatalf("parseDocument: %v", err)
				}
			}
		})
	}
}

// BenchmarkRenderListTemplate measures full in-memory render flow for list template.
func BenchmarkRenderListTemplate(b *testing.B) {
	benchmarkRenderTemplate(b, "list")
}

// BenchmarkRenderTableTemplate measures full in-memory render flow for table template.
func BenchmarkRenderTableTemplate(b *testing.B) {
	benchmarkRenderTemplate(b, "table")
}

// BenchmarkRenderHTMLTemplate measures full in-memory render flow for html template.
func BenchmarkRenderHTMLTemplate(b *testing.B) {
	benchmarkRenderTemplate(b, "html")
}

// BenchmarkRenderFileListTemplate measures read + render flow from file path.
func BenchmarkRenderFileListTemplate(b *testing.B) {
	schemaPath := filepath.Join("testdata", "app.schema.json")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := RenderFile(schemaPath, Options{
			Title:        "schema reference",
			TemplateName: "list",
		})
		if err != nil {
			b.Fatalf("RenderFile: %v", err)
		}
	}
}

// benchmarkRenderTemplate runs common in-memory benchmark for selected template.
func benchmarkRenderTemplate(b *testing.B, templateName string) {
	schemaPath := filepath.Join("testdata", "app.schema.json")
	schemaBytes := readBenchmarkFile(b, schemaPath)

	options := Options{
		Title:        "schema reference",
		SourcePath:   schemaPath,
		TemplateName: templateName,
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(schemaBytes)))

	for i := 0; i < b.N; i++ {
		_, err := Render(schemaBytes, options)
		if err != nil {
			b.Fatalf("Render: %v", err)
		}
	}
}

// readBenchmarkFile loads benchmark fixture file and fails benchmark on read errors.
func readBenchmarkFile(b *testing.B, path string) []byte {
	b.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("read benchmark file %q: %v", path, err)
	}

	if len(data) == 0 {
		b.Fatalf("empty benchmark file: %s", path)
	}

	return data
}
