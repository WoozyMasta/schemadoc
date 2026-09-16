// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"strings"
	"testing"
)

func TestFormatYAMLOutputPreservesJSONNumbers(t *testing.T) {
	t.Parallel()

	content := []byte(`{
  "large": 9007199254740993,
  "decimal": 1.2300,
  "scientific": 1e+03
}`)

	got, err := formatYAMLOutputFromJSON(content, 2)
	if err != nil {
		t.Fatalf("formatYAMLOutputFromJSON: %v", err)
	}

	output := string(got)
	for _, value := range []string{
		"9007199254740993",
		"1.2300",
		"1e+03",
	} {
		if !strings.Contains(output, value) {
			t.Fatalf("YAML output %q does not contain %q", output, value)
		}
	}
	if strings.Contains(output, `"9007199254740993"`) {
		t.Fatalf("large integer was encoded as a YAML string: %q", output)
	}
}
