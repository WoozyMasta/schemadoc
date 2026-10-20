// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypeNamerModeValidation(t *testing.T) {
	t.Parallel()

	if got := NormalizeOptions(Options{}).TypeNamer; got != TypeNamerModePackage {
		t.Fatalf("default type namer=%q, want package", got)
	}

	for _, mode := range []TypeNamerMode{"invalid", "FULL", " full "} {
		options := Options{Module: "missing-module", TypeNamer: mode}
		data, _, err := Generate(options)

		if !errors.Is(err, ErrInvalidTypeNamerMode) || len(data) != 0 {
			t.Fatalf("Generate(%q) data=%q, error=%v", mode, data, err)
		}

		if _, err := BuildProgramSource(options); !errors.Is(err, ErrInvalidTypeNamerMode) {
			t.Fatalf("BuildProgramSource(%q) error=%v", mode, err)
		}
	}
}

func TestGeneratorTypeNameCollisions(t *testing.T) {
	t.Parallel()

	moduleDir, err := filepath.Abs(filepath.Join("testdata", "collisions", "root"))
	if err != nil {
		t.Fatal(err)
	}

	generator := NewGenerator()
	t.Cleanup(func() {
		if err := generator.Close(); err != nil {
			t.Error(err)
		}
	})
	options := Options{Module: moduleDir, Type: "Single"}
	defaultSchema, _, err := generator.Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	options.TypeNamer = TypeNamerModePackage
	packageSchema, _, err := generator.Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(defaultSchema, packageSchema) || !bytes.Contains(defaultSchema, []byte(`"common_Config"`)) {
		t.Fatal("default non-colliding names changed")
	}

	options.Type = "Config"
	options.TypeNamer = ""
	assertTypeNameCollision(t, generator, options)
	options.TypeNamer = TypeNamerModePackage
	assertTypeNameCollision(t, generator, options)
	options.TypeNamer = TypeNamerModeFull
	fullSchema, _, err := generator.Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	var schema struct {
		Ref  string `json:"$ref"`
		Defs map[string]struct {
			Properties map[string]struct {
				Ref   string `json:"$ref"`
				Items struct {
					Ref string `json:"$ref"`
				} `json:"items"`
			} `json:"properties"`
		} `json:"$defs"`
	}

	if err := json.Unmarshal(fullSchema, &schema); err != nil {
		t.Fatal(err)
	}

	const firstName = "github_com_acme_a_common_478c535a9efb9d99_Config"
	const secondName = "github_com_acme_b_common_cb563cf1708713af_Config"
	properties := schema.Defs["Config"].Properties

	if schema.Ref != "#/$defs/Config" || len(schema.Defs) != 4 {
		t.Fatalf("unexpected definitions: %s", fullSchema)
	}

	if properties["first"].Ref != "#/$defs/"+firstName || properties["second"].Ref != "#/$defs/"+secondName ||
		properties["again"].Items.Ref != properties["first"].Ref || properties["local"].Ref != "#/$defs/Local" {
		t.Fatalf("incorrect full refs: %s", fullSchema)
	}

	if _, exists := schema.Defs[firstName].Properties["a"]; !exists {
		t.Fatal("first external definition was lost")
	}

	if _, exists := schema.Defs[secondName].Properties["b"]; !exists {
		t.Fatal("second external definition was lost")
	}

	if schema.Defs[firstName].Properties["next"].Ref != "#/$defs/"+firstName {
		t.Fatal("recursive pointer did not reuse the same definition")
	}

	repeatedSchema, _, err := generator.Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(fullSchema, repeatedSchema) {
		t.Fatal("full names changed between helper processes")
	}

	options.TypeNamer = TypeNamerModePackage
	assertTypeNameCollision(t, generator, options)

	if len(generator.workspaces) != 1 {
		t.Fatalf("type naming modes created %d workspaces, want 1", len(generator.workspaces))
	}
}

// assertTypeNameCollision checks subprocess diagnostics and rejects any schema bytes.
func assertTypeNameCollision(t *testing.T, generator *Generator, options Options) {
	t.Helper()
	data, _, err := generator.Generate(options)
	if err == nil || len(data) != 0 {
		t.Fatalf("colliding generation data=%q, error=%v", data, err)
	}

	for _, part := range []string{
		`definition name collision "common_Config"`, "github.com/acme/a/common.Config",
		"github.com/acme/b/common.Config", "--type-namer full", "rename",
	} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("collision error missing %q: %v", part, err)
		}
	}
}
