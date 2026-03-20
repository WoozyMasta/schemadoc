// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"embed"
	"fmt"
	"html"
	"strings"
	"text/template"
	"unicode"
)

// templateFS stores built-in markdown templates embedded into the package.
//
//go:embed templates/*.md.gotmpl templates/html.gotmpl
var templateFS embed.FS

// builtInTemplateFiles maps template aliases to embedded file paths.
var builtInTemplateFiles = map[string]string{
	templateListName:  "templates/list.md.gotmpl",
	templateTableName: "templates/table.md.gotmpl",
	templateHTMLName:  "templates/html.gotmpl",
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
		"headingAnchor": markdownHeadingAnchor,
		"tableCell":     normalizeTableCell,
		"inlineHTML":    renderInlineMarkdownHTML,
		"attrHTML":      renderAttributeValueHTML,
		"blockHTML":     renderMarkdownBlockHTML,

		"renderAttrList": func(attrs []attributeView, marker string) string {
			if len(attrs) == 0 {
				return ""
			}

			items := make([]string, 0, len(attrs))
			for _, attr := range attrs {
				items = append(items, renderAttrItemMarkdown(attr, marker))
			}

			return strings.Join(items, "\n")
		},

		"jsonInline": func(value any) string {
			return escapeInline(mustJSONInline(value))
		},

		"html": func(value any) string {
			return html.EscapeString(fmt.Sprint(value))
		},
	}
}

// renderAttrItemMarkdown renders one markdown attribute list item.
func renderAttrItemMarkdown(attr attributeView, marker string) string {
	marker = strings.TrimSpace(marker)
	if marker != "-" && marker != "*" {
		marker = "*"
	}

	if code, ok := extractSingleCodeSpan(strings.TrimSpace(normalizeLineEndings(attr.Value))); ok &&
		strings.Contains(code, "\n") {
		codeLines := strings.Split(normalizeLineEndings(code), "\n")
		for index, line := range codeLines {
			codeLines[index] = "  " + line
		}

		return marker + " " + attr.Name + ":\n\n  ```text\n" +
			strings.Join(codeLines, "\n") + "\n  ```\n"
	}

	return marker + " " + attr.Name + ": " + attr.Value
}

// renderInlineMarkdownHTML renders a tiny markdown subset (links and code spans)
// into safe HTML for inline template values.
func renderInlineMarkdownHTML(value any) string {
	text := fmt.Sprint(value)
	text = normalizeLineEndings(text)
	if text == "" {
		return ""
	}

	var out strings.Builder
	out.Grow(len(text) + 16)

	for index := 0; index < len(text); {
		if text[index] == '`' {
			end := strings.IndexByte(text[index+1:], '`')
			if end >= 0 {
				code := text[index+1 : index+1+end]
				out.WriteString("<code>")
				out.WriteString(strings.ReplaceAll(html.EscapeString(code), "\n", "<br>"))
				out.WriteString("</code>")
				index += end + 2
				continue
			}
		}

		if text[index] == '\n' {
			out.WriteString("<br>")
			index++
			continue
		}

		linkText, href, consumed, ok := parseInlineMarkdownLink(text[index:])
		if ok {
			out.WriteString("<a href=\"")
			out.WriteString(html.EscapeString(href))
			out.WriteString("\">")
			out.WriteString(renderInlineMarkdownHTML(linkText))
			out.WriteString("</a>")
			index += consumed
			continue
		}

		out.WriteString(html.EscapeString(string(text[index])))
		index++
	}

	return out.String()
}

// renderAttributeValueHTML renders attribute value and upgrades multiline
// standalone code spans to pre/code blocks for readability.
func renderAttributeValueHTML(value any) string {
	text := normalizeLineEndings(fmt.Sprint(value))
	trimmed := strings.TrimSpace(text)

	if code, ok := extractSingleCodeSpan(trimmed); ok && strings.Contains(code, "\n") {
		return "<pre><code>" + html.EscapeString(code) + "</code></pre>"
	}

	return renderInlineMarkdownHTML(text)
}

// extractSingleCodeSpan returns code content when the whole input is one
// markdown code span: `...`.
func extractSingleCodeSpan(value string) (string, bool) {
	if len(value) < 2 {
		return "", false
	}
	if value[0] != '`' || value[len(value)-1] != '`' {
		return "", false
	}
	if strings.Count(value, "`") != 2 {
		return "", false
	}

	return value[1 : len(value)-1], true
}

// renderMarkdownBlockHTML renders simple markdown-like block content.
// It supports paragraphs and list items for `*`, `-`, `+`, and `1.` styles.
func renderMarkdownBlockHTML(value any) string {
	text := strings.TrimSpace(normalizeLineEndings(fmt.Sprint(value)))
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	blocks := make([]string, 0, 4)
	paragraph := make([]string, 0, 4)
	listItems := make([]string, 0, 4)
	orderedList := false

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}

		blocks = append(blocks, "<p>"+renderInlineMarkdownHTML(strings.Join(paragraph, " "))+"</p>")
		paragraph = paragraph[:0]
	}

	flushList := func() {
		if len(listItems) == 0 {
			return
		}

		tag := "ul"
		if orderedList {
			tag = "ol"
		}

		var builder strings.Builder
		builder.Grow(32 + len(listItems)*32)
		builder.WriteString("<")
		builder.WriteString(tag)
		builder.WriteString(">")

		for _, item := range listItems {
			builder.WriteString("<li>")
			builder.WriteString(renderInlineMarkdownHTML(item))
			builder.WriteString("</li>")
		}

		builder.WriteString("</")
		builder.WriteString(tag)
		builder.WriteString(">")
		blocks = append(blocks, builder.String())
		listItems = listItems[:0]
		orderedList = false
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			flushParagraph()
			flushList()
			continue
		}

		if item, isOrdered, ok := parseMarkdownListItem(trimmed); ok {
			flushParagraph()
			if len(listItems) > 0 && orderedList != isOrdered {
				flushList()
			}

			orderedList = isOrdered
			listItems = append(listItems, item)
			continue
		}

		flushList()
		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	flushList()
	return strings.Join(blocks, "\n")
}

// parseMarkdownListItem extracts markdown list item content from one trimmed line.
func parseMarkdownListItem(line string) (item string, ordered bool, ok bool) {
	for _, prefix := range []string{"* ", "- ", "+ "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):]), false, true
		}
	}

	index := 0
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		index++
	}
	if index == 0 || index+1 >= len(line) {
		return "", false, false
	}
	if line[index] != '.' && line[index] != ')' {
		return "", false, false
	}
	if line[index+1] != ' ' && line[index+1] != '\t' {
		return "", false, false
	}

	return strings.TrimSpace(line[index+1:]), true, true
}

// parseInlineMarkdownLink parses [text](href) from the beginning of input.
func parseInlineMarkdownLink(input string) (text, href string, consumed int, ok bool) {
	if !strings.HasPrefix(input, "[") {
		return "", "", 0, false
	}

	closeBracket := strings.IndexByte(input, ']')
	if closeBracket <= 1 {
		return "", "", 0, false
	}
	if len(input) <= closeBracket+2 || input[closeBracket+1] != '(' {
		return "", "", 0, false
	}

	closeParenOffset := strings.IndexByte(input[closeBracket+2:], ')')
	if closeParenOffset < 0 {
		return "", "", 0, false
	}

	text = input[1:closeBracket]
	href = input[closeBracket+2 : closeBracket+2+closeParenOffset]
	consumed = closeBracket + 3 + closeParenOffset

	if !isSafeLinkHref(href) {
		return "", "", 0, false
	}

	return text, href, consumed, true
}

// isSafeLinkHref validates that href has a safe scheme/path for inline links.
func isSafeLinkHref(href string) bool {
	href = strings.TrimSpace(href)
	if href == "" {
		return false
	}

	lower := strings.ToLower(href)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return false
	}

	if strings.HasPrefix(href, "#") {
		return true
	}
	if strings.HasPrefix(href, "/") ||
		strings.HasPrefix(href, "./") ||
		strings.HasPrefix(href, "../") {
		return true
	}
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return true
	}

	return !strings.Contains(href, ":")
}

// normalizeTableCell converts multiline markdown text into a safe table cell value.
func normalizeTableCell(value string) string {
	value = normalizeLineEndings(value)
	if strings.TrimSpace(value) == "" {
		return ""
	}

	parts := strings.Split(value, "\n")
	normalizedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		normalizedParts = append(normalizedParts, part)
	}

	normalized := strings.Join(normalizedParts, " ")
	normalized = strings.Join(strings.Fields(normalized), " ")
	normalized = strings.ReplaceAll(normalized, "|", "\\|")
	return normalized
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
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_':
			out.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r), r == '-':
			if lastDash || out.Len() == 0 {
				continue
			}

			out.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(out.String(), "-")
}
