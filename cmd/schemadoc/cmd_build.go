// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woozymasta/schemadoc"
	"github.com/woozymasta/schemadoc/cmd/schemadoc/buildcfg"
	"github.com/woozymasta/schemadoc/merge"
	"github.com/woozymasta/schemadoc/modschema"
)

const defaultBuildConfigPath = "schemadoc.build.yaml"

// Execute runs build subcommand.
func (command *buildCommand) Execute(_ []string) error {
	configPath := resolveBuildConfigPath(command.Args.ConfigPath)
	command.runner.logf(
		"build: config=%s index=%d",
		configPath,
		command.ConfigIndex,
	)

	if err := command.runner.runConfig(
		configPath,
		command.ConfigIndex,
	); err != nil {
		return err
	}

	command.runner.logf("build: ok")
	return nil
}

// resolveBuildConfigPath resolves explicit path or default build config path.
func resolveBuildConfigPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed != "" {
		return trimmed
	}

	return defaultBuildConfigPath
}

// runConfig executes selected config documents from config file.
func (runner *cliRunner) runConfig(configPath string, configIndex int) error {
	loadedConfigs, err := loadBuildConfig(configPath, configIndex)
	if err != nil {
		return err
	}

	for index, loadedConfig := range loadedConfigs {
		docNumber := index + 1
		if configIndex > 0 {
			docNumber = configIndex
		}

		if err := runner.runLoadedConfig(docNumber, loadedConfig); err != nil {
			return err
		}
	}

	return nil
}

// runLoadedConfig executes one pipeline document in deterministic stage order.
func (runner *cliRunner) runLoadedConfig(docNumber int, loadedConfig buildcfg.Config) error {
	schemaPath := strings.TrimSpace(loadedConfig.Schema)
	if schemaPath == "" {
		return fmt.Errorf("config doc %d: schema is required", docNumber)
	}

	if loadedConfig.Mod2Schema != nil {
		stage := loadedConfig.Mod2Schema
		moduleOptions := modschema.Options{
			Module:   strings.TrimSpace(stage.Module),
			Type:     strings.TrimSpace(stage.Type),
			Package:  strings.TrimSpace(stage.Package),
			KeyNamer: strings.TrimSpace(stage.KeyNamer),
		}
		if loadedConfig.Check {
			if err := validateModuleReferenceForCheck(moduleOptions.Module); err != nil {
				return fmt.Errorf("config doc %d stage mod2schema: %w", docNumber, err)
			}
		}

		err := runner.runModuleToSchemaForBuild(
			moduleOptions,
			schemaPath,
			loadedConfig.Check,
			jsonOutputFromBuild(stage.JSON),
		)
		if err != nil {
			return fmt.Errorf("config doc %d stage mod2schema: %w", docNumber, err)
		}
		runner.logf(
			"build: doc=%d stage=mod2schema output=%s status=ok",
			docNumber,
			schemaPath,
		)
	}

	if loadedConfig.Merge != nil {
		if err := runner.RunSchemaMerge(
			schemaPath,
			loadedConfig.Merge,
			loadedConfig.Check,
		); err != nil {
			return fmt.Errorf("config doc %d stage merge: %w", docNumber, err)
		}
		runner.logf(
			"build: doc=%d stage=merge output=%s status=ok",
			docNumber,
			schemaPath,
		)
	}

	if loadedConfig.Schema2JSON != nil {
		stage := loadedConfig.Schema2JSON
		outputPath := resolveBuildExampleOutputPath(schemaPath, strings.TrimSpace(stage.Output), "json")
		if err := runner.runSchemaToExampleForBuild(
			firstNonEmpty(stage.Mode, "all"),
			"json",
			schemaPath,
			outputPath,
			loadedConfig.Check,
			exampleOutputOptions{
				JSON: jsonOutputFromBuild(stage.JSON),
			},
		); err != nil {
			return fmt.Errorf("config doc %d stage schema2json: %w", docNumber, err)
		}
		runner.logf(
			"build: doc=%d stage=schema2json output=%s status=ok",
			docNumber,
			outputPath,
		)
	}

	if loadedConfig.Schema2Doc != nil {
		stage := loadedConfig.Schema2Doc
		outputPath := resolveBuildDocOutputPath(
			schemaPath,
			strings.TrimSpace(stage.Output),
			strings.TrimSpace(stage.Template),
		)
		if err := runner.runSchemaToDocForBuild(schemaPath, markdownRenderRequest{
			TemplateName:      strings.TrimSpace(stage.Template),
			TemplatePath:      strings.TrimSpace(stage.TemplateFile),
			Title:             strings.TrimSpace(stage.Title),
			Description:       strings.TrimSpace(stage.Description),
			ListMarker:        strings.TrimSpace(stage.ListMarker),
			WrapWidth:         stage.Wrap,
			HideExtraKeywords: stage.HideExtraKeywords,
			Footer:            stage.Footer,
			ExampleMode:       strings.TrimSpace(stage.Mode),
			ExampleFmt:        strings.TrimSpace(stage.Format),
			OutputPath:        outputPath,
			ExampleOut: exampleOutputOptions{
				JSON: jsonOutputFromBuild(stage.JSON),
				YAML: yamlOutputFromBuild(stage.YAML),
			},
		}, loadedConfig.Check); err != nil {
			return fmt.Errorf("config doc %d stage schema2doc: %w", docNumber, err)
		}
		runner.logf(
			"build: doc=%d stage=schema2doc output=%s status=ok",
			docNumber,
			outputPath,
		)
	}

	if loadedConfig.Schema2YAML != nil {
		stage := loadedConfig.Schema2YAML
		outputPath := resolveBuildExampleOutputPath(schemaPath, strings.TrimSpace(stage.Output), "yaml")
		if err := runner.runSchemaToExampleForBuild(
			firstNonEmpty(stage.Mode, "all"),
			"yaml",
			schemaPath,
			outputPath,
			loadedConfig.Check,
			exampleOutputOptions{
				YAML: yamlOutputFromBuild(stage.YAML),
			},
		); err != nil {
			return fmt.Errorf("config doc %d stage schema2yaml: %w", docNumber, err)
		}
		runner.logf(
			"build: doc=%d stage=schema2yaml output=%s status=ok",
			docNumber,
			outputPath,
		)
	}

	return nil
}

// resolveBuildExampleOutputPath returns explicit output or schema-derived default.
func resolveBuildExampleOutputPath(schemaPath, outputPath, extension string) string {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath != "" {
		return outputPath
	}

	basePath := buildSchemaBasePath(schemaPath)
	return basePath + "." + strings.ToLower(strings.TrimSpace(extension))
}

// resolveBuildDocOutputPath returns explicit output or template-based default.
func resolveBuildDocOutputPath(schemaPath, outputPath, templateName string) string {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath != "" {
		return outputPath
	}

	basePath := buildSchemaBasePath(schemaPath)
	switch strings.ToLower(strings.TrimSpace(templateName)) {
	case "table":
		return basePath + ".table.md"
	case "html":
		return basePath + ".html"
	default:
		return basePath + ".list.md"
	}
}

// buildSchemaBasePath strips extension from schema path for derived outputs.
func buildSchemaBasePath(schemaPath string) string {
	schemaPath = strings.TrimSpace(schemaPath)
	extension := filepath.Ext(schemaPath)
	if extension == "" {
		return schemaPath
	}

	return strings.TrimSuffix(schemaPath, extension)
}

// validateModuleReferenceForCheck rejects non-deterministic module refs in check mode.
func validateModuleReferenceForCheck(module string) error {
	module = strings.TrimSpace(module)
	if strings.EqualFold(module, "") {
		module = "."
	}

	if strings.HasSuffix(strings.ToLower(module), "@latest") {
		return errors.New("module version @latest is not allowed in check mode")
	}

	return nil
}

// RunSchemaMerge executes merge stage over one working schema.
func (runner *cliRunner) RunSchemaMerge(
	schemaPath string,
	stage *buildcfg.MergeStage,
	check bool,
) error {
	if stage == nil {
		return nil
	}

	plan := schemaMergePlan{
		SourcePath:           strings.TrimSpace(schemaPath),
		TargetPath:           strings.TrimSpace(schemaPath),
		Check:                check,
		InPlace:              true,
		PruneUnreachableDefs: stage.PruneUnreachableDefs,
		Actions: make(
			[]merge.Action,
			0,
			len(stage.Patches)+len(stage.Imports)*8,
		),
	}

	for index, patch := range stage.Patches {
		action := merge.Action{
			Type:          firstNonEmpty(strings.TrimSpace(patch.Op.Node), merge.NodeOpReplace),
			SourcePath:    strings.TrimSpace(patch.File),
			SourcePointer: strings.TrimSpace(patch.Source),
			TargetPointer: strings.TrimSpace(patch.Target),
			ObjectOp:      firstNonEmpty(strings.TrimSpace(patch.Op.Object), merge.ObjectOpMerge),
			ArrayOp:       firstNonEmpty(strings.TrimSpace(patch.Op.Array), merge.ArrayOpReplace),
		}
		if strings.TrimSpace(action.SourcePath) == "" {
			return fmt.Errorf("merge.patches[%d].file is required", index)
		}

		plan.Actions = append(plan.Actions, action)
	}

	importActions, err := buildImportActions(schemaPath, stage.Imports)
	if err != nil {
		return err
	}

	plan.Actions = append(plan.Actions, importActions...)
	return runner.executeSchemaMergePlan(plan)
}

// buildImportActions expands high-level defs imports into merge actions.
func buildImportActions(
	targetSchemaPath string,
	imports []buildcfg.MergeImport,
) ([]merge.Action, error) {
	if len(imports) == 0 {
		return nil, nil
	}

	mergeImports := make([]merge.DefsImport, 0, len(imports))
	for index, item := range imports {
		sourcePath := strings.TrimSpace(item.File)
		if sourcePath == "" {
			return nil, fmt.Errorf("merge.imports[%d].file is required", index)
		}

		mergeImports = append(mergeImports, merge.DefsImport{
			SourcePath: strings.TrimSpace(item.File),
			SourceDefs: strings.TrimSpace(item.SourceDefs),
			TargetDefs: strings.TrimSpace(item.TargetDefs),
			Rename:     mergeRenameFromBuild(item.Rename),
			Conflict:   strings.TrimSpace(item.Conflict),
		})
	}

	actions, err := merge.PlanImportsFile(strings.TrimSpace(targetSchemaPath), mergeImports)
	if err != nil {
		return nil, fmt.Errorf("merge.imports: %w", err)
	}

	return actions, nil
}

// runModuleToSchemaForBuild runs mod2schema stage in write/check mode.
func (runner *cliRunner) runModuleToSchemaForBuild(
	options modschema.Options,
	outputPath string,
	check bool,
	jsonOptions jsonOutputOptions,
) error {
	content, _, err := modschema.Generate(options)
	if err != nil {
		return fmt.Errorf("generate schema: %w", err)
	}

	formattedSchema, err := formatSchemaOutput(content, outputPath, jsonOptions)
	if err != nil {
		return fmt.Errorf("format schema: %w", err)
	}

	if !check {
		return writeBytes(runner.stdout, outputPath, formattedSchema, "schema")
	}

	return checkFileContent(outputPath, formattedSchema, "schema")
}

// runSchemaToExampleForBuild runs schema2json/yaml stage in write/check mode.
func (runner *cliRunner) runSchemaToExampleForBuild(
	mode string,
	format string,
	inputPath string,
	outputPath string,
	check bool,
	options exampleOutputOptions,
) error {
	schemaBytes, _, err := readSchemaInput(inputPath, runner.stdin)
	if err != nil {
		return fmt.Errorf("read schema input: %w", err)
	}

	selectedMode, err := parseExampleMode(mode)
	if err != nil {
		return err
	}

	selectedFormat, err := parseExampleFormat(format)
	if err != nil {
		return err
	}

	content, err := schemadoc.GenerateExampleWithOptions(
		schemaBytes,
		selectedMode,
		selectedFormat,
		toExampleOptions(options),
	)
	if err != nil {
		return fmt.Errorf("generate %s %s example: %w", selectedMode, selectedFormat, err)
	}

	if !check {
		return writeBytes(runner.stdout, outputPath, content, "example")
	}

	return checkFileContent(outputPath, content, "example")
}

// runSchemaToDocForBuild runs schema2doc stage in write/check mode.
func (runner *cliRunner) runSchemaToDocForBuild(
	inputPath string,
	request markdownRenderRequest,
	check bool,
) error {
	schemaBytes, sourcePath, err := readSchemaInput(inputPath, runner.stdin)
	if err != nil {
		return fmt.Errorf("read schema input: %w", err)
	}

	rendered, err := runner.renderSchemaToDoc(
		schemaBytes,
		sourcePath,
		request,
	)
	if err != nil {
		return err
	}

	if !check {
		return writeString(
			runner.stdout,
			request.OutputPath,
			rendered,
			"markdown",
		)
	}

	return checkFileContent(request.OutputPath, []byte(rendered), "markdown")
}

// jsonOutputFromBuild converts optional build JSON options to runtime options.
func jsonOutputFromBuild(options *buildcfg.JSONOutputOptions) jsonOutputOptions {
	if options == nil {
		return jsonOutputOptions{}
	}

	return jsonOutputOptions{
		Indent:     options.Indent,
		IndentType: options.IndentType,
		Minify:     options.Minify,
	}
}

// yamlOutputFromBuild converts optional build YAML options to runtime options.
func yamlOutputFromBuild(options *buildcfg.YAMLOutputOptions) yamlOutputOptions {
	if options == nil {
		return yamlOutputOptions{}
	}

	return yamlOutputOptions{
		Indent:                 options.Indent,
		DisableExampleComments: options.DisableExampleComments,
	}
}

// mergeRenameFromBuild converts optional build rename options to merge API options.
func mergeRenameFromBuild(options *buildcfg.MergeImportRename) merge.DefsImportRename {
	if options == nil {
		return merge.DefsImportRename{}
	}

	return merge.DefsImportRename{
		Mode:  strings.TrimSpace(options.Mode),
		Value: strings.TrimSpace(options.Value),
	}
}
