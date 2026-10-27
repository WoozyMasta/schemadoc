// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestCLI_ExampleCompositionModes(t *testing.T) {
	t.Parallel()
	path := repoTestdataPath(t, "cases", "positive", "composition", "common-object-one-of.json")
	schema, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var raw any
	if err := json.Unmarshal(schema, &raw); err != nil {
		t.Fatal(err)
	}

	compiler := jsonschema.NewCompiler()
	const url = "urn:schemadoc:cli-example-test"
	if err := compiler.AddResource(url, raw); err != nil {
		t.Fatal(err)
	}

	validator, err := compiler.Compile(url)
	if err != nil {
		t.Fatal(err)
	}

	for _, mode := range []string{"required", "all"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run([]string{"schema2json", "--mode", mode, path}, &stdout, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, &stderr)
			}

			var value map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			if err := validator.Validate(value); err != nil {
				t.Fatal(err)
			}

			_, remove := value["remove"]
			_, encrypt := value["encrypt"]
			_, name := value["name"]
			if remove == encrypt || name != (mode == "all") {
				t.Fatalf("unexpected fields: %s", &stdout)
			}
		})
	}
}
