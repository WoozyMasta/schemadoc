// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadBuildDefaultsPositions(t *testing.T) {
	t.Parallel()
	defaults := `defaults:
  check: true
  mod2schema:
    module: .
    package: example.com/config
  mod2doc:
    template: table
    wrap: 80
`
	jobs := []string{
		"schema: first.json\nmod2schema:\n  type: First\nmod2doc:\n  title: First\n",
		"schema: second.json\ncheck: false\nmod2schema:\n  type: Second\nmod2doc: null\n",
	}

	for position := range 3 {
		t.Run([]string{"first", "middle", "last"}[position], func(t *testing.T) {
			t.Parallel()
			documents := append([]string(nil), jobs[:position]...)
			documents = append(documents, defaults)
			documents = append(documents, jobs[position:]...)
			path := filepath.Join(t.TempDir(), "build.yaml")
			writeGeneratorConfigFile(t, path, strings.Join(documents, "---\n"))
			configs, err := loadBuildConfig(path, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(configs) != 2 {
				t.Fatalf("jobs=%d, want 2", len(configs))
			}
			first, second := configs[0], configs[1]
			if first.Mod2Schema.Module != "." || first.Mod2Schema.Package != "example.com/config" || first.Mod2Schema.Type != "First" {
				t.Fatalf("unexpected first target: %+v", first.Mod2Schema)
			}
			if !first.Check || first.Mod2Doc.Template != "table" || first.Mod2Doc.Wrap != 80 || first.Mod2Doc.Title != "First" {
				t.Fatalf("unexpected first job: %+v", first)
			}
			if second.Check || second.Mod2Doc != nil || second.Mod2Schema.Type != "Second" {
				t.Fatalf("unexpected second job: %+v", second)
			}
			selected, err := loadBuildConfig(path, 2)
			if err != nil || len(selected) != 1 || !reflect.DeepEqual(selected[0], second) {
				t.Fatalf("index selection=%+v, error=%v", selected, err)
			}
		})
	}
}

func TestMergeBuildDefaults(t *testing.T) {
	t.Parallel()
	defaults := map[string]any{
		"flag": true, "number": 80, "text": "default",
		"stage": map[string]any{"keep": "value", "remove": "value"},
		"list":  []any{"first", "second"},
	}
	overrides := map[string]any{
		"flag": false, "number": 0, "text": "",
		"stage": map[string]any{"remove": nil}, "list": []any{},
	}
	want := map[string]any{
		"flag": false, "number": 0, "text": "",
		"stage": map[string]any{"keep": "value"}, "list": []any{},
	}
	merged := mergeBuildDefaults(defaults, overrides)
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("merged=%#v, want %#v", merged, want)
	}
	merged["stage"].(map[string]any)["keep"] = "changed"
	if defaults["stage"].(map[string]any)["keep"] != "value" {
		t.Fatal("merge mutated defaults")
	}
}

func TestLoadBuildDefaultsErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, text string
		sentinel   error
	}{
		{"duplicate", "defaults: {}\n---\ndefaults: {}\n", errDuplicateBuildDefaults},
		{"mixed", "defaults: {}\nschema: config.json\n", errMixedBuildDefaults},
		{"null", "defaults: null\n", errInvalidBuildDefaults},
		{"scalar", "defaults: true\n", errInvalidBuildDefaults},
		{"unknown defaults", "defaults:\n  typo: true\n---\nschema: config.json\n", nil},
		{"bad defaults type", "defaults:\n  check: wrong\n---\nschema: config.json\ncheck: false\n", nil},
		{"unknown nested default", "defaults:\n  mod2schema:\n    typo: true\n---\nschema: config.json\nmod2schema: null\n", nil},
		{"missing schema", "defaults: {}\n---\ncheck: false\n", nil},
		{"missing type", "defaults:\n  mod2schema:\n    module: .\n---\nschema: config.json\n", nil},
		{"defaults only", "defaults: {}\n", nil},
		{"bad job type", "defaults: {}\n---\nschema: config.json\ncheck: wrong\n", nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "build.yaml")
			writeGeneratorConfigFile(t, path, test.text)
			_, err := loadBuildConfig(path, 0)
			if err == nil {
				t.Fatal("invalid config accepted")
			}
			if test.sentinel != nil && !errors.Is(err, test.sentinel) {
				t.Fatalf("error=%v, want %v", err, test.sentinel)
			}
		})
	}
}

func TestBuildDuplicateDefaultsDoesNotWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	output := filepath.Join(dir, "example.json")
	path := filepath.Join(dir, "build.yaml")
	text := "defaults: {}\n---\nschema: " + filepath.ToSlash(repoTestdataPath(t, "generated", "app.schema.json")) +
		"\nschema2json:\n  output: " + filepath.ToSlash(output) + "\n---\ndefaults: {}\n"
	writeGeneratorConfigFile(t, path, text)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"build", path}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit=%d: %s", code, stderr.String())
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid stream wrote output: %v", err)
	}
}

// writeGeneratorConfigFile writes a raw stream, preserving explicit null and false values.
func writeGeneratorConfigFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
