// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"github.com/woozymasta/schemadoc"
	"github.com/woozymasta/schemadoc/modschema"
)

// moduleOptionsFromFlags converts reflection flags into generator options.
func moduleOptionsFromFlags(module string, flags moduleReflectFlags) modschema.Options {
	return modschema.Options{
		Module:            module,
		Type:              flags.TypeName,
		Package:           flags.PackagePath,
		KeyNamer:          flags.KeyNamer,
		TypeNamer:         modschema.TypeNamerMode(flags.TypeNamer),
		Comments:          modschema.CommentMode(flags.Comments),
		RootID:            flags.RootID,
		JSONSchemaVersion: flags.JSONSchemaVersion,
	}
}

// Execute runs mod2json subcommand.
func (command *moduleToJSONCommand) Execute(_ []string) error {
	return command.runner.runModuleToExample(
		moduleOptionsFromFlags(command.Args.Module, command.ModuleFlags),
		command.ExampleFlags.Mode,
		schemadoc.ExampleFormatJSON,
		command.Args.Output,
		exampleOutputOptions{JSON: jsonOutputOptions{
			Indent:     command.JSONFlags.Indent,
			IndentType: command.JSONFlags.IndentType,
			Minify:     command.JSONFlags.Minify,
		}},
	)
}

// Execute runs mod2yaml subcommand.
func (command *moduleToYAMLCommand) Execute(_ []string) error {
	return command.runner.runModuleToExample(
		moduleOptionsFromFlags(command.Args.Module, command.ModuleFlags),
		command.ExampleFlags.Mode,
		schemadoc.ExampleFormatYAML,
		command.Args.Output,
		exampleOutputOptions{YAML: yamlOutputFromFlags(yamlExampleFlags(command.YAMLFlags))},
	)
}

// Execute runs mod2schema subcommand.
func (command *moduleToSchemaCommand) Execute(_ []string) error {
	return command.runner.runModuleToSchema(
		moduleOptionsFromFlags(command.Args.Module, command.ModuleFlags),
		command.Args.Output, jsonOutputOptions{
			Indent:     command.JSONFlags.Indent,
			IndentType: command.JSONFlags.IndentType,
			Minify:     command.JSONFlags.Minify,
		})
}

// Execute runs mod2doc subcommand.
func (command *moduleToMarkdownCommand) Execute(_ []string) error {
	return command.runner.runModuleToMarkdown(
		moduleOptionsFromFlags(command.Args.Module, command.ModuleFlags),
		markdownRenderRequest{
			TemplateName:         command.TemplateFlags.TemplateName,
			Title:                command.RenderFlags.Title,
			Description:          command.RenderFlags.Description,
			SourcePath:           command.RenderFlags.SourcePath,
			SourceRawPath:        command.RenderFlags.SourceRawPath,
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
				YAML: yamlOutputFromFlags(command.YAMLFlags),
			},
		},
	)
}
