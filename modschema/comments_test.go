// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCommentModeValidation(t *testing.T) {
	t.Parallel()

	if got := NormalizeOptions(Options{}).Comments; got != CommentModeRaw {
		t.Fatalf("default comments = %q, want raw", got)
	}

	for _, mode := range []CommentMode{"invalid", "RAW", " plain "} {
		options := Options{Module: "missing-module", Comments: mode}
		_, _, err := Generate(options)

		if !errors.Is(err, ErrInvalidCommentMode) {
			t.Fatalf("Generate(%q) error = %v, want ErrInvalidCommentMode", mode, err)
		}

		if _, err := BuildProgramSource(options); !errors.Is(err, ErrInvalidCommentMode) {
			t.Fatalf("BuildProgramSource(%q) error = %v, want ErrInvalidCommentMode", mode, err)
		}
	}
}

func TestGenerate_CommentModes(t *testing.T) {
	t.Parallel()

	moduleDir, err := filepath.Abs(filepath.Join("testdata", "comments"))
	if err != nil {
		t.Fatal(err)
	}

	options := Options{Module: moduleDir, Type: "Config"}
	defaultSchema, _, err := Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	options.Comments = CommentModeRaw
	rawSchema, _, err := Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(defaultSchema, rawSchema) {
		t.Fatal("default and explicit raw schemas differ")
	}

	options.Comments = CommentModePlain
	plainSchema, _, err := Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	var schema struct {
		Defs map[string]struct {
			Description string `json:"description"`
			Properties  map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		} `json:"$defs"`
	}

	if err := json.Unmarshal(plainSchema, &schema); err != nil {
		t.Fatal(err)
	}

	config := schema.Defs["Config"]
	want := "Text spans physically wrapped source lines with SomeType and SomeType.Method.\n" +
		"See the guide (https://example.com/guide) and https://example.com/auto.\n" +
		"Allowed usage patterns:\nSupported values:\n" +
		"- First item wrapped onto a second line.\n- Second item.\n" +
		"Steps:\n3. First step.\n4. Second step.\nExample:\n    first()\n    second()"

	if got := config.Properties["text"].Description; got != want {
		t.Fatalf("plain description = %q, want %q", got, want)
	}

	if got := config.Properties["tagged"].Description; got != "Keep [SomeType] and  double spaces." {
		t.Fatalf("struct tag description changed: %q", got)
	}

	if config.Description != "Config describes a comment normalization fixture." {
		t.Fatalf("type synopsis changed: %q", config.Description)
	}
}

func TestGeneratedCommentRenderer(t *testing.T) {
	t.Parallel()

	moduleDir, err := filepath.Abs(filepath.Join("testdata", "comments"))
	if err != nil {
		t.Fatal(err)
	}

	options := NormalizeOptions(Options{Module: moduleDir, Type: "Config"})
	target, err := resolveTarget(options)
	if err != nil {
		t.Fatal(err)
	}

	helperDir := t.TempDir()
	if err := initSchemaGeneratorWorkspace(context.Background(), helperDir, target, schemaGeneratorJSONSchemaVersion); err != nil {
		t.Fatal(err)
	}

	options.Package = target.PackagePath
	source, err := BuildProgramSource(options)
	if err != nil {
		t.Fatal(err)
	}

	if err := writeSchemaGeneratorProgram(helperDir, source); err != nil {
		t.Fatal(err)
	}

	tests, err := os.ReadFile(filepath.Join("testdata", "comments", "helper_test.go.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(helperDir, "main_test.go"), tests, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := prepareSchemaGeneratorDependencies(context.Background(), helperDir); err != nil {
		t.Fatal(err)
	}

	if err := runGoCommand(context.Background(), helperDir, "test", "./..."); err != nil {
		t.Fatal(err)
	}
}
