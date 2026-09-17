// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseDocumentPreservesJSONNumbers(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
  "type": "integer",
  "minimum": 9007199254740993,
  "maximum": 9007199254740994,
  "examples": [9007199254740992, 9223372036854775807, -9223372036854775808, 1.2300, 1e+03]
}`)

	document, err := parseDocument(schema)
	if err != nil {
		t.Fatalf("parseDocument: %v", err)
	}

	object := document.Root.Object
	checks := map[string]string{
		"minimum": "9007199254740993",
		"maximum": "9007199254740994",
	}
	for key, want := range checks {
		value, ok := object[key].(json.Number)
		if !ok {
			t.Fatalf("%s type = %T, want json.Number", key, object[key])
		}
		if value.String() != want {
			t.Fatalf("%s = %q, want %q", key, value, want)
		}
	}

	encoded, err := json.Marshal(document.Raw)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	for _, value := range []string{
		"9007199254740993",
		"9007199254740994",
		"9223372036854775807",
		"-9223372036854775808",
		"1.2300",
		"1e+03",
	} {
		if !strings.Contains(string(encoded), value) {
			t.Fatalf("encoded schema %q does not contain %q", encoded, value)
		}
	}
}

func TestJSONContainsNumberIgnoresStringContent(t *testing.T) {
	t.Parallel()

	if jsonContainsNumber([]byte(`{"message":"retry 3 times"}`)) {
		t.Fatal("jsonContainsNumber reported a number inside a string")
	}
	if !jsonContainsNumber([]byte(`{"minimum":3}`)) {
		t.Fatal("jsonContainsNumber missed a JSON number")
	}
}
