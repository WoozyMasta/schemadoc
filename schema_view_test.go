// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"errors"
	"reflect"
	"testing"
)

func TestEffectiveSchemaAppliesModernRefSiblings(t *testing.T) {
	t.Parallel()

	doc := parseTestSchemaDocument(t, `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$ref": "#/$defs/Base",
		"title": "overlay title",
		"description": "overlay description",
		"default": "overlay default",
		"examples": ["overlay example"],
		"x-order": 2,
		"properties": {"overlay": {"type": "boolean"}},
		"$defs": {
			"Base": {
				"type": "object",
				"title": "base title",
				"description": "base description",
				"default": "base default",
				"examples": ["base example"],
				"x-order": 1,
				"properties": {"base": {"type": "string"}},
				"required": ["base"]
			}
		}
	}`)
	view := newSchemaSemanticView(doc)
	effective, err := view.effective(doc.Root)
	if err != nil {
		t.Fatalf("effective: %v", err)
	}

	properties, err := effective.properties(&view)
	if err != nil {
		t.Fatalf("properties: %v", err)
	}
	if len(properties) != 2 {
		t.Fatalf("properties = %v, want base and overlay", properties)
	}
	for _, test := range []struct {
		keyword string
		want    any
	}{
		{keyword: "title", want: "overlay title"},
		{keyword: "description", want: "overlay description"},
		{keyword: "default", want: "overlay default"},
		{keyword: "examples", want: []any{"overlay example"}},
		{keyword: "x-order", want: float64(2)},
	} {
		got, ok := effective.annotation(test.keyword)
		if !ok || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s = %v, want %v", test.keyword, got, test.want)
		}
	}
	if got := effective.required(); len(got) != 1 || got[0] != "base" {
		t.Fatalf("required = %v, want [base]", got)
	}
}

func TestEffectiveSchemaIgnoresOldRefSiblings(t *testing.T) {
	t.Parallel()

	doc := parseTestSchemaDocument(t, `{
		"$schema": "https://json-schema.org/draft-07/schema",
		"$ref": "#/$defs/Base",
		"description": "ignored description",
		"properties": {"overlay": {"type": "boolean"}},
		"$defs": {"Base": {
			"type": "object",
			"description": "base description",
			"properties": {"base": {"type": "string"}}
		}}
	}`)
	view := newSchemaSemanticView(doc)
	effective, err := view.effective(doc.Root)
	if err != nil {
		t.Fatalf("effective: %v", err)
	}

	properties, err := effective.properties(&view)
	if err != nil {
		t.Fatalf("properties: %v", err)
	}
	if len(properties) != 1 {
		t.Fatalf("properties = %v, want only base", properties)
	}
	if got, _ := effective.annotation("description"); got != "base description" {
		t.Fatalf("description = %v, want base description", got)
	}
}

func TestEffectiveSchemaRefOverlayKeepsBaseMetadata(t *testing.T) {
	t.Parallel()

	doc := parseTestSchemaDocument(t, `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$ref": "#/$defs/Base",
		"properties": {"overlay": {"type": "boolean"}},
		"$defs": {"Base": {
			"type": "object",
			"title": "base title",
			"description": "base description",
			"default": "base default",
			"examples": ["base example"],
			"x-order": 1,
			"properties": {"base": {"type": "string"}},
			"required": ["base"]
		}}
	}`)
	view := newSchemaSemanticView(doc)
	effective, err := view.effective(doc.Root)
	if err != nil {
		t.Fatalf("effective: %v", err)
	}

	properties, err := effective.properties(&view)
	if err != nil {
		t.Fatalf("properties: %v", err)
	}
	if len(properties) != 2 {
		t.Fatalf("properties = %v, want base and overlay", properties)
	}
	for _, test := range []struct {
		keyword string
		want    any
	}{
		{keyword: "title", want: "base title"},
		{keyword: "description", want: "base description"},
		{keyword: "default", want: "base default"},
		{keyword: "examples", want: []any{"base example"}},
		{keyword: "x-order", want: float64(1)},
	} {
		got, ok := effective.annotation(test.keyword)
		if !ok || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s = %v, want %v", test.keyword, got, test.want)
		}
	}
	if got := effective.required(); len(got) != 1 || got[0] != "base" {
		t.Fatalf("required = %v, want [base]", got)
	}
}

func TestEffectiveSchemaKeepsCompositionAndConstraintTerms(t *testing.T) {
	t.Parallel()

	doc := parseTestSchemaDocument(t, `{
		"allOf": [
			{
				"type": "object",
				"properties": {
					"same": {"type": "string"},
					"left": {"type": "string"}
				},
				"required": ["same", "left"]
			},
			{
				"properties": {
					"same": {"minLength": 3},
					"right": {"type": "integer"}
				},
				"required": ["same", "right"]
			}
		],
		"anyOf": [{"type": "string"}, {"type": "integer"}],
		"oneOf": [{"const": "a"}, {"const": "b"}]
	}`)
	view := newSchemaSemanticView(doc)
	effective, err := view.effective(doc.Root)
	if err != nil {
		t.Fatalf("effective: %v", err)
	}

	properties, err := effective.properties(&view)
	if err != nil {
		t.Fatalf("properties: %v", err)
	}
	if len(properties) != 3 {
		t.Fatalf("properties = %v, want left/right/same", properties)
	}
	if same := properties["same"]; len(same.terms) != 2 {
		t.Fatalf("same terms = %d, want 2", len(same.terms))
	}
	if got := effective.required(); len(got) != 3 || got[0] != "same" || got[1] != "left" || got[2] != "right" {
		t.Fatalf("required = %v, want stable union", got)
	}
	if got := effective.compositions("anyOf"); len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("anyOf branches = %v, want one group of two", got)
	}
	if got := effective.compositions("oneOf"); len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("oneOf branches = %v, want one group of two", got)
	}
}

func TestEffectiveSchemaExposesDialectSpecificArrayItems(t *testing.T) {
	t.Parallel()

	legacyDoc := parseTestSchemaDocument(t, `{
		"$schema": "http://json-schema.org/draft-07/schema",
		"items": [{"type": "string"}, {"type": "integer"}],
		"additionalItems": false
	}`)
	legacyView := newSchemaSemanticView(legacyDoc)
	legacy, err := legacyView.effective(legacyDoc.Root)
	if err != nil {
		t.Fatalf("legacy effective: %v", err)
	}
	legacyItems, err := legacy.arrayItems(&legacyView)
	if err != nil {
		t.Fatalf("legacy items: %v", err)
	}
	if len(legacyItems) != 1 || !legacyView.legacyArraySemantics() || legacyItems[0].PrefixItems == nil || len(*legacyItems[0].PrefixItems) != 2 {
		t.Fatalf("legacy items = %+v", legacyItems)
	}

	modernDoc := parseTestSchemaDocument(t, `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"prefixItems": [{"type": "string"}],
		"items": {"type": "integer"}
	}`)
	modernView := newSchemaSemanticView(modernDoc)
	modern, err := modernView.effective(modernDoc.Root)
	if err != nil {
		t.Fatalf("modern effective: %v", err)
	}
	modernItems, err := modern.arrayItems(&modernView)
	if err != nil {
		t.Fatalf("modern items: %v", err)
	}
	if len(modernItems) != 1 || modernView.legacyArraySemantics() || modernItems[0].PrefixItems == nil || len(*modernItems[0].PrefixItems) != 1 || modernItems[0].Items == nil {
		t.Fatalf("modern items = %+v", modernItems)
	}
}

func TestEffectiveSchemaExposesObjectKeywords(t *testing.T) {
	t.Parallel()

	doc := parseTestSchemaDocument(t, `{
		"type": ["object", "null"],
		"const": {"kind": "fixed"},
		"enum": [{"kind": "fixed"}, {"kind": "other"}],
		"additionalProperties": false,
		"patternProperties": {"^x-": {"type": "string"}},
		"propertyNames": {"pattern": "^[a-z-]+$"}
	}`)
	view := newSchemaSemanticView(doc)
	effective, err := view.effective(doc.Root)
	if err != nil {
		t.Fatalf("effective: %v", err)
	}

	if got := effective.types(); len(got) != 1 {
		t.Fatalf("types = %v, want one type constraint", got)
	}
	if got := effective.consts(); len(got) != 1 || got[0] == nil {
		t.Fatalf("consts = %v, want one const constraint", got)
	}
	if got := effective.enums(); len(got) != 1 || got[0] == nil {
		t.Fatalf("enums = %v, want one enum constraint", got)
	}
	additionalProperties, err := effective.additionalProperties(&view)
	if err != nil || len(additionalProperties) != 1 || additionalProperties[0].terms[0].Bool == nil {
		t.Fatalf("additionalProperties = %v, err=%v, want false schema", additionalProperties, err)
	}
	propertyNames, err := effective.propertyNames(&view)
	if err != nil || len(propertyNames) != 1 {
		t.Fatalf("propertyNames = %v, err=%v, want one schema", propertyNames, err)
	}
	patternProperties, err := effective.patternProperties(&view)
	if err != nil || len(patternProperties) != 1 || len(patternProperties[0]) != 1 {
		t.Fatalf("patternProperties = %v, err=%v, want one pattern", patternProperties, err)
	}
}

func TestEffectiveSchemaRejectsReferenceCycles(t *testing.T) {
	t.Parallel()

	doc := parseTestSchemaDocument(t, `{
		"$ref": "#/$defs/a",
		"$defs": {
			"a": {"$ref": "#/$defs/b"},
			"b": {"$ref": "#/$defs/a"}
		}
	}`)
	view := newSchemaSemanticView(doc)
	_, err := view.effective(doc.Root)
	if !errors.Is(err, ErrSchemaReferenceCycle) {
		t.Fatalf("cycle error = %v, want errors.Is(..., %v)", err, ErrSchemaReferenceCycle)
	}
}

func parseTestSchemaDocument(t *testing.T, source string) schemaDocument {
	t.Helper()

	doc, err := parseDocument([]byte(source))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	return doc
}
