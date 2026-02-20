// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

/*
Package schemadoc renders CommonMark documentation from JSON Schema documents.

The package focuses on deterministic markdown output for generated schemas and
project configuration models. It supports built-in templates ("list", "table")
and custom template text.

Basic render from schema bytes:

	schemaBytes, err := os.ReadFile("schema.json")
	if err != nil {
		return err
	}

	md, err := schemadoc.Render(schemaBytes, schemadoc.Options{
		Title:        "Config Reference",
		SourcePath:   "schema.json",
		TemplateName: "list",
		WrapWidth:    100,
	})
	if err != nil {
		return err
	}

	fmt.Println(md)

Render directly from file:

	md, err := schemadoc.RenderFile("schema.json", schemadoc.Options{
		TemplateName: "table",
	})
	if err != nil {
		return err
	}

	fmt.Println(md)

Use built-in templates:

	names := schemadoc.BuiltinTemplateNames()
	fmt.Println(strings.Join(names, ", "))

	tpl, err := schemadoc.BuiltinTemplate("list")
	if err != nil {
		return err
	}

	fmt.Println(len(tpl) > 0)

Detect JSON Schema draft support:

	info := schemadoc.DetectDraft("https://json-schema.org/draft/2020-12/schema")
	fmt.Printf("draft=%s supported=%v\n", info.Canonical, info.Supported)

Generate example payload from schema:

	jsonExample, err := schemadoc.GenerateExampleJSON(schemaBytes, schemadoc.ExampleModeRequired)
	if err != nil {
		return err
	}

	fmt.Println(string(jsonExample))

Enable embedded example block in markdown template output:

	md, err := schemadoc.Render(schemaBytes, schemadoc.Options{
		TemplateName:  "list",
		ExampleMode:   schemadoc.ExampleModeRequired,
		ExampleFormat: schemadoc.ExampleFormatYAML,
	})
	if err != nil {
		return err
	}

	fmt.Println(md)
*/
package schemadoc
