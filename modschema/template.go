// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"bytes"
	_ "embed"
	"fmt"
	"sync"
	"text/template"
)

const helperTemplateName = "helper"

//go:embed helper.go.tmpl
var mod2schemaHelperTemplate string

var (
	schemaGeneratorHelperOnce sync.Once
	schemaGeneratorHelper     *template.Template
	schemaGeneratorHelperErr  error
)

// Options configures module-to-schema generation.
type Options struct {
	// Module selects the target module directory or module reference.
	Module string
	// Package selects the Go package containing the target type.
	Package string
	// Type selects the root Go type to reflect.
	Type string
	// KeyNamer controls names for fields without explicit JSON tags.
	KeyNamer string
	// RootID overrides the generated root JSON Schema $id when non-empty.
	RootID string
	// JSONSchemaVersion overrides the jsonschema helper dependency version.
	JSONSchemaVersion string
}

// templateData provides values for helper source template.
type templateData struct {
	PackagePath string
	TypeName    string
	ModulePath  string
	ModuleDir   string
	KeyNamer    string
	RootID      string
}

// BuildProgramSource renders temporary Go source for target type reflection.
func BuildProgramSource(options Options) (string, error) {
	return renderProgramSource(templateData{
		PackagePath: options.Package,
		TypeName:    options.Type,
		ModulePath:  options.Module,
		ModuleDir:   options.Module,
		KeyNamer:    options.KeyNamer,
		RootID:      options.RootID,
	})
}

// renderProgramSource renders helper source from prepared template values.
func renderProgramSource(data templateData) (string, error) {
	schemaGeneratorHelperOnce.Do(func() {
		schemaGeneratorHelper, schemaGeneratorHelperErr = template.New(
			helperTemplateName,
		).Parse(mod2schemaHelperTemplate)
	})
	if schemaGeneratorHelperErr != nil {
		return "", fmt.Errorf(
			"parse mod2schema helper template: %w",
			schemaGeneratorHelperErr,
		)
	}

	var out bytes.Buffer
	if err := schemaGeneratorHelper.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render mod2schema helper template: %w", err)
	}

	return out.String(), nil
}
