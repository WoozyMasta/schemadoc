// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc_test

import (
	"fmt"
	"strings"

	"github.com/woozymasta/schemadoc"
)

func ExampleRender() {
	schema := []byte(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	document, err := schemadoc.Render(schema, schemadoc.Options{
		Title: "Config Reference", TemplateName: "list", WrapWidth: 100,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(strings.Contains(document, "# Config Reference"))
	// Output: true
}

func ExampleRenderFile() {
	document, err := schemadoc.RenderFile("testdata/generated/app.schema.json", schemadoc.Options{
		Title: "Config Reference", TemplateName: "table",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(strings.Contains(document, "# Config Reference"))
	// Output: true
}

func ExampleBuiltinTemplate() {
	template, err := schemadoc.BuiltinTemplate("list")
	if err != nil {
		panic(err)
	}

	fmt.Println(len(template) > 0)
	// Output: true
}

func ExampleBuiltinTemplateNames() {
	fmt.Println(strings.Join(schemadoc.BuiltinTemplateNames(), ", "))
	// Output: html, list, table
}

func ExampleDetectDraft() {
	info := schemadoc.DetectDraft("https://json-schema.org/draft/2020-12/schema")
	fmt.Printf("draft=%s supported=%v\n", info.Canonical, info.Supported)
	// Output: draft=2020-12 supported=true
}

func ExampleGenerateExampleJSON() {
	schema := []byte(`{"type":"object","required":["name"],"properties":{"name":{"type":"string","default":"demo"}}}`)
	payload, err := schemadoc.GenerateExampleJSON(schema, schemadoc.ExampleModeRequired)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(payload))
	// Output:
	// {
	//   "name": "demo"
	// }
}

func ExampleRender_embeddedExample() {
	schema := []byte(`{"type":"object","required":["name"],"properties":{"name":{"type":"string","default":"demo"}}}`)
	document, err := schemadoc.Render(schema, schemadoc.Options{
		TemplateName: "list",
		ExampleMode:  schemadoc.ExampleModeRequired, ExampleFormat: schemadoc.ExampleFormatYAML,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(strings.Contains(document, "name: demo"))
	// Output: true
}

func ExampleRender_html() {
	schema := []byte(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	document, err := schemadoc.Render(schema, schemadoc.Options{
		Title: "Config Reference", TemplateName: "html",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(strings.Contains(document, "<h1>Config Reference</h1>"))
	// Output: true
}
