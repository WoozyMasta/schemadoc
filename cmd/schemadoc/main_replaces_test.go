// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReplaceEditArg_ResolvesRelativePath(t *testing.T) {
	t.Parallel()

	moduleRoot := t.TempDir()
	tests := []struct {
		name     string
		newPath  string
		expected string
	}{
		{
			name:    "relative unix-like path",
			newPath: "../dependency",
			expected: "-replace=github.com/example/dependency=" +
				filepath.ToSlash(filepath.Join(moduleRoot, "..", "dependency")),
		},
		{
			name:    "relative windows-like path",
			newPath: "..\\dependency",
			expected: "-replace=github.com/example/dependency=" +
				filepath.ToSlash(filepath.Join(moduleRoot, "..\\dependency")),
		},
	}

	for index := range tests {
		tt := tests[index]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			arg, ok, err := buildReplaceEditArg(
				goModEditReplace{
					Old: goModEditModule{Path: "github.com/example/dependency"},
					New: goModEditModule{Path: tt.newPath},
				},
				moduleRoot,
				"github.com/example/project",
			)
			if err != nil {
				t.Fatalf("buildReplaceEditArg() error=%v, want nil", err)
			}

			if !ok {
				t.Fatal("buildReplaceEditArg() ok=false, want true")
			}

			if arg != tt.expected {
				t.Fatalf("arg=%q, want %q", arg, tt.expected)
			}
		})
	}
}

func TestBuildReplaceEditArg_SkipsTargetModule(t *testing.T) {
	t.Parallel()

	arg, ok, err := buildReplaceEditArg(
		goModEditReplace{
			Old: goModEditModule{Path: "github.com/example/project"},
			New: goModEditModule{Path: "../warpbox"},
		},
		".",
		"github.com/example/project",
	)
	if err != nil {
		t.Fatalf("buildReplaceEditArg() error=%v, want nil", err)
	}

	if ok || arg != "" {
		t.Fatalf("arg=%q ok=%v, want empty/false", arg, ok)
	}
}

func TestBuildReplaceEditArg_UsesModuleVersionReplacement(t *testing.T) {
	t.Parallel()

	arg, ok, err := buildReplaceEditArg(
		goModEditReplace{
			Old: goModEditModule{
				Path:    "example.com/a",
				Version: "v1.2.0",
			},
			New: goModEditModule{
				Path:    "example.com/b",
				Version: "v1.3.0",
			},
		},
		".",
		"example.com/target",
	)
	if err != nil {
		t.Fatalf("buildReplaceEditArg() error=%v, want nil", err)
	}

	if !ok {
		t.Fatal("buildReplaceEditArg() ok=false, want true")
	}

	if !strings.Contains(arg, "-replace=example.com/a@v1.2.0=example.com/b@v1.3.0") {
		t.Fatalf("arg=%q, want module-version replacement", arg)
	}
}

func TestIsRelativeOrAbsolutePath_PathStyles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "relative unix style", value: "../module", want: true},
		{name: "relative windows style", value: "..\\module", want: true},
		{name: "import path", value: "github.com/example/module", want: false},
		{name: "empty", value: "", want: false},
	}

	for index := range tests {
		tt := tests[index]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isRelativeOrAbsolutePath(tt.value)
			if got != tt.want {
				t.Fatalf("isRelativeOrAbsolutePath(%q)=%v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
