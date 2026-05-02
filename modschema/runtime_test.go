// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testModulePath = "github.com/woozymasta/schemadoc"

func TestNormalizeOptions_Defaults(t *testing.T) {
	t.Parallel()

	options := NormalizeOptions(Options{})
	if options.Module != "." {
		t.Fatalf("Module=%q, want %q", options.Module, ".")
	}

	if options.KeyNamer != "none" {
		t.Fatalf("KeyNamer=%q, want %q", options.KeyNamer, "none")
	}
}

func TestResolveJSONSchemaVersion(t *testing.T) {
	t.Parallel()

	version, hasCustom := resolveJSONSchemaVersion("")
	if hasCustom {
		t.Fatalf("hasCustom=%t, want false", hasCustom)
	}

	if version != schemaGeneratorJSONSchemaVersion {
		t.Fatalf(
			"version=%q, want %q",
			version,
			schemaGeneratorJSONSchemaVersion,
		)
	}

	version, hasCustom = resolveJSONSchemaVersion("v9.9.9")
	if !hasCustom {
		t.Fatalf("hasCustom=%t, want true", hasCustom)
	}

	if version != "v9.9.9" {
		t.Fatalf("version=%q, want %q", version, "v9.9.9")
	}
}

func TestParseGoMajorMinor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantErr   bool
	}{
		{
			name:      "release patch",
			input:     "go1.26.1",
			wantMajor: 1,
			wantMinor: 26,
		},
		{
			name:      "release candidate",
			input:     "go1.24rc1",
			wantMajor: 1,
			wantMinor: 24,
		},
		{
			name:    "invalid format",
			input:   "devel",
			wantErr: true,
		},
	}

	for index := range testCases {
		testCase := testCases[index]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			major, minor, err := parseGoMajorMinor(testCase.input)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("parseGoMajorMinor(%q) error = nil, want error", testCase.input)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseGoMajorMinor(%q) error = %v", testCase.input, err)
			}

			if major != testCase.wantMajor || minor != testCase.wantMinor {
				t.Fatalf(
					"parseGoMajorMinor(%q) = (%d, %d), want (%d, %d)",
					testCase.input,
					major,
					minor,
					testCase.wantMajor,
					testCase.wantMinor,
				)
			}
		})
	}
}

func TestResolveTarget_Local(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(moduleDir, "go.mod"),
		[]byte("module github.com/acme/local\n\ngo 1.25.5\n"),
		0o600,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	target, err := resolveTarget(Options{
		Module: moduleDir,
		Type:   "Config",
	})
	if err != nil {
		t.Fatalf("resolveTarget() error = %v", err)
	}

	if target.Source != moduleSourceLocal {
		t.Fatalf("Source=%q, want %q", target.Source, moduleSourceLocal)
	}

	if target.ModulePath != "github.com/acme/local" {
		t.Fatalf("ModulePath=%q, want %q", target.ModulePath, "github.com/acme/local")
	}

	if target.PackagePath != "github.com/acme/local" {
		t.Fatalf("PackagePath=%q, want %q", target.PackagePath, "github.com/acme/local")
	}
}

func TestResolveTarget_RemoteValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		options     Options
		wantErrPart string
		wantSource  string
	}{
		{
			name: "remote no version",
			options: Options{
				Module: "github.com/acme/project",
				Type:   "Config",
			},
			wantErrPart: "must include explicit version",
		},
		{
			name: "invalid module token",
			options: Options{
				Module: "mymod",
				Type:   "Config",
			},
			wantErrPart: "not found on disk and is not valid remote module reference",
		},
		{
			name: "versioned remote",
			options: Options{
				Module: "github.com/acme/project@v1.2.3",
				Type:   "Config",
			},
			wantSource: moduleSourceRemote,
		},
	}

	for index := range testCases {
		testCase := testCases[index]
		t.Run(testCase.name, func(t *testing.T) {
			target, err := resolveTarget(testCase.options)
			if testCase.wantErrPart != "" {
				if err == nil {
					t.Fatalf("resolveTarget() error = nil, want contains %q", testCase.wantErrPart)
				}

				if !strings.Contains(err.Error(), testCase.wantErrPart) {
					t.Fatalf("resolveTarget() error = %q, want contains %q", err.Error(), testCase.wantErrPart)
				}

				return
			}

			if err != nil {
				t.Fatalf("resolveTarget() error = %v", err)
			}

			if target.Source != testCase.wantSource {
				t.Fatalf("Source=%q, want %q", target.Source, testCase.wantSource)
			}
		})
	}
}

func TestBuildProgramSource(t *testing.T) {
	t.Parallel()

	source, err := BuildProgramSource(Options{
		Module:   testModulePath,
		Type:     "SchemaModel",
		Package:  testModulePath,
		KeyNamer: "snake",
	})
	if err != nil {
		t.Fatalf("BuildProgramSource() error = %v", err)
	}

	if !strings.Contains(source, `applyKeyNamer(reflector, "snake")`) {
		t.Fatalf("generated source does not contain key namer setup")
	}

	if !strings.Contains(source, `case "snake":`) {
		t.Fatalf("generated source does not contain snake key namer case")
	}
}

func TestGenerate_RejectsMainPackage(t *testing.T) {
	t.Parallel()

	moduleDir := t.TempDir()
	mainSource := `package main

type Config struct {
	Name string ` + "`json:\"name\"`" + `
}
`
	if err := os.WriteFile(
		filepath.Join(moduleDir, "go.mod"),
		[]byte("module github.com/acme/mainapp\n\ngo 1.25.5\n"),
		0o600,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(moduleDir, "main.go"),
		[]byte(mainSource),
		0o600,
	); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	_, _, err := Generate(Options{
		Module: moduleDir,
		Type:   "Config",
	})
	if err == nil {
		t.Fatal("Generate() error = nil, want rejection for package main")
	}

	if !errors.Is(err, ErrMainPackageUnsupported) {
		t.Fatalf("Generate() error = %v, want errors.Is(..., ErrMainPackageUnsupported)", err)
	}
}
