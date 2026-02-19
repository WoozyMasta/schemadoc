// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	// defaultTitle is used when caller does not provide custom title.
	defaultTitle = "schema reference"
	// defaultTemplateName is used when caller does not provide template name.
	defaultTemplateName = "list"
	// defaultWrapWidth wraps plain description paragraphs at this width.
	defaultWrapWidth = 80
	// defaultListMarker is used when caller does not provide list marker style.
	defaultListMarker = "*"
)

const (
	templateListName  = "list"
	templateTableName = "table"
)

// renderView is the root view model passed to markdown templates.
type renderView struct {
	Title              string
	SourceSchema       string
	SchemaID           string
	SchemaDraft        string
	SchemaDraftSupport string
	RootRef            string
	ListMarker         string
	Definitions        []definitionView
}

// definitionView represents one top-level definition section in markdown output.
type definitionView struct {
	Name          string
	Description   string
	Attributes    []attributeView
	Properties    []propertyView
	HasProperties bool
}

// propertyView represents one property section inside a definition.
type propertyView struct {
	Heading     string
	Name        string
	Paths       []string
	Description string
	Attributes  []attributeView
}

// attributeView is a single rendered name/value metadata item.
type attributeView struct {
	Name  string
	Value string
}

// RenderFile reads schema from file and renders markdown documentation.
func RenderFile(path string, opt Options) (string, error) {
	schemaBytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrReadSchemaFile, err)
	}

	if strings.TrimSpace(opt.SourcePath) == "" {
		opt.SourcePath = path
	}

	return Render(schemaBytes, opt)
}

// Render converts schema bytes into deterministic CommonMark document.
func Render(schemaBytes []byte, opt Options) (string, error) {
	doc, err := parseDocument(schemaBytes)
	if err != nil {
		return "", err
	}

	view, err := buildRenderView(doc, opt)
	if err != nil {
		return "", err
	}

	markdownTemplate, err := resolveTemplate(opt)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	if err := markdownTemplate.Execute(&out, view); err != nil {
		return "", fmt.Errorf("%w: %w", ErrExecuteMarkdownTemplate, err)
	}

	return ensureTrailingNewline(normalizeMarkdownOutput(out.String())), nil
}

// BuiltinTemplateNames returns all available built-in template names.
func BuiltinTemplateNames() []string {
	names := make([]string, 0, len(builtInTemplateFiles))
	for name := range builtInTemplateFiles {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// BuiltinTemplate returns one built-in template by name.
func BuiltinTemplate(name string) (string, error) {
	name = normalizeTemplateName(name)
	path, ok := builtInTemplateFiles[name]
	if !ok {
		return "", fmt.Errorf("%w %q", ErrUnknownBuiltinTemplate, name)
	}

	data, err := templateFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrReadBuiltinTemplate, err)
	}

	return string(data), nil
}
