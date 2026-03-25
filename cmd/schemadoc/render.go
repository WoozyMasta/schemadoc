// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/woozymasta/schemadoc"
	"github.com/woozymasta/schemadoc/modschema"
	"go.yaml.in/yaml/v3"
)

// markdownRenderRequest stores markdown rendering request parameters.
type markdownRenderRequest struct {
	TemplateName      string
	Title             string
	Description       string
	TemplatePath      string
	ListMarker        string
	ExampleMode       string
	ExampleFmt        string
	OutputPath        string
	ExampleOut        exampleOutputOptions
	WrapWidth         int
	HideExtraKeywords bool
}

// jsonOutputOptions stores JSON formatting options for CLI output.
type jsonOutputOptions struct {
	IndentType string
	Indent     int
	Minify     bool
}

// yamlOutputOptions stores YAML formatting/comment options for CLI output.
type yamlOutputOptions struct {
	Indent                 int
	DisableExampleComments bool
}

// exampleOutputOptions stores JSON/YAML example output options.
type exampleOutputOptions struct {
	JSON jsonOutputOptions
	YAML yamlOutputOptions
}

// runModuleToMarkdown executes module-to-markdown flow without temporary schema files.
func (runner *cliRunner) runModuleToMarkdown(
	moduleOptions modschema.Options,
	request markdownRenderRequest,
) error {
	schemaBytes, sourcePath, err := modschema.Generate(moduleOptions)
	if err != nil {
		return fmt.Errorf("generate schema: %w", err)
	}

	if err := runner.runSchemaToDocBytes(schemaBytes, sourcePath, request); err != nil {
		return err
	}

	runner.logf(
		"mod2doc: module=%s package=%s type=%s output=%s status=ok",
		firstNonEmpty(moduleOptions.Module, "."),
		firstNonEmpty(moduleOptions.Package, "-"),
		moduleOptions.Type,
		firstNonEmpty(request.OutputPath, "-"),
	)
	return nil
}

// runModuleToSchema executes module-to-schema flow and writes result to stdout or file.
func (runner *cliRunner) runModuleToSchema(
	moduleOptions modschema.Options,
	outputPath string,
	jsonOptions jsonOutputOptions,
) error {
	schemaBytes, _, err := modschema.Generate(moduleOptions)
	if err != nil {
		return fmt.Errorf("generate schema: %w", err)
	}

	formattedSchema, err := formatSchemaOutput(schemaBytes, outputPath, jsonOptions)
	if err != nil {
		return fmt.Errorf("format schema: %w", err)
	}

	if err := writeBytes(runner.stdout, outputPath, formattedSchema, "schema"); err != nil {
		return err
	}

	runner.logf(
		"mod2schema: module=%s package=%s type=%s output=%s status=ok",
		firstNonEmpty(moduleOptions.Module, "."),
		firstNonEmpty(moduleOptions.Package, "-"),
		moduleOptions.Type,
		firstNonEmpty(outputPath, "-"),
	)
	return nil
}

// runSchemaToDoc executes schema-to-doc flow and writes result to stdout or file.
func (runner *cliRunner) runSchemaToDoc(inputPath string, render markdownRenderRequest) error {
	schemaBytes, sourcePath, err := readSchemaInput(inputPath, runner.stdin)
	if err != nil {
		return fmt.Errorf("read schema input: %w", err)
	}

	if err := runner.runSchemaToDocBytes(schemaBytes, sourcePath, render); err != nil {
		return err
	}

	runner.logf(
		"schema2doc: input=%s output=%s template=%s status=ok",
		firstNonEmpty(strings.TrimSpace(inputPath), "(stdin)"),
		firstNonEmpty(strings.TrimSpace(render.OutputPath), "-"),
		firstNonEmpty(strings.TrimSpace(render.TemplateName), "list"),
	)
	return nil
}

// runSchemaToExample generates example payload for selected mode and format.
func (runner *cliRunner) runSchemaToExample(
	mode,
	format,
	inputPath,
	outputPath string,
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
		return fmt.Errorf(
			"generate %s %s example: %w",
			selectedMode,
			selectedFormat,
			err,
		)
	}

	if err := writeBytes(runner.stdout, outputPath, content, "example"); err != nil {
		return err
	}

	runner.logf(
		"schema2%s: input=%s output=%s mode=%s status=ok",
		strings.ToLower(strings.TrimSpace(format)),
		firstNonEmpty(strings.TrimSpace(inputPath), "(stdin)"),
		firstNonEmpty(strings.TrimSpace(outputPath), "-"),
		firstNonEmpty(strings.TrimSpace(mode), "all"),
	)
	return nil
}

// toExampleOptions converts CLI options to schemadoc example options.
func toExampleOptions(options exampleOutputOptions) schemadoc.ExampleOptions {
	return schemadoc.ExampleOptions{
		JSONIndent:             options.JSON.Indent,
		JSONIndentType:         options.JSON.IndentType,
		JSONMinify:             options.JSON.Minify,
		YAMLIndent:             options.YAML.Indent,
		DisableExampleComments: options.YAML.DisableExampleComments,
	}
}

// formatJSONOutput applies JSON indentation/minify settings to JSON bytes.
func formatJSONOutput(content []byte, options jsonOutputOptions) ([]byte, error) {
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		return nil, err
	}

	exampleOptions := toExampleOptions(exampleOutputOptions{
		JSON: options,
	})
	return marshalJSONValue(decoded, exampleOptions)
}

// marshalJSONValue serializes JSON value with configured formatting.
func marshalJSONValue(value any, options schemadoc.ExampleOptions) ([]byte, error) {
	if options.JSONMinify {
		return json.Marshal(value)
	}

	indentSize := options.JSONIndent
	if indentSize < 1 {
		indentSize = 2
	}

	indent := strings.Repeat(" ", indentSize)
	if strings.EqualFold(strings.TrimSpace(options.JSONIndentType), "tab") {
		indent = strings.Repeat("\t", indentSize)
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)

	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// formatSchemaOutput formats schema output as JSON or YAML by output path extension.
func formatSchemaOutput(
	content []byte,
	outputPath string,
	options jsonOutputOptions,
) ([]byte, error) {
	if isYAMLOutputPath(outputPath) {
		return formatYAMLOutputFromJSON(content, options.Indent)
	}

	return formatJSONOutput(content, options)
}

// isYAMLOutputPath reports whether output path extension selects YAML format.
func isYAMLOutputPath(path string) bool {
	extension := strings.ToLower(strings.TrimSpace(filepath.Ext(path)))
	return extension == ".yaml" || extension == ".yml"
}

// formatYAMLOutputFromJSON converts JSON schema bytes to YAML with configured indentation.
func formatYAMLOutputFromJSON(content []byte, indent int) ([]byte, error) {
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		return nil, err
	}

	if indent < 1 {
		indent = 2
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(indent)

	if err := encoder.Encode(decoded); err != nil {
		return nil, err
	}

	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// runSchemaToDocBytes renders docs from schema bytes and writes result.
func (runner *cliRunner) runSchemaToDocBytes(
	schemaBytes []byte,
	sourcePath string,
	request markdownRenderRequest,
) error {
	rendered, err := runner.renderSchemaToDoc(schemaBytes, sourcePath, request)
	if err != nil {
		return err
	}

	return writeString(
		runner.stdout,
		request.OutputPath,
		rendered,
		"markdown",
	)
}

// renderSchemaToDoc renders docs from schema payload.
func (runner *cliRunner) renderSchemaToDoc(
	schemaBytes []byte,
	sourcePath string,
	request markdownRenderRequest,
) (string, error) {
	draftURI := extractSchemaDraftURI(schemaBytes)
	draft := schemadoc.DetectDraft(draftURI)
	if strings.TrimSpace(draftURI) == "" {
		_, _ = fmt.Fprintln(
			runner.stderr,
			"warning: schema has no $schema value; draft support is unknown",
		)
	} else if !draft.Supported {
		_, _ = fmt.Fprintf(
			runner.stderr,
			"warning: unsupported $schema value %q\n",
			draftURI,
		)
	}

	mode, format, err := parseMarkdownExampleOptions(
		request.ExampleMode,
		request.ExampleFmt,
	)
	if err != nil {
		return "", err
	}

	options := schemadoc.Options{
		Title:             request.Title,
		Description:       request.Description,
		SourcePath:        sourcePath,
		TemplateName:      request.TemplateName,
		ListMarker:        request.ListMarker,
		HideExtraKeywords: request.HideExtraKeywords,
		ExampleMode:       mode,
		ExampleFormat:     format,
		ExampleOptions:    toExampleOptions(request.ExampleOut),
		WrapWidth:         request.WrapWidth,
		FooterToolName:    "schemadoc",
		FooterToolURL:     URL,
		FooterVersion:     Version,
		FooterCommit:      Commit,
	}

	if request.TemplatePath != "" {
		customTemplate, err := os.ReadFile(request.TemplatePath)
		if err != nil {
			return "", fmt.Errorf("read template file %q: %w", request.TemplatePath, err)
		}

		options.TemplateText = string(customTemplate)
	}

	rendered, err := schemadoc.Render(schemaBytes, options)
	if err != nil {
		return "", err
	}

	return rendered, nil
}

// parseMarkdownExampleOptions parses optional markdown embedded example options.
func parseMarkdownExampleOptions(
	modeRaw,
	formatRaw string,
) (schemadoc.ExampleMode, schemadoc.ExampleFormat, error) {
	formatRaw = strings.TrimSpace(formatRaw)
	if formatRaw == "" {
		return "", "", nil
	}

	mode, err := parseExampleMode(modeRaw)
	if err != nil {
		return "", "", err
	}

	format, err := parseExampleFormat(formatRaw)
	if err != nil {
		return "", "", err
	}

	return mode, format, nil
}

// extractSchemaDraftURI extracts raw $schema URI from schema payload.
func extractSchemaDraftURI(schemaBytes []byte) string {
	var root map[string]any
	if err := json.Unmarshal(schemaBytes, &root); err != nil {
		return ""
	}

	value, ok := root["$schema"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(value)
}
