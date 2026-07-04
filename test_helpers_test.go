// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func minimalSchemaBytes(t *testing.T, doc map[string]any) []byte {
	t.Helper()

	if _, ok := doc["$schema"]; !ok {
		doc["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	}

	if _, ok := doc["$id"]; !ok {
		doc["$id"] = "urn:test"
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal schema fixture: %v", err)
	}

	return data
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if !strings.Contains(haystack, needle) {
		t.Fatalf("missing substring %q in:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if strings.Contains(haystack, needle) {
		t.Fatalf("unexpected substring %q in:\n%s", needle, haystack)
	}
}
