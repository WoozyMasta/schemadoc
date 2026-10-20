// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/woozymasta/schemadoc/modschema"
)

func ExampleGenerate() {
	schema, source, err := modschema.Generate(modschema.Options{
		Module: "testdata/comments", Type: "Config",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(source)
	fmt.Println(json.Valid(schema))
	// Output:
	// module:example.com/comments.Config
	// true
}

func ExampleGenerator_Generate() {
	generator := modschema.NewGenerator()
	defer func() {
		if err := generator.Close(); err != nil {
			panic(err)
		}
	}()

	for _, comments := range []modschema.CommentMode{modschema.CommentModeRaw, modschema.CommentModePlain} {
		schema, _, err := generator.Generate(modschema.Options{
			Module: "testdata/comments", Type: "Config", Comments: comments,
		})
		if err != nil {
			panic(err)
		}

		fmt.Printf("%s: %v\n", comments, json.Valid(schema))
	}
	// Output:
	// raw: true
	// plain: true
}

func ExampleGenerateContext() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := modschema.GenerateContext(ctx, modschema.Options{Module: ".", Type: "Config"})
	fmt.Println(errors.Is(err, context.Canceled))
	// Output: true
}

func ExampleNormalizeOptions() {
	options := modschema.NormalizeOptions(modschema.Options{Type: "Config"})
	fmt.Println(options.Module)
	fmt.Println(options.Comments)
	// Output:
	// .
	// raw
}
