// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// orNone renders empty metadata values as explicit (none) marker.
func orNone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(none)"
	}

	return value
}

// mustJSONInline marshals values as single-line JSON text for markdown snippets.
func mustJSONInline(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}

	return string(data)
}

// sanitizeText trims and squashes repeated whitespace in plain text fields.
func sanitizeText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	return strings.Join(strings.Fields(text), " ")
}

// normalizeWrapWidth validates wrap width and falls back to default.
func normalizeWrapWidth(value int) int {
	if value <= 0 {
		return defaultWrapWidth
	}

	return value
}

// normalizeListMarker validates list marker and falls back to default.
func normalizeListMarker(value string) string {
	switch strings.TrimSpace(value) {
	case "*":
		return "*"
	case "-":
		return "-"
	default:
		return defaultListMarker
	}
}

// formatDescriptionMarkdown wraps plain paragraphs and preserves markdown structures.
func formatDescriptionMarkdown(text string, wrapWidth int, listMarker string) string {
	text = normalizeLineEndings(text)
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	listMarker = normalizeListMarker(listMarker)

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	paragraph := make([]string, 0, 4)
	inFence := false

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}

		joined := strings.Join(paragraph, " ")
		out = append(out, wrapParagraph(joined, wrapWidth)...)
		paragraph = paragraph[:0]
	}

	appendBlank := func() {
		if len(out) == 0 || out[len(out)-1] == "" {
			return
		}

		out = append(out, "")
	}

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, " \t")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
			out = append(out, line)
			inFence = !inFence
			continue
		}

		if inFence {
			out = append(out, line)
			continue
		}

		if trimmed == "" {
			flushParagraph()
			appendBlank()
			continue
		}

		if isMarkdownStructuredLine(line) {
			flushParagraph()
			normalized := normalizeMarkdownStructuredLine(line, listMarker)
			if shouldInsertBlankBeforeList(normalized, out) {
				appendBlank()
			}

			out = append(out, normalized)
			continue
		}

		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	return strings.Join(out, "\n")
}

// shouldInsertBlankBeforeList reports whether list line needs a blank separator from previous paragraph line.
func shouldInsertBlankBeforeList(line string, out []string) bool {
	if !isListLine(line) {
		return false
	}

	if len(out) == 0 {
		return false
	}

	previous := out[len(out)-1]
	trimmedPrevious := strings.TrimSpace(previous)
	if trimmedPrevious == "" {
		return false
	}

	if isListLine(previous) {
		return false
	}

	return !isMarkdownStructuredLine(previous)
}

// isListLine reports whether line is unordered or ordered markdown list item.
func isListLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	prefixes := []string{"- ", "* ", "+ "}
	for _, prefix := range prefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	return hasOrderedListPrefix(trimmed)
}

// isMarkdownStructuredLine reports whether line must bypass normal paragraph wrapping.
func isMarkdownStructuredLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}

	if isIndentedCodeLine(line) {
		return true
	}

	prefixes := []string{
		"#",
		">",
		"- ",
		"* ",
		"+ ",
		"|",
		"```",
		"---",
		"***",
		"___",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	return hasOrderedListPrefix(trimmed)
}

// isIndentedCodeLine reports whether line starts with markdown code indentation.
func isIndentedCodeLine(line string) bool {
	return strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")
}

// normalizeMarkdownStructuredLine normalizes list markers and indentation while preserving other structures.
func normalizeMarkdownStructuredLine(line, listMarker string) string {
	if isIndentedCodeLine(line) {
		return line
	}

	if normalized, ok := normalizeUnorderedListLine(line, listMarker); ok {
		return normalized
	}

	if normalized, ok := normalizeOrderedListLine(line); ok {
		return normalized
	}

	return line
}

// normalizeUnorderedListLine normalizes markdown unordered list marker and indentation.
func normalizeUnorderedListLine(line, listMarker string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 2 {
		return "", false
	}

	marker := trimmed[0]
	if marker != '-' && marker != '*' && marker != '+' {
		return "", false
	}

	if !strings.HasPrefix(trimmed[1:], " ") && !strings.HasPrefix(trimmed[1:], "\t") {
		return "", false
	}

	content := strings.TrimSpace(trimmed[1:])
	indentLevel := listIndentLevel(leadingIndentColumns(line))
	normalized := strings.Repeat("  ", indentLevel) + listMarker
	if content != "" {
		normalized += " " + content
	}

	return normalized, true
}

// normalizeOrderedListLine normalizes markdown ordered list indentation.
func normalizeOrderedListLine(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	index := 0
	for index < len(trimmed) && trimmed[index] >= '0' && trimmed[index] <= '9' {
		index++
	}

	if index == 0 || index+1 >= len(trimmed) {
		return "", false
	}

	marker := trimmed[index]
	if marker != '.' && marker != ')' {
		return "", false
	}

	if trimmed[index+1] != ' ' && trimmed[index+1] != '\t' {
		return "", false
	}

	content := strings.TrimSpace(trimmed[index+1:])
	indentLevel := listIndentLevel(leadingIndentColumns(line))
	normalized := strings.Repeat("  ", indentLevel) + trimmed[:index+1]
	if content != "" {
		normalized += " " + content
	}

	return normalized, true
}

// leadingIndentColumns returns visual indentation width for leading spaces and tabs.
func leadingIndentColumns(line string) int {
	columns := 0
	for _, r := range line {
		switch r {
		case ' ':
			columns++
		case '\t':
			columns += 4
		default:
			return columns
		}
	}

	return columns
}

// listIndentLevel maps raw indentation width to normalized markdown list nesting level.
func listIndentLevel(columns int) int {
	if columns <= 1 {
		return 0
	}

	return columns / 2
}

// hasOrderedListPrefix reports whether line starts with ordered list marker.
func hasOrderedListPrefix(line string) bool {
	index := 0
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		index++
	}

	if index == 0 || index+1 >= len(line) {
		return false
	}

	marker := line[index]
	if marker != '.' && marker != ')' {
		return false
	}

	return line[index+1] == ' '
}

// wrapParagraph wraps one plain paragraph to max rune width.
func wrapParagraph(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	if width <= 0 {
		return []string{strings.Join(words, " ")}
	}

	out := make([]string, 0, 2)
	current := words[0]
	currentLen := utf8.RuneCountInString(current)

	for _, word := range words[1:] {
		wordLen := utf8.RuneCountInString(word)
		if currentLen+1+wordLen <= width {
			current += " " + word
			currentLen += 1 + wordLen
			continue
		}

		out = append(out, current)
		current = word
		currentLen = wordLen
	}

	out = append(out, current)
	return out
}

// normalizeLineEndings converts CRLF/CR to LF.
func normalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

// normalizeMarkdownOutput collapses extra blank lines outside fenced blocks.
func normalizeMarkdownOutput(text string) string {
	text = normalizeLineEndings(text)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))

	inFence := false
	blankCount := 0
	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, " \t")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			out = append(out, line)
			blankCount = 0
			continue
		}

		if !inFence && trimmed == "" {
			if blankCount == 0 {
				out = append(out, "")
			}

			blankCount++
			continue
		}

		blankCount = 0
		out = append(out, line)
	}

	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// escapeInline escapes backticks in inline code markdown segments.
func escapeInline(value string) string {
	return strings.ReplaceAll(value, "`", "\\`")
}

// ensureTrailingNewline guarantees exactly one trailing newline in output.
func ensureTrailingNewline(value string) string {
	value = strings.TrimRight(value, "\n")
	return value + "\n"
}
