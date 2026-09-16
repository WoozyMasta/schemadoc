// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"errors"
	"testing"
)

func TestSchemaPathKeepsAmbiguousNamesDistinct(t *testing.T) {
	t.Parallel()

	dotted := schemaPath{}.appendProperty("foo").appendProperty("bar")
	literal := schemaPath{}.appendProperty("foo.bar")
	arrayItem := schemaPath{}.appendProperty("foo").appendArrayItem()
	literalArray := schemaPath{}.appendProperty("foo").appendProperty("[]")

	if dotted.equal(literal) {
		t.Fatal("dotted and literal property paths must have different identities")
	}
	if arrayItem.equal(literalArray) {
		t.Fatal("array item and literal [] property paths must have different identities")
	}
	if got, want := literal.display(), `["foo.bar"]`; got != want {
		t.Fatalf("literal path display = %q, want %q", got, want)
	}
}

func TestLocalSchemaResolverEscapesAndDefinitionMaps(t *testing.T) {
	t.Parallel()

	root := map[string]any{
		"$defs": map[string]any{
			"a/b~c": map[string]any{"type": "string"},
		},
		"definitions": map[string]any{
			"legacy": map[string]any{"type": "integer"},
		},
	}
	resolver := newLocalSchemaResolver(root)

	for _, test := range []struct {
		name string
		ref  string
		want string
	}{
		{name: "modern", ref: "#/$defs/a~1b~0c", want: "string"},
		{name: "legacy", ref: "#/definitions/legacy", want: "integer"},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, err := resolver.lookup(test.ref)
			if err != nil {
				t.Fatalf("lookup(%q): %v", test.ref, err)
			}
			if got := asString(value.Object["type"]); got != test.want {
				t.Fatalf("resolved type = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLocalSchemaResolverErrors(t *testing.T) {
	t.Parallel()

	resolver := newLocalSchemaResolver(map[string]any{
		"$defs": map[string]any{
			"cycleA": map[string]any{"$ref": "#/$defs/cycleB"},
			"cycleB": map[string]any{"$ref": "#/$defs/cycleA"},
		},
	})

	for _, test := range []struct {
		name string
		ref  string
		want error
	}{
		{name: "external", ref: "other.json#/x", want: ErrExternalSchemaReference},
		{name: "invalid escape", ref: "#/$defs/cycle~2", want: ErrInvalidSchemaPointer},
		{name: "missing", ref: "#/$defs/missing", want: ErrUnresolvedSchemaReference},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := resolver.lookup(test.ref)
			if !errors.Is(err, test.want) {
				t.Fatalf("lookup(%q) error = %v, want errors.Is(..., %v)", test.ref, err, test.want)
			}
		})
	}

}

func TestDetectDraftMapsDraft5ToDraft4Compatible(t *testing.T) {
	t.Parallel()

	got := DetectDraft("http://json-schema.org/draft-05/schema")
	if !got.Supported || got.Canonical != "draft-05" {
		t.Fatalf("draft 5 info = %+v, want supported draft-05", got)
	}

	doc, err := parseDocument([]byte(`{"$schema":"draft-05","type":"object"}`))
	if err != nil {
		t.Fatalf("parse draft 5 schema: %v", err)
	}
	if doc.Dialect != schemaDialectDraft4Compatible {
		t.Fatalf("draft 5 dialect = %q, want %q", doc.Dialect, schemaDialectDraft4Compatible)
	}

	for _, input := range []string{
		"draft-04",
		"draft-06",
		"draft-07",
		"2019-09",
		"2020-12",
	} {
		if info := DetectDraft(input); !info.Supported {
			t.Fatalf("draft %q should be supported: %+v", input, info)
		}
	}
}

func FuzzLocalSchemaResolver(f *testing.F) {
	f.Add("#/$defs/value")
	f.Add("#/$defs/a~1b~0c")
	f.Add("other.json#/value")

	f.Fuzz(func(t *testing.T, ref string) {
		resolver := newLocalSchemaResolver(map[string]any{
			"$defs": map[string]any{
				"value": map[string]any{"type": "string"},
				"a/b~c": map[string]any{"type": "number"},
			},
		})
		_, _ = resolver.lookup(ref)
	})
}
