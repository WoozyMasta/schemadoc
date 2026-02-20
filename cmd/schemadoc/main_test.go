// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testModulePath = "github.com/woozymasta/schemadoc"

func TestRunSchemaToMarkdownWritesMarkdownToStdout(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaFixture(t, "https://json-schema.org/draft/2020-12/schema")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2md", schemaPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if stdout.Len() == 0 {
		t.Fatal("stdout is empty")
	}

	if !strings.Contains(stdout.String(), "# schema reference") {
		t.Fatalf("stdout does not contain default title: %s", stdout.String())
	}
}

func TestRunSchemaToMarkdownTemplateTable(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaFixture(t, "https://json-schema.org/draft/2020-12/schema")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2md", "--template", "table", schemaPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "| Attribute | Value |") {
		t.Fatalf("table output expected, got: %s", stdout.String())
	}
}

func TestRunSchemaToMarkdownFromStdin(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:test",
  "type": "object",
  "properties": {
    "name": { "type": "string" }
  }
}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runWithIO([]string{"schema2md"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "## Root") {
		t.Fatalf("expected root section in output: %s", stdout.String())
	}

	if strings.Contains(stdout.String(), "Source schema:") {
		t.Fatalf("stdin output should not include source schema marker: %s", stdout.String())
	}
}

func TestRunSchemaToMarkdownWritesMarkdownToOutputFile(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaFixture(t, "https://json-schema.org/draft/2020-12/schema")
	outPath := filepath.Join(t.TempDir(), "config.md")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2md", "--title", "Custom Doc", schemaPath, outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty when output path is provided, got: %s", stdout.String())
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out file: %v", err)
	}

	if !strings.Contains(string(content), "# Custom Doc") {
		t.Fatalf("output file does not contain custom title: %s", string(content))
	}
}

func TestRunSchemaToMarkdownWithTemplateFile(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaFixture(t, "https://json-schema.org/draft/2020-12/schema")
	customTemplatePath := filepath.Join(t.TempDir(), "custom.gotmpl")
	if err := os.WriteFile(customTemplatePath, []byte("# custom\n{{ range .Definitions }}- {{ .Name }}\n{{ end }}\n"), 0o600); err != nil {
		t.Fatalf("write custom template: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2md", "--template-file", customTemplatePath, schemaPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "# custom") {
		t.Fatalf("expected custom template output, got: %s", stdout.String())
	}
}

func TestRunSchemaToMarkdownWrapWidth(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:test",
  "type": "object",
  "properties": {
    "notes": {
      "type": "string",
      "description": "This description should be wrapped by words into shorter lines."
    }
  }
}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runWithIO([]string{"schema2md", "--wrap", "24"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "This description should") {
		t.Fatalf("missing wrapped line in output: %s", stdout.String())
	}

	if !strings.Contains(stdout.String(), "be wrapped by words into") {
		t.Fatalf("missing wrapped continuation in output: %s", stdout.String())
	}
}

func TestRunSchemaToMarkdownListMarker(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:test",
  "type": "object",
  "properties": {
    "template_name": {
      "type": "string",
      "description": "Supported values:\n\n - list\n - table"
    }
  }
}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runWithIO([]string{"schema2md", "--list-marker", "*"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "* list") {
		t.Fatalf("expected normalized list marker in output: %s", stdout.String())
	}

	if strings.Contains(stdout.String(), "\n - list") {
		t.Fatalf("expected list indent normalization, got: %s", stdout.String())
	}
}

func TestRunSchemaToMarkdownListMarkerAffectsTemplateLists(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaFixture(t, "https://json-schema.org/draft/2020-12/schema")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2md", "--list-marker", "-", schemaPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "- Schema ID:") {
		t.Fatalf("expected template metadata list marker '-', got: %s", stdout.String())
	}

	if strings.Contains(stdout.String(), "* Schema ID:") {
		t.Fatalf("expected template metadata marker to be '-', got: %s", stdout.String())
	}
}

func TestRunTemplateStdout(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"template"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "# {{ .Title }}") {
		t.Fatalf("expected template body, got: %s", stdout.String())
	}
}

func TestRunTemplateToOutputFile(t *testing.T) {
	t.Parallel()

	outPath := filepath.Join(t.TempDir(), "table.gotmpl")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"template", "--template", "table", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read exported template: %v", err)
	}

	if !strings.Contains(string(content), "| Attribute | Value |") {
		t.Fatalf("expected table template, got: %s", string(content))
	}
}

func TestRunSchemaToJSONWritesDefaultAllToStdout(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaExampleFixture(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2json", schemaPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if stdout.Len() == 0 {
		t.Fatal("stdout is empty")
	}

	assertContains(t, stdout.String(), `"mode": "safe"`)
	assertContains(t, stdout.String(), `"note": "<string>"`)
}

func TestRunSchemaToJSONWritesRequiredToOutputFile(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaExampleFixture(t)
	outputPath := filepath.Join(t.TempDir(), "config.required.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2json", "--mode", "required", schemaPath, outputPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty when output path is provided, got: %s", stdout.String())
	}

	requiredJSON, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read required json example: %v", err)
	}

	assertContains(t, string(requiredJSON), `"name": "demo"`)
	assertContains(t, string(requiredJSON), `"settings": {`)
	assertNotContains(t, string(requiredJSON), `"mode": "safe"`)
	assertNotContains(t, string(requiredJSON), `"note": "<string>"`)
}

func TestRunSchemaToYAMLIncludesSchemaComments(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaExampleFixture(t)
	outputPath := filepath.Join(t.TempDir(), "config.required.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2yaml", "--mode", "required", schemaPath, outputPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty when output path is provided, got: %s", stdout.String())
	}

	requiredYAML, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read required yaml example: %v", err)
	}

	assertContains(t, string(requiredYAML), "# Service Name")
	assertContains(t, string(requiredYAML), "# Human-readable service name.")
	assertContains(t, string(requiredYAML), "name: demo")
	assertContains(t, string(requiredYAML), "# Enabled")
	assertContains(t, string(requiredYAML), "# Enables processing pipeline.")
	assertContains(t, string(requiredYAML), "enabled: true")
}

func TestRunSchemaToMarkdownEmbedsExampleWithModeAndFormat(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaExampleFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2md", "--mode", "required", "--format", "yaml", schemaPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	rendered := stdout.String()
	assertContains(t, rendered, "## Example yaml document")
	assertContains(t, rendered, "```yaml")
	assertContains(t, rendered, "name: demo")
	assertContains(t, rendered, "enabled: true")
	assertNotContains(t, rendered, "mode: safe")
}

func TestRunMod2SchemaWritesSchemaToStdout(t *testing.T) {
	t.Parallel()

	moduleRoot := findModuleRoot(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"mod2schema", "--module-root", moduleRoot, "--type", "SchemaModel", testModulePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if stdout.Len() == 0 {
		t.Fatal("stdout is empty")
	}

	if !strings.Contains(stdout.String(), "\"SchemaModel\"") {
		t.Fatalf("schema output does not contain root model: %s", stdout.String())
	}
}

func TestRunMod2SchemaWritesSchemaToOutputFile(t *testing.T) {
	t.Parallel()

	moduleRoot := findModuleRoot(t)
	outPath := filepath.Join(t.TempDir(), "schema.model.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"mod2schema", "--module-root", moduleRoot, "--type", "SchemaModel", testModulePath, outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty when output path is provided, got: %s", stdout.String())
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	if !strings.Contains(string(content), "\"Options\"") {
		t.Fatalf("schema file does not contain options definition: %s", string(content))
	}
}

func TestRunMod2MarkdownWritesToStdout(t *testing.T) {
	t.Parallel()

	moduleRoot := findModuleRoot(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"mod2md", "--module-root", moduleRoot, "--type", "SchemaModel", testModulePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "# schema reference") {
		t.Fatalf("stdout does not contain default title: %s", stdout.String())
	}

	if !strings.Contains(stdout.String(), "## Options") {
		t.Fatalf("stdout does not contain generated model section: %s", stdout.String())
	}
}

func TestRunMod2MarkdownWritesToOutputFile(t *testing.T) {
	t.Parallel()

	moduleRoot := findModuleRoot(t)
	outPath := filepath.Join(t.TempDir(), "schema.model.md")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"mod2md", "--module-root", moduleRoot, "--type", "SchemaModel", "--title", "Schema Model", testModulePath, outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty when output path is provided, got: %s", stdout.String())
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	if !strings.Contains(string(content), "# Schema Model") {
		t.Fatalf("output file does not contain custom title: %s", string(content))
	}
}

func TestRunWarnsOnUnknownDraft(t *testing.T) {
	t.Parallel()

	schemaPath := writeSchemaFixture(t, "https://json-schema.org/draft/2099-01/schema")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2md", schemaPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stderr.String(), "warning: unsupported $schema value") {
		t.Fatalf("expected warning in stderr, got: %s", stderr.String())
	}
}

func TestRunReturnsErrorForMissingInputFile(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"schema2md", filepath.Join(t.TempDir(), "missing.json")}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code for missing schema file")
	}

	if !strings.Contains(stderr.String(), "read schema input:") {
		t.Fatalf("stderr does not contain render error prefix: %s", stderr.String())
	}
}

func TestRunReturnsErrorForInvalidTypeName(t *testing.T) {
	t.Parallel()

	moduleRoot := findModuleRoot(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"mod2schema", "--module-root", moduleRoot, "--type", "Schema-Model", testModulePath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stderr.String(), "run module schema generator:") {
		t.Fatalf("expected module schema generator error, got: %s", stderr.String())
	}
}

func TestRunReturnsErrorForMissingCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d, stderr: %s", code, stderr.String())
	}
}

func TestRunReturnsErrorForUnknownTemplate(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"template", "--template", "missing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d, stderr: %s", code, stderr.String())
	}

	if !strings.Contains(stderr.String(), "Invalid value") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir: %v", err)
	}

	for {
		candidate := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	t.Fatal("module root with go.mod was not found")
	return ""
}

func writeSchemaFixture(t *testing.T, schemaURI string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "schema.json")
	body := `{
  "$schema": "` + schemaURI + `",
  "$id": "urn:test",
  "$ref": "#/$defs/Config",
  "$defs": {
    "Config": {
      "type": "object",
      "properties": {
        "name": {
          "type": "string"
        }
      }
    }
  }
}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write schema fixture: %v", err)
	}

	return path
}

func writeSchemaExampleFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "schema.example.json")
	body := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/Config",
  "$defs": {
    "Config": {
      "type": "object",
      "required": ["name", "settings"],
      "properties": {
        "name": {
          "type": "string",
          "default": "demo",
          "title": "Service Name",
          "description": "Human-readable service name."
        },
        "mode": {
          "type": "string",
          "examples": ["safe"]
        },
        "settings": {
          "type": "object",
          "required": ["enabled"],
          "properties": {
            "enabled": {
              "type": "boolean",
              "default": true,
              "title": "Enabled",
              "description": "Enables processing pipeline."
            },
            "note": {
              "type": "string"
            }
          }
        }
      }
    }
  }
}`

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write schema fixture: %v", err)
	}

	return path
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
