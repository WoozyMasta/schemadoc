// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"bytes"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func TestFormatDescriptionMarkdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "Go doc root unordered list",
			in:   "Items:\n\n  - one\n  - two",
			want: "Items:\n\n* one\n* two",
		},
		{
			name: "nested unordered list",
			in:   "* parent\n  * child",
			want: "* parent\n  * child",
		},
		{
			name: "ordered list",
			in:   "1. one\n2. two",
			want: "1. one\n2. two",
		},
		{
			name: "inline Markdown is preserved",
			in:   "Use **strong**, _emphasis_, `inline code`, and [a link](https://example.test).",
			want: "Use **strong**, _emphasis_, `inline code`, and [a link](https://example.test).",
		},
		{
			name: "heading block quote table and HTML are preserved",
			in: "## Details\n\n> Keep this value.\n\n| Name | Value |\n| --- | --- |\n| A | B |\n\n" +
				"<details><summary>More</summary></details>",
			want: "## Details\n\n> Keep this value.\n\n| Name | Value |\n| --- | --- |\n| A | B |\n\n" +
				"<details><summary>More</summary></details>",
		},
		{
			name: "existing backtick fence",
			in:   "Example:\n\n```go\nfmt.Println(\"ok\")\n```",
			want: "Example:\n\n```go\nfmt.Println(\"ok\")\n```",
		},
		{
			name: "existing tilde fence",
			in:   "~~~yaml\nkey: value\n~~~",
			want: "~~~yaml\nkey: value\n~~~",
		},
		{
			name: "four space preformatted block",
			in:   "Example:\n\n    first := 1\n    second := 2",
			want: "Example:\n\n```\nfirst := 1\nsecond := 2\n```",
		},
		{
			name: "two space Go doc preformatted block",
			in: "Example:\n\n  {\n" +
				"    \"x-provider\": \"acme-cloud\"\n" +
				"  }",
			want: "Example:\n\n```\n{\n" +
				"  \"x-provider\": \"acme-cloud\"\n" +
				"}\n```",
		},
		{
			name: "preformatted block preserves blank lines and relative indentation",
			in:   "Example:\n\n  func main() {\n    if enabled {\n      run()\n    }\n\n    cleanup()\n  }",
			want: "Example:\n\n```\nfunc main() {\n  if enabled {\n    run()\n  }\n\n  cleanup()\n}\n```",
		},
		{
			name: "preformatted block chooses a safe fence",
			in:   "Example:\n\n  fmt.Println(\"```\")",
			want: "Example:\n\n````\nfmt.Println(\"```\")\n````",
		},
		{
			name: "paragraph after preformatted block",
			in:   "Example:\n\n  command --flag\n\nContinue here.",
			want: "Example:\n\n```\ncommand --flag\n```\n\nContinue here.",
		},
		{
			name: "preformatted block nested in a list item",
			in:   "Items:\n\n  - Run the command:\n\n    command --flag\n\n  - Check the output.",
			want: "Items:\n\n* Run the command:\n\n  ```\n  command --flag\n  ```\n\n* Check the output.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := formatDescriptionMarkdown(test.in, 80, "*")
			if got != test.want {
				t.Fatalf("formatDescriptionMarkdown() = %q, want %q", got, test.want)
			}
			assertMarkdownParses(t, got)
		})
	}
}

func TestFormatDescriptionMarkdownKeepsCodeBlocksIntact(t *testing.T) {
	t.Parallel()

	input := "Shell:\n\n  #!/bin/sh\n  printf '%s\\n' \"$HOME\"\n\nYAML:\n\n  ---\n  key: value\n\nSQL:\n\n  SELECT id, name\n  FROM users;"
	got := formatDescriptionMarkdown(input, 80, "*")
	assertMarkdownParses(t, got)

	source := []byte(got)
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	var blocks []string
	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if code, ok := node.(*ast.FencedCodeBlock); ok {
				blocks = append(blocks, string(code.Text(source)))
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatalf("walk Markdown AST: %v", err)
	}
	want := []string{"#!/bin/sh\nprintf '%s\\n' \"$HOME\"\n", "---\nkey: value\n", "SELECT id, name\nFROM users;\n"}
	if len(blocks) != len(want) {
		t.Fatalf("fenced code blocks = %d, want %d: %q", len(blocks), len(want), got)
	}
	for index := range want {
		if blocks[index] != want[index] {
			t.Fatalf("code block %d = %q, want %q", index, blocks[index], want[index])
		}
	}
}

func FuzzFormatDescriptionMarkdown(f *testing.F) {
	for _, seed := range []string{
		"Text\n\n  code\n\nMore text.",
		"- root\n  - child\n\n  command --flag",
		"```go\nfmt.Println(\"```\")\n```",
		"~~~yaml\nkey: value\n~~~\n\n> quote",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got := formatDescriptionMarkdown(input, 80, "*")
		assertMarkdownParses(t, got)
	})
}

func assertMarkdownParses(t *testing.T, input string) {
	t.Helper()

	source := []byte(input)
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	if document == nil {
		t.Fatal("Markdown parser returned nil document")
	}
	var rendered bytes.Buffer
	if err := goldmark.Convert(source, &rendered); err != nil {
		t.Fatalf("render Markdown: %v", err)
	}
}

func TestNormalizeMarkdownOutputAddsBlankAfterFence(t *testing.T) {
	t.Parallel()

	got := normalizeMarkdownOutput("~~~yaml\nkey: value\n~~~\n<!-- marker -->")
	want := "~~~yaml\nkey: value\n~~~\n\n<!-- marker -->"
	if got != want {
		t.Fatalf("normalizeMarkdownOutput() = %q, want %q", got, want)
	}
}
