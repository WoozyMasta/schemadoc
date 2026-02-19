// SPDX-License-Identifier: AGPL-3.0-only
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
	schemaPath := filepath.Join("testdata", "schema.fixture.json")
	schemaBytes := readBenchmarkFile(b, schemaPath)

	b.ReportAllocs()
	b.SetBytes(int64(len(schemaBytes)))

	for i := 0; i < b.N; i++ {
		if _, err := parseDocument(schemaBytes); err != nil {
			b.Fatalf("parseDocument: %v", err)
		}
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

// BenchmarkRenderFileListTemplate measures read + render flow from file path.
func BenchmarkRenderFileListTemplate(b *testing.B) {
	schemaPath := filepath.Join("testdata", "schema.fixture.json")

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
	schemaPath := filepath.Join("testdata", "schema.fixture.json")
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
