// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"github.com/woozymasta/schemadoc/modschema"
)

// Execute runs mod2schema subcommand.
func (command *moduleToSchemaCommand) Execute(_ []string) error {
	return command.runner.runModuleToSchema(modschema.Options{
		Module:            command.Args.Module,
		Type:              command.ModuleFlags.TypeName,
		Package:           command.ModuleFlags.PackagePath,
		KeyNamer:          command.ModuleFlags.KeyNamer,
		JSONSchemaVersion: command.ModuleFlags.JSONSchemaVersion,
	}, command.Args.Output, jsonOutputOptions{
		Indent:     command.JSONFlags.Indent,
		IndentType: command.JSONFlags.IndentType,
		Minify:     command.JSONFlags.Minify,
	})
}

// Execute runs mod2doc subcommand.
func (command *moduleToMarkdownCommand) Execute(_ []string) error {
	return command.runner.runModuleToMarkdown(
		modschema.Options{
			Module:            command.Args.Module,
			Type:              command.ModuleFlags.TypeName,
			Package:           command.ModuleFlags.PackagePath,
			KeyNamer:          command.ModuleFlags.KeyNamer,
			JSONSchemaVersion: command.ModuleFlags.JSONSchemaVersion,
		},
		markdownRenderRequest{
			TemplateName:         command.TemplateFlags.TemplateName,
			Title:                command.RenderFlags.Title,
			Description:          command.RenderFlags.Description,
			TemplatePath:         command.RenderFlags.TemplatePath,
			WrapWidth:            command.RenderFlags.WrapWidth,
			ListMarker:           command.RenderFlags.ListMarker,
			HideExtraKeywords:    command.RenderFlags.HideExtraKeywords,
			ShowInternalKeywords: command.RenderFlags.ShowInternalKeywords,
			Footer:               command.RenderFlags.Footer,
			ExampleMode:          command.ExampleFlags.Mode,
			ExampleFmt:           command.ExampleFlags.Format,
			OutputPath:           command.Args.Output,
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
		},
	)
}
