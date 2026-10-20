// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/woozymasta/schemadoc/cmd/schemadoc/buildcfg"
)

func TestCLI_BuildCheckFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	schema := repoTestdataPath(t, "generated", "app.schema.json")
	configPath := filepath.Join(dir, "build.yaml")
	jsonPath, docPath, yamlPath := filepath.Join(dir, "example.json"), filepath.Join(dir, "reference.md"), filepath.Join(dir, "example.yaml")
	writeBuildTestConfig(t, configPath,
		buildcfg.Config{
			Schema:      schema,
			Schema2JSON: &buildcfg.Schema2JSONStage{Output: jsonPath},
			Schema2Doc:  &buildcfg.Schema2DocStage{Output: docPath},
		},
		buildcfg.Config{Schema: schema, Schema2YAML: &buildcfg.Schema2YAMLStage{Output: yamlPath}},
	)
	var stdout, stderr bytes.Buffer

	if code := run([]string{"build", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("generate exit=%d: %s", code, stderr.String())
	}

	for _, flag := range []string{"--check", "-c"} {
		stdout.Reset()
		stderr.Reset()

		if code := run([]string{"build", flag, configPath}, &stdout, &stderr); code != 0 {
			t.Fatalf("matching %s exit=%d: %s", flag, code, stderr.String())
		}
	}

	cases := []struct {
		stage string
		path  string
	}{
		{"schema2json", jsonPath},
		{"schema2doc", docPath},
		{"schema2yaml", yamlPath},
	}

	for _, test := range cases {
		t.Run(test.stage, func(t *testing.T) {
			original, err := os.ReadFile(test.path)
			if err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(test.path, []byte("stale\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			stdout.Reset()
			stderr.Reset()

			if code := run([]string{"build", "--check", configPath}, &stdout, &stderr); code != 1 {
				t.Fatalf("stale exit=%d: %s", code, stderr.String())
			}

			checkContainsAll(t, stderr.String(), []string{"stage " + test.stage, "content differs"})
			content, err := os.ReadFile(test.path)
			if err != nil || string(content) != "stale\n" {
				t.Fatalf("check changed output: %q, error=%v", content, err)
			}

			if test.path == jsonPath {
				stdout.Reset()
				stderr.Reset()

				if code := run([]string{"build", "--check", "--index", "2", configPath}, &stdout, &stderr); code != 0 {
					t.Fatalf("selected document exit=%d: %s", code, stderr.String())
				}
			}

			if err := os.WriteFile(test.path, original, 0o600); err != nil {
				t.Fatal(err)
			}
		})
	}

	missingPath := filepath.Join(dir, "missing.yaml")
	writeBuildTestConfig(t, configPath, buildcfg.Config{
		Schema: schema, Schema2YAML: &buildcfg.Schema2YAMLStage{Output: missingPath},
	})
	stdout.Reset()
	stderr.Reset()

	if code := run([]string{"build", "--check", configPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("missing output exit=%d: %s", code, stderr.String())
	}

	checkContainsAll(t, stderr.String(), []string{"file does not exist"})
	if _, err := os.Stat(missingPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("check created missing output: %v", err)
	}
}

func TestCLI_BuildCheckFlagRejectsLatest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "build.yaml")
	writeBuildTestConfig(t, configPath, buildcfg.Config{
		Schema:     filepath.Join(dir, "schema.json"),
		Mod2Schema: &buildcfg.Mod2SchemaStage{Module: "example.com/config@latest", Type: "Config"},
	})
	var stdout, stderr bytes.Buffer

	if code := run([]string{"build", "--check", configPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("latest exit=%d: %s", code, stderr.String())
	}

	checkContainsAll(t, stderr.String(), []string{"stage mod2schema", "@latest is not allowed in check mode"})
}
