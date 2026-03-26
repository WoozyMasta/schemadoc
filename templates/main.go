// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

// Package templates stores embedded schemadoc templates.
package templates

import (
	"embed"
	"fmt"
	"strings"
)

const (
	// TemplateList is list-style markdown template name.
	TemplateList = "list"
	// TemplateTable is table-style markdown template name.
	TemplateTable = "table"
	// TemplateHTML is HTML template name.
	TemplateHTML = "html"
)

var (
	//go:embed *.gotmpl
	builtinTemplateFS embed.FS
)

var templateNames = []string{TemplateHTML, TemplateList, TemplateTable}

func templatePath(name string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case TemplateList:
		return "list.md.gotmpl", true

	case TemplateTable:
		return "table.md.gotmpl", true

	case TemplateHTML:
		return "html.gotmpl", true

	default:
		return "", false
	}
}

// HasTemplate reports whether template name is available.
func HasTemplate(name string) bool {
	_, exists := templatePath(name)
	return exists
}

// TemplateNames returns built-in template names.
func TemplateNames() []string {
	names := make([]string, len(templateNames))
	copy(names, templateNames)
	return names
}

// ReadTemplate reads one built-in template by name.
func ReadTemplate(name string) ([]byte, error) {
	path, ok := templatePath(name)
	if !ok {
		return nil, fmt.Errorf("unknown builtin template %q", name)
	}

	data, err := builtinTemplateFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read builtin template %q: %w", name, err)
	}

	return data, nil
}
