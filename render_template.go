// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
	"unicode"
)

// templateFS stores built-in markdown templates embedded into the package.
//
//go:embed templates/*.md.gotmpl
var templateFS embed.FS

// builtInTemplateFiles maps template aliases to embedded file paths.
var builtInTemplateFiles = map[string]string{
	templateListName:  "templates/list.md.gotmpl",
	templateTableName: "templates/table.md.gotmpl",
}

// resolveTemplate resolves either custom or built-in template text into a parsed template.
func resolveTemplate(opt Options) (*template.Template, error) {
	templateText := strings.TrimSpace(opt.TemplateText)
	if templateText != "" {
		return template.New("custom").Funcs(templateFuncs()).Parse(templateText)
	}

	templateName := normalizeTemplateName(opt.TemplateName)
	if templateName == "" {
		templateName = defaultTemplateName
	}

	templateText, err := BuiltinTemplate(templateName)
	if err != nil {
		return nil, err
	}

	parsed, err := template.New(templateName).Funcs(templateFuncs()).Parse(templateText)
	if err != nil {
		return nil, fmt.Errorf("%w %q: %w", ErrParseBuiltinTemplate, templateName, err)
	}

	return parsed, nil
}

// normalizeTemplateName normalizes built-in template identifiers.
func normalizeTemplateName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// templateFuncs provides utility functions available inside markdown templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"jsonInline": func(value any) string {
			return escapeInline(mustJSONInline(value))
		},
		"headingAnchor": markdownHeadingAnchor,
	}
}

// markdownHeadingAnchor converts heading text into a markdown anchor slug.
func markdownHeadingAnchor(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}

	var out strings.Builder
	out.Grow(len(trimmed))

	lastDash := false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			out.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r), r == '-', r == '_':
			if lastDash || out.Len() == 0 {
				continue
			}

			out.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(out.String(), "-")
}
