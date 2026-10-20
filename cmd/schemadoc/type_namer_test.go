// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/woozymasta/schemadoc/cmd/schemadoc/buildcfg"
)

func TestCLI_TypeNamer(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(findModuleRoot(t), "modschema", "testdata", "collisions", "root")

	for _, command := range []string{"mod2schema", "mod2doc"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			for _, mode := range []string{"", "package", "full", "invalid"} {
				args := []string{command, "--type", "Config"}

				if mode != "" {
					args = append(args, "--type-namer", mode)
				}

				args = append(args, moduleDir)
				var stdout, stderr bytes.Buffer
				code := run(args, &stdout, &stderr)

				if mode == "full" {
					if code != 0 {
						t.Fatalf("full %s exit=%d: %s", command, code, stderr.String())
					}

					checkContainsAll(t, stdout.String(), []string{
						"github_com_acme_a_common_478c535a9efb9d99_Config",
						"github_com_acme_b_common_cb563cf1708713af_Config",
					})
					continue
				}

				if code == 0 || stdout.Len() != 0 {
					t.Fatalf("mode=%q exit=%d, stdout=%q, stderr=%s", mode, code, stdout.String(), stderr.String())
				}

				if mode == "invalid" {
					checkContainsAll(t, stderr.String(), []string{"type-namer", "package", "full"})
				} else {
					checkContainsAll(t, stderr.String(), []string{
						`definition name collision "common_Config"`, "github.com/acme/a/common.Config",
						"github.com/acme/b/common.Config", "--type-namer full",
					})
				}
			}
		})
	}
}

func TestCLI_BuildTypeNamer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "config.schema.json")
	docPath := filepath.Join(dir, "config.md")
	configPath := filepath.Join(dir, "build.yaml")
	config := buildcfg.Config{
		Schema: schemaPath,
		Mod2Schema: &buildcfg.Mod2SchemaStage{
			Module: filepath.Join(findModuleRoot(t), "modschema", "testdata", "collisions", "root"),
			Type:   "Config",
		},
		Mod2Doc: &buildcfg.Mod2DocStage{
			Schema2DocStage: buildcfg.Schema2DocStage{Output: docPath},
		},
	}

	if err := os.WriteFile(schemaPath, []byte("unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	writeBuildTestConfig(t, configPath, config)
	var stdout, stderr bytes.Buffer

	if code := run([]string{"build", configPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("colliding build exit=%d: %s", code, stderr.String())
	}

	checkContainsAll(t, stderr.String(), []string{"stage mod2schema", "definition name collision"})
	unchanged, err := os.ReadFile(schemaPath)
	if err != nil || string(unchanged) != "unchanged\n" {
		t.Fatalf("colliding build changed output: %q, error=%v", unchanged, err)
	}

	config.Mod2Schema.TypeNamer = "full"
	writeBuildTestConfig(t, configPath, config)
	stdout.Reset()
	stderr.Reset()

	if code := run([]string{"build", configPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("full build exit=%d: %s", code, stderr.String())
	}

	for _, path := range []string{schemaPath, docPath} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		checkContainsAll(t, string(content), []string{
			"github_com_acme_a_common_478c535a9efb9d99_Config",
			"github_com_acme_b_common_cb563cf1708713af_Config",
		})
	}

	config.Mod2Schema.TypeNamer = "invalid"
	writeBuildTestConfig(t, configPath, config)

	if _, err := loadBuildConfig(configPath, 0); err == nil {
		t.Fatal("build schema accepted an invalid type_namer")
	}
}
