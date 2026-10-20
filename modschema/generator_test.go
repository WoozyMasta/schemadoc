// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGeneratorReuse(t *testing.T) {
	t.Parallel()

	moduleDir := newGeneratorTestModule(t)
	generator := NewGenerator()
	t.Cleanup(func() {
		if err := generator.Close(); err != nil {
			t.Error(err)
		}
	})

	options := Options{Module: moduleDir, Type: "Config"}
	raw, source, err := generator.Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	if source != "module:example.com/reuse.Config" || len(generator.workspaces) != 1 {
		t.Fatalf("source=%q, workspaces=%d", source, len(generator.workspaces))
	}

	var workspace *schemaWorkspace

	for _, prepared := range generator.workspaces {
		workspace = prepared
	}

	goMod, err := os.Stat(filepath.Join(workspace.dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	options.Module += string(os.PathSeparator) + "."
	options.Type = "Other"
	options.Comments = CommentModePlain
	options.KeyNamer = "snake"
	options.RootID = "urn:example:other"
	other, source, err := generator.Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	if source != "module:example.com/reuse.Other" {
		t.Fatalf("source=%q", source)
	}

	var document map[string]any

	if err := json.Unmarshal(other, &document); err != nil {
		t.Fatal(err)
	}

	if document["$ref"] != "#/$defs/Other" || document["$id"] != options.RootID {
		t.Fatalf("unexpected root: %s", other)
	}

	if !bytes.Contains(other, []byte(`"some_value"`)) || bytes.Contains(other, []byte(`"Config"`)) {
		t.Fatalf("stale type or key namer: %s", other)
	}

	options.Type = "Config"
	options.KeyNamer = "none"
	options.RootID = ""
	plain, _, err := generator.Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(plain, []byte(`"description": "Text is physically wrapped.\nSecond paragraph."`)) {
		t.Fatalf("plain comments not applied: %s", plain)
	}

	options.Type = "Missing"

	if _, _, err := generator.Generate(options); err == nil {
		t.Fatal("missing type accepted")
	}

	options.Type = "Config"
	options.Comments = CommentModeRaw
	repeated, _, err := generator.Generate(options)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(raw, repeated) {
		t.Fatal("raw schema changed after other requests or failed execution")
	}

	updatedGoMod, err := os.Stat(filepath.Join(workspace.dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	if len(generator.workspaces) != 1 || !os.SameFile(goMod, updatedGoMod) || !goMod.ModTime().Equal(updatedGoMod.ModTime()) {
		t.Fatal("compatible requests repeated workspace setup")
	}

	if err := generator.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(workspace.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace remains after Close: %v", err)
	}

	if err := generator.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	if _, _, err := generator.Generate(options); !errors.Is(err, ErrGeneratorClosed) {
		t.Fatalf("Generate after Close: %v", err)
	}

	if _, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err != nil {
		t.Fatalf("source module affected by cleanup: %v", err)
	}
}

func TestGeneratorWorkspaceIsolation(t *testing.T) {
	t.Parallel()

	moduleDir := newGeneratorTestModule(t)
	var generator Generator
	t.Cleanup(func() {
		if err := generator.Close(); err != nil {
			t.Error(err)
		}
	})

	options := Options{Module: moduleDir, Type: "Config"}

	if _, _, err := generator.Generate(options); err != nil {
		t.Fatal(err)
	}

	var dependencyDir string

	for _, workspace := range generator.workspaces {
		var err error
		dependencyDir, err = moduleDirFromHelperWorkspace(workspace.dir, schemaGeneratorJSONSchemaModule)
		if err != nil {
			t.Fatal(err)
		}
	}

	options.Package = "example.com/reuse/nested"

	if _, _, err := generator.Generate(options); err != nil {
		t.Fatal(err)
	}

	if len(generator.workspaces) != 2 {
		t.Fatalf("different package reused workspace: %d", len(generator.workspaces))
	}

	otherModuleDir := newGeneratorTestModule(t)
	goMod := "module example.com/reuse\n\ngo 1.25.0\n\nreplace " + schemaGeneratorJSONSchemaModule +
		" => " + strconv.Quote(dependencyDir) + "\n"
	writeGeneratorTestFile(t, filepath.Join(otherModuleDir, "go.mod"), goMod)
	options.Module = otherModuleDir
	options.Package = ""

	if _, _, err := generator.Generate(options); err != nil {
		t.Fatal(err)
	}

	options.JSONSchemaVersion = "v0.0.0" // Replaced locally; no new dependency download.

	if _, _, err := generator.Generate(options); err != nil {
		t.Fatal(err)
	}

	if len(generator.workspaces) != 4 {
		t.Fatalf("different module or dependency version reused workspace: %d", len(generator.workspaces))
	}

	options.JSONSchemaVersion = schemaGeneratorJSONSchemaVersion

	if _, _, err := generator.Generate(options); err != nil {
		t.Fatal(err)
	}

	if len(generator.workspaces) != 4 {
		t.Fatal("explicit default version did not reuse its prepared workspace")
	}

	dirs := make([]string, 0, len(generator.workspaces))

	for _, workspace := range generator.workspaces {
		dirs = append(dirs, workspace.dir)
	}

	if err := generator.Close(); err != nil {
		t.Fatal(err)
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("workspace remains after Close: %s: %v", dir, err)
		}
	}

	if _, err := os.Stat(filepath.Join(dependencyDir, "go.mod")); err != nil {
		t.Fatalf("dependency cache affected by cleanup: %v", err)
	}
}

func TestGeneratorPreparationFailureCleanup(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMP", tempRoot)
	t.Setenv("TEMP", tempRoot)
	t.Setenv("TMPDIR", tempRoot)
	moduleDir := newGeneratorTestModule(t)
	var generator Generator
	t.Cleanup(func() {
		if err := generator.Close(); err != nil {
			t.Error(err)
		}
	})

	_, _, err := generator.Generate(Options{Module: moduleDir, Package: "example.com/reuse/main", Type: "Config"})
	if !errors.Is(err, ErrMainPackageUnsupported) {
		t.Fatalf("package main error = %v", err)
	}

	writeGeneratorTestFile(t, filepath.Join(moduleDir, "go.mod"), "module example.com/reuse\ninvalid directive\n")

	if _, _, err := generator.Generate(Options{Module: moduleDir, Type: "Config"}); err == nil {
		t.Fatal("invalid source go.mod accepted")
	}

	leaked, err := filepath.Glob(filepath.Join(tempRoot, "schemadoc-mod2schema-*"))
	if err != nil {
		t.Fatal(err)
	}

	if len(leaked) != 0 || len(generator.workspaces) != 0 {
		t.Fatalf("failed preparation leaked workspaces: %v", leaked)
	}
}

func TestToolchainVersionCache(t *testing.T) {
	t.Parallel()

	var cache toolchainVersionCache
	calls := 0
	probe := func() (string, error) {
		calls++

		if calls == 1 {
			return "", os.ErrPermission
		}

		return "go1.27.2", nil
	}

	if _, err := cache.load(probe); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("probe error = %v", err)
	}

	for range 3 {
		version, err := cache.load(probe)
		if err != nil || version != "go1.27.2" {
			t.Fatalf("version=%q, error=%v", version, err)
		}
	}

	if calls != 2 {
		t.Fatalf("probe calls=%d, want one failure and one success", calls)
	}
}

func TestGoVersionProbeWithoutTemporaryDirectory(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMP", tempRoot)
	t.Setenv("TEMP", tempRoot)
	t.Setenv("TMPDIR", tempRoot)
	t.Setenv("GOTOOLCHAIN", "go1.999.0")
	t.Setenv("GOWORK", filepath.Join(tempRoot, "go.work"))
	writeGeneratorTestFile(t, filepath.Join(tempRoot, "go.mod"), "module example.com/unrelated\n\ngo 1.999.0\n")
	writeGeneratorTestFile(t, filepath.Join(tempRoot, "go.work"), "invalid workspace\n")

	for range 2 {
		version, err := probeGoToolchainVersion()
		if err != nil || !strings.HasPrefix(version, "go1.") {
			t.Fatalf("local version=%q, error=%v", version, err)
		}
	}

	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("version probe created temporary files: %v", entries)
	}
}

// newGeneratorTestModule creates a local fixture with two types and two packages.
func newGeneratorTestModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeGeneratorTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/reuse\n\ngo 1.25.0\n")
	writeGeneratorTestFile(t, filepath.Join(dir, "config.go"), `package reuse

// Config is the first root.
type Config struct {
	// Text is physically
	// wrapped.
	//
	// Second paragraph.
	Text string
}

// Other is the second root.
type Other struct {
	SomeValue int
}
`)
	writeGeneratorTestFile(t, filepath.Join(dir, "nested", "config.go"), "package nested\n\ntype Config struct { Nested bool }\n")
	writeGeneratorTestFile(t, filepath.Join(dir, "main", "main.go"), "package main\n\ntype Config struct{}\n\nfunc main() {}\n")
	return dir
}

// writeGeneratorTestFile writes fixture sources and creates their parent directories.
func writeGeneratorTestFile(t *testing.T, path, text string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
