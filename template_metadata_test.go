// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseTemplateMetadataAbsent(t *testing.T) {
	t.Parallel()

	metadata, err := parseTemplateMetadata("plain output")
	if err != nil {
		t.Fatalf("parseTemplateMetadata: %v", err)
	}
	if !reflect.DeepEqual(metadata, templateMetadata{}) {
		t.Fatalf("metadata = %#v, want empty metadata", metadata)
	}
}

func TestParseTemplateMetadataValid(t *testing.T) {
	t.Parallel()

	source := `{{/*
schemadoc:
  output:
    extension: ".custom"
  postprocess:
    - normalize-line-endings
    - trailing-newline
*/ -}}
custom`

	metadata, err := parseTemplateMetadata(source)
	if err != nil {
		t.Fatalf("parseTemplateMetadata: %v", err)
	}

	if metadata.Output == nil || metadata.Output.Extension == nil ||
		*metadata.Output.Extension != ".custom" {
		t.Fatalf("unexpected output metadata: %#v", metadata.Output)
	}
	wantProcessors := []string{"normalize-line-endings", "trailing-newline"}
	if !reflect.DeepEqual(metadata.Postprocess, wantProcessors) {
		t.Fatalf("processors = %v, want %v", metadata.Postprocess, wantProcessors)
	}
}

func TestParseTemplateMetadataEmptyProcessorList(t *testing.T) {
	t.Parallel()

	metadata, err := parseTemplateMetadata(`{{/*
schemadoc:
  postprocess: []
*/}}output`)
	if err != nil {
		t.Fatalf("parseTemplateMetadata: %v", err)
	}
	if metadata.Postprocess == nil || len(metadata.Postprocess) != 0 {
		t.Fatalf("postprocess = %#v, want explicit empty list", metadata.Postprocess)
	}
}

func TestParseTemplateMetadataRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   error
	}{
		{
			name:   "malformed",
			source: "{{/* schemadoc: [ */}}output",
			want:   ErrParseTemplateMetadata,
		},
		{
			name: "unknown field",
			source: `{{/*
schemadoc:
  unsupported: true
*/}}output`,
			want: ErrParseTemplateMetadata,
		},
		{
			name: "unknown processor",
			source: `{{/*
schemadoc:
  postprocess:
    - missing
*/}}output`,
			want: ErrUnknownTemplatePostProcessor,
		},
		{
			name: "duplicate processor",
			source: `{{/*
schemadoc:
  postprocess:
    - trailing-newline
    - trailing-newline
*/}}output`,
			want: ErrDuplicateTemplatePostProcessor,
		},
		{
			name: "invalid extension",
			source: `{{/*
schemadoc:
  output:
    extension: "md"
*/}}output`,
			want: ErrInvalidTemplateExtension,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseTemplateMetadata(test.source)
			if !errors.Is(err, test.want) {
				t.Fatalf("parseTemplateMetadata error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestTemplateExtensionUsesPortableGrammar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		extension string
		valid     bool
	}{
		{extension: ".md", valid: true},
		{extension: ".html", valid: true},
		{extension: ".1", valid: true},
		{extension: ".tar.md", valid: true},
		{extension: ".", valid: false},
		{extension: "..", valid: false},
		{extension: "md", valid: false},
		{extension: ".man page", valid: false},
		{extension: ".md:backup", valid: false},
		{extension: ".md*", valid: false},
		{extension: ".md?", valid: false},
		{extension: ".md\"", valid: false},
		{extension: ".md|", valid: false},
		{extension: ".md/extra", valid: false},
		{extension: ".мд", valid: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.extension, func(t *testing.T) {
			t.Parallel()

			if got := validTemplateExtension(test.extension); got != test.valid {
				t.Fatalf("validTemplateExtension(%q) = %t, want %t", test.extension, got, test.valid)
			}
		})
	}
}

func TestTemplatePostProcessorOrdering(t *testing.T) {
	t.Parallel()

	first, err := applyTemplatePostProcessors("value\n\n", templateMetadata{
		Postprocess: []string{"trailing-newline", "normalize-markdown-spacing"},
	})
	if err != nil {
		t.Fatalf("applyTemplatePostProcessors first: %v", err)
	}
	second, err := applyTemplatePostProcessors("value\n\n", templateMetadata{
		Postprocess: []string{"normalize-markdown-spacing", "trailing-newline"},
	})
	if err != nil {
		t.Fatalf("applyTemplatePostProcessors second: %v", err)
	}

	if first != "value" || second != "value\n" {
		t.Fatalf("processor ordering outputs = %q and %q", first, second)
	}
}

func TestTemplatePostProcessorsAreDeterministicAndIdempotent(t *testing.T) {
	t.Parallel()

	metadata := templateMetadata{Postprocess: []string{
		"normalize-line-endings",
		"normalize-markdown-spacing",
		"trim-trailing-whitespace",
		"trailing-newline",
	}}
	input := "one\r\n\r\n\r\ntwo\t\r\n"

	first, err := applyTemplatePostProcessors(input, metadata)
	if err != nil {
		t.Fatalf("applyTemplatePostProcessors: %v", err)
	}
	second, err := applyTemplatePostProcessors(first, metadata)
	if err != nil {
		t.Fatalf("applyTemplatePostProcessors second pass: %v", err)
	}
	if first != second {
		t.Fatalf("processors are not idempotent: first %q, second %q", first, second)
	}
}

func TestBuiltinTemplatesDeclareMetadata(t *testing.T) {
	t.Parallel()

	wantExtensions := map[string]string{
		"list":  ".md",
		"table": ".md",
		"html":  ".html",
	}
	for name, wantExtension := range wantExtensions {
		name, wantExtension := name, wantExtension
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			source, err := BuiltinTemplate(name)
			if err != nil {
				t.Fatalf("BuiltinTemplate: %v", err)
			}
			metadata, err := parseTemplateMetadata(source)
			if err != nil {
				t.Fatalf("parseTemplateMetadata: %v", err)
			}
			if metadata.Output == nil || metadata.Output.Extension == nil ||
				*metadata.Output.Extension != wantExtension {
				t.Fatalf("extension = %#v, want %q", metadata.Output, wantExtension)
			}
			if len(metadata.Postprocess) == 0 {
				t.Fatal("builtin template declares no post-processors")
			}
		})
	}
}

func TestRenderCustomTemplateWithoutMetadataPreservesOutput(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{"type": "string"})
	got, err := Render(schema, Options{TemplateText: "MAN\n\n\nEND"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if got != "MAN\n\n\nEND" {
		t.Fatalf("custom output = %q", got)
	}
	if strings.Contains(got, "\n\nEND") == false {
		t.Fatalf("custom output lost deliberate blank lines: %q", got)
	}
}

func TestRenderCustomTemplateUsesAutoGeneratedNote(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{"type": "string"})
	got, err := Render(schema, Options{
		TemplateText:      "<!-- {{ .AutoGeneratedNote }} -->",
		AutoGeneratedNote: "Generated by test-tool.",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if got != "<!-- Generated by test-tool. -->" {
		t.Fatalf("custom generated note = %q", got)
	}
}

func TestRenderTemplateMetadataTrimMarkerDoesNotEmitNewline(t *testing.T) {
	t.Parallel()

	schema := minimalSchemaBytes(t, map[string]any{"type": "string"})
	got, err := Render(schema, Options{TemplateText: `{{/*
schemadoc:
  postprocess: []
*/ -}}
MANUAL(1)`})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if got != "MANUAL(1)" {
		t.Fatalf("metadata trim marker output = %q", got)
	}
}
