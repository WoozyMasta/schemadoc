// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woozymasta/schemadoc/cmd/schemadoc/buildcfg"
	"github.com/woozymasta/schemadoc/modschema"
	"go.yaml.in/yaml/v3"
)

func TestCLI_BuildModuleDocs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	moduleDir := filepath.Join(findModuleRoot(t), "modschema", "testdata", "comments")
	schemaPath := filepath.Join(dir, "config.schema.json")
	rawPath := filepath.Join(dir, "raw.md")
	plainPath := filepath.Join(dir, "config.schema.table.md")
	configPath := filepath.Join(dir, "build.yaml")
	config := buildcfg.Config{
		Schema: schemaPath,
		Mod2Schema: &buildcfg.Mod2SchemaStage{
			Module: moduleDir, Type: "Config", Comments: "plain",
			RootID: "https://example.com/config.schema.json", JSONSchemaVersion: "v0.14.0",
		},
		Mod2Doc: &buildcfg.Mod2DocStage{
			Schema2DocStage: buildcfg.Schema2DocStage{
				Output: rawPath, Title: "Comment reference", Wrap: 200,
			},
		},
	}
	plainConfig := config
	plainConfig.Mod2Doc = &buildcfg.Mod2DocStage{
		Schema2DocStage: buildcfg.Schema2DocStage{
			Template: "table", Title: "Comment reference", Wrap: 200,
			SourcePath: "custom-source", SourceRawPath: "https://example.com/raw.json",
		},
		Comments: "plain",
	}
	writeBuildTestConfig(t, configPath, config, plainConfig)
	var stdout, stderr bytes.Buffer

	if code := run([]string{"build", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("build exit=%d: %s", code, stderr.String())
	}

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}

	var schema map[string]any

	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatal(err)
	}

	if schema["$id"] != config.Mod2Schema.RootID {
		t.Fatalf("root ID was not propagated: %v", schema["$id"])
	}

	checkContainsAll(t, string(schemaBytes), []string{
		"Text spans physically wrapped source lines with SomeType and SomeType.Method.",
		"the guide (https://example.com/guide)",
	})
	checkNotContainsAny(t, string(schemaBytes), []string{"# Allowed usage", "[the guide]:"})
	rawDoc, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	args := []string{
		"mod2doc", "--type", "Config", "--comments", "raw", "--title", "Comment reference", "--wrap", "200",
		"--root-id", config.Mod2Schema.RootID, "--jsonschema-version", config.Mod2Schema.JSONSchemaVersion, moduleDir,
	}

	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("direct mod2doc exit=%d: %s", code, stderr.String())
	}

	if !bytes.Equal(rawDoc, stdout.Bytes()) {
		t.Fatal("build mod2doc default differs from direct raw rendering")
	}

	checkContainsAll(t, string(rawDoc), []string{"module:example.com/comments.Config"})
	plainDoc, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatal(err)
	}

	checkContainsAll(t, string(plainDoc), []string{
		"SomeType and SomeType.Method.", "the guide (https://example.com/guide)",
		"custom-source", "https://example.com/raw.json",
	})
	checkNotContainsAny(t, string(plainDoc), []string{"module:example.com/comments.Config"})
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 4 {
		t.Fatalf("unexpected files, including possible transient schema: %v", entries)
	}

	stdout.Reset()
	stderr.Reset()

	if code := run([]string{"build", "--check", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("matching check exit=%d: %s", code, stderr.String())
	}

	if err := os.WriteFile(rawPath, []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()

	if code := run([]string{"build", "--check", configPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("stale check exit=%d: %s", code, stderr.String())
	}

	checkContainsAll(t, stderr.String(), []string{"stage mod2doc", "content differs"})
	staleDoc, err := os.ReadFile(rawPath)
	if err != nil || string(staleDoc) != "stale\n" {
		t.Fatalf("check rewrote stale output: %q, error=%v", staleDoc, err)
	}

	checkedSchema, err := os.ReadFile(schemaPath)
	if err != nil || !bytes.Equal(checkedSchema, schemaBytes) {
		t.Fatalf("check changed persisted schema: %v", err)
	}
}

func TestCLI_BuildModuleDocsBeforeMerge(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	patchPath := filepath.Join(dir, "patch.json")
	if err := os.WriteFile(patchPath, []byte(`"Merged description."`), 0o600); err != nil {
		t.Fatal(err)
	}

	config := buildcfg.Config{
		Schema: filepath.Join(dir, "config.schema.json"),
		Mod2Schema: &buildcfg.Mod2SchemaStage{
			Module: filepath.Join(findModuleRoot(t), "modschema", "testdata", "comments"),
			Type:   "Config", Comments: "plain",
		},
		Mod2Doc: &buildcfg.Mod2DocStage{
			Schema2DocStage: buildcfg.Schema2DocStage{Output: filepath.Join(dir, "source.md")},
			Comments:        "raw",
		},
		Merge: &buildcfg.MergeStage{Patches: []buildcfg.MergePatch{{
			File: patchPath, Target: "/$defs/Config/properties/text/description", Op: buildcfg.MergeOp{Node: "replace"},
		}}},
		Schema2JSON: &buildcfg.Schema2JSONStage{Output: filepath.Join(dir, "example.json")},
		Schema2Doc:  &buildcfg.Schema2DocStage{Output: filepath.Join(dir, "merged.md")},
		Schema2YAML: &buildcfg.Schema2YAMLStage{Output: filepath.Join(dir, "example.yaml")},
	}
	configPath := filepath.Join(dir, "build.yaml")
	writeBuildTestConfig(t, configPath, config)
	var stdout, stderr bytes.Buffer

	if code := run([]string{"build", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("build exit=%d: %s", code, stderr.String())
	}

	last := -1

	for _, stage := range []string{"mod2schema", "mod2doc", "merge", "schema2json", "schema2doc", "schema2yaml"} {
		position := strings.Index(stderr.String(), "stage="+stage+" ")
		if position <= last {
			t.Fatalf("stage %s is missing or out of order: %s", stage, stderr.String())
		}

		last = position
	}

	sourceDoc, err := os.ReadFile(config.Mod2Doc.Output)
	if err != nil {
		t.Fatal(err)
	}

	mergedDoc, err := os.ReadFile(config.Schema2Doc.Output)
	if err != nil {
		t.Fatal(err)
	}

	checkNotContainsAny(t, string(sourceDoc), []string{"Merged description."})
	checkContainsAll(t, string(mergedDoc), []string{"Merged description."})
}

func TestBuildModuleDocsValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		want string
	}{
		{"missing source", "mod2doc: {}", errMod2DocRequiresMod2Schema.Error()},
		{"invalid schema comments", "mod2schema: {type: Config, comments: invalid}", "comments"},
		{"invalid doc comments", "mod2schema: {type: Config}\nmod2doc: {comments: invalid}", "comments"},
		{"duplicated source", "mod2schema: {type: Config}\nmod2doc: {module: .}", "module"},
		{"duplicated version", "mod2schema: {type: Config}\nmod2doc: {jsonschema_version: v0.14.0}", "jsonschema_version"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "build.yaml")

			if err := os.WriteFile(path, []byte("schema: missing.json\n"+test.body+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := loadBuildConfig(path, 0)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("loadBuildConfig error=%v, want %q", err, test.want)
			}

			if test.name == "missing source" && !errors.Is(err, errMod2DocRequiresMod2Schema) {
				t.Fatalf("missing stage dependency error=%v", err)
			}
		})
	}
}

func TestBuildModuleDocsReuseAndCleanup(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMP", tempRoot)
	t.Setenv("TEMP", tempRoot)
	t.Setenv("TMPDIR", tempRoot)
	dir := t.TempDir()
	generator := modschema.NewGenerator()
	t.Cleanup(func() {
		if err := generator.Close(); err != nil {
			t.Error(err)
		}
	})
	runner := &cliRunner{stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard}
	config := buildcfg.Config{
		Schema: filepath.Join(dir, "config.schema.json"),
		Mod2Schema: &buildcfg.Mod2SchemaStage{
			Module: filepath.Join(findModuleRoot(t), "modschema", "testdata", "comments"),
			Type:   "Config", Comments: "plain",
		},
		Mod2Doc: &buildcfg.Mod2DocStage{},
	}
	var workspaceDir string
	templatePath := filepath.Join(dir, "custom.gotmpl")
	templateText := "{{/*\nschemadoc:\n  output:\n    extension: .txt\n  postprocess: []\n*/ -}}\nCUSTOM DOC\n"

	if err := os.WriteFile(templatePath, []byte(templateText), 0o600); err != nil {
		t.Fatal(err)
	}

	for index := range 2 {
		if index == 1 {
			config.Mod2Doc.TemplateFile = templatePath
		}

		if err := runner.runLoadedConfig(index+1, config, generator); err != nil {
			t.Fatal(err)
		}

		dirs, err := filepath.Glob(filepath.Join(tempRoot, "schemadoc-mod2schema-*"))
		if err != nil || len(dirs) != 1 {
			t.Fatalf("workspace dirs=%v, error=%v", dirs, err)
		}

		if index == 0 {
			workspaceDir = dirs[0]
		} else if dirs[0] != workspaceDir {
			t.Fatal("second document did not reuse the same workspace")
		}
	}

	customDoc, err := os.ReadFile(filepath.Join(dir, "config.schema.txt"))
	if err != nil || string(customDoc) != "CUSTOM DOC" {
		t.Fatalf("custom template output=%q, error=%v", customDoc, err)
	}

	if err := generator.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(workspaceDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace not removed: %v", err)
	}

	config.Mod2Doc.TemplateFile = filepath.Join(dir, "missing.gotmpl")
	config.Mod2Doc.Output = filepath.Join(dir, "failed.md")
	configPath := filepath.Join(dir, "failure.yaml")
	writeBuildTestConfig(t, configPath, config)

	if err := runner.runConfig(configPath, 0, false); err == nil {
		t.Fatal("missing template accepted")
	}

	dirs, err := filepath.Glob(filepath.Join(tempRoot, "schemadoc-mod2schema-*"))
	if err != nil || len(dirs) != 0 {
		t.Fatalf("build failure leaked workspaces: %v, error=%v", dirs, err)
	}
}

func BenchmarkBuildModuleDocs(b *testing.B) {
	dir := b.TempDir()
	moduleDir, err := filepath.Abs(filepath.Join("..", "..", "modschema", "testdata", "comments"))
	if err != nil {
		b.Fatal(err)
	}

	config := buildcfg.Config{
		Schema: filepath.Join(dir, "config.schema.json"),
		Mod2Schema: &buildcfg.Mod2SchemaStage{
			Module: moduleDir, Type: "Config", Comments: "plain",
		},
		Mod2Doc: &buildcfg.Mod2DocStage{},
	}
	configPath := filepath.Join(dir, "build.yaml")
	writeBuildTestConfig(b, configPath, config)
	runner := &cliRunner{stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard}

	for b.Loop() {
		if err := runner.runConfig(configPath, 0, false); err != nil {
			b.Fatal(err)
		}
	}
}

// writeBuildTestConfig encodes typed configs as an ordinary multi-document YAML stream.
func writeBuildTestConfig(t testing.TB, path string, configs ...buildcfg.Config) {
	t.Helper()
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)

	for _, config := range configs {
		if err := encoder.Encode(config); err != nil {
			t.Fatal(err)
		}
	}

	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}
