// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"github.com/woozymasta/schemadoc"
)

// Execute runs schema2doc subcommand.
func (command *schemaToDocCommand) Execute(_ []string) error {
	return command.runner.runSchemaToDoc(command.Args.Input, markdownRenderRequest{
		TemplateName:      command.TemplateFlags.TemplateName,
		Title:             command.RenderFlags.Title,
		Description:       command.RenderFlags.Description,
		TemplatePath:      command.RenderFlags.TemplatePath,
		WrapWidth:         command.RenderFlags.WrapWidth,
		ListMarker:        command.RenderFlags.ListMarker,
		HideExtraKeywords: command.RenderFlags.HideExtraKeywords,
		Footer:            command.RenderFlags.Footer,
		ExampleMode:       command.ExampleFlags.Mode,
		ExampleFmt:        command.ExampleFlags.Format,
		OutputPath:        command.Args.Output,
		ExampleOut: exampleOutputOptions{
			JSON: jsonOutputOptions{
				Indent:     command.JSONFlags.Indent,
				IndentType: command.JSONFlags.IndentType,
				Minify:     command.JSONFlags.Minify,
			},
			YAML: yamlOutputOptions{
				Indent:                 command.YAMLFlags.Indent,
				DisableExampleComments: command.YAMLFlags.DisableExampleComments,
			},
		},
	})
}

// Execute runs schema2json subcommand.
func (command *schemaToJSONCommand) Execute(_ []string) error {
	return command.runner.runSchemaToExample(
		command.ExampleFlags.Mode,
		string(schemadoc.ExampleFormatJSON),
		command.Args.Input,
		command.Args.Output,
		exampleOutputOptions{
			JSON: jsonOutputOptions{
				Indent:     command.JSONFlags.Indent,
				IndentType: command.JSONFlags.IndentType,
				Minify:     command.JSONFlags.Minify,
			},
		},
	)
}

// Execute runs schema2yaml subcommand.
func (command *schemaToYAMLCommand) Execute(_ []string) error {
	return command.runner.runSchemaToExample(
		command.ExampleFlags.Mode,
		string(schemadoc.ExampleFormatYAML),
		command.Args.Input,
		command.Args.Output,
		exampleOutputOptions{
			YAML: yamlOutputOptions{
				Indent:                 command.YAMLFlags.Indent,
				DisableExampleComments: command.YAMLFlags.DisableExampleComments,
			},
		},
	)
}
