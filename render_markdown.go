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

// formatDescriptionMarkdown normalizes Markdown embedded in schema descriptions.
func formatDescriptionMarkdown(text string, wrapWidth int, listMarker string) string {
	text = normalizeLineEndings(text)
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	paragraph := make([]string, 0, 4)
	inFence := false
	fenceMarker := byte(0)
	fenceWidth := 0
	previousListIndent := -1
	previousListLevel := 0
	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		out = append(out, wrapParagraph(strings.Join(paragraph, " "), wrapWidth)...)
		paragraph = paragraph[:0]
	}
	appendBlank := func() {
		if len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, "")
		}
	}

	for index := 0; index < len(lines); index++ {
		rawLine := lines[index]
		line := strings.TrimRight(rawLine, "\t")
		trimmed := strings.TrimSpace(line)
		if marker, width, ok := markdownFence(trimmed); ok {
			flushParagraph()
			if !inFence {
				inFence, fenceMarker, fenceWidth = true, marker, width
				out = append(out, line)
				continue
			}
			if marker == fenceMarker && width >= fenceWidth {
				inFence = false
				out = append(out, line)
				continue
			}
		}

		if inFence {
			out = append(out, line)
			continue
		}

		if code, next, ok := collectIndentedCodeBlock(lines, index); ok {
			flushParagraph()
			codePrefix := ""
			if previousListIndent >= 0 && leadingIndentColumns(line) > previousListIndent {
				codePrefix = strings.Repeat("  ", previousListLevel+1)
			}
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			fence := markdownCodeFence(code)
			out = append(out, codePrefix+fence)
			for _, codeLine := range code {
				out = append(out, codePrefix+codeLine)
			}
			out = append(out, codePrefix+fence)
			if next < len(lines) && strings.TrimSpace(lines[next]) != "" {
				out = append(out, "")
			}
			index = next - 1
			continue
		}

		if trimmed == "" {
			flushParagraph()
			appendBlank()
			continue
		}

		if normalized, indent, level, ok := normalizeMarkdownListLine(
			line,
			listMarker,
			previousListIndent,
			previousListLevel,
		); ok {
			flushParagraph()
			if len(out) > 0 && out[len(out)-1] != "" && !isMarkdownListLine(out[len(out)-1]) {
				appendBlank()
			}
			out = append(out, normalized)
			previousListIndent, previousListLevel = indent, level
			continue
		}

		previousListIndent = -1
		if isMarkdownBlockLine(line) {
			flushParagraph()
			out = append(out, line)
			continue
		}

		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	return strings.Join(out, "\n")
}

// isMarkdownBlockLine reports whether line starts a block preserved verbatim.
func isMarkdownBlockLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ">") ||
		strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "---") ||
		strings.HasPrefix(trimmed, "***") || strings.HasPrefix(trimmed, "___") ||
		strings.HasPrefix(trimmed, "<")
}

// isMarkdownListLine reports whether line starts an ordered or unordered list item.
func isMarkdownListLine(line string) bool {
	_, ok := parseMarkdownListLine(line)
	return ok
}

// markdownListLine is a parsed Markdown list item with source indentation.
type markdownListLine struct {
	marker string
	text   string
	indent int
}

// parseMarkdownListLine parses an ordered or unordered Markdown list item.
func parseMarkdownListLine(line string) (markdownListLine, bool) {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') &&
		(trimmed[1] == ' ' || trimmed[1] == '\t') {
		return markdownListLine{
			indent: len(line) - len(strings.TrimLeft(line, " \t")),
			marker: trimmed[:1],
			text:   strings.TrimSpace(trimmed[1:]),
		}, true
	}
	index := 0
	for index < len(trimmed) && trimmed[index] >= '0' && trimmed[index] <= '9' {
		index++
	}
	if index == 0 || index+1 >= len(trimmed) || (trimmed[index] != '.' && trimmed[index] != ')') ||
		(trimmed[index+1] != ' ' && trimmed[index+1] != '\t') {
		return markdownListLine{}, false
	}
	return markdownListLine{
		indent: len(line) - len(strings.TrimLeft(line, " \t")),
		marker: trimmed[:index+1],
		text:   strings.TrimSpace(trimmed[index+1:]),
	}, true
}

// normalizeMarkdownListLine preserves nested lists and treats Go-doc two-space lists as root lists.
func normalizeMarkdownListLine(
	line, listMarker string,
	previousIndent, previousLevel int,
) (string, int, int, bool) {
	item, ok := parseMarkdownListLine(line)
	if !ok {
		return "", 0, 0, false
	}
	level := 0
	switch {
	case item.indent >= 4:
		level = item.indent / 4
	case item.indent == 2 && previousIndent == 0 && previousLevel == 0:
		level = 1
	}
	marker := item.marker
	if marker == "-" || marker == "*" || marker == "+" {
		marker = normalizeListMarker(listMarker)
	}
	return strings.Repeat("  ", level) + marker + " " + item.text, item.indent, level, true
}

// collectIndentedCodeBlock recognizes a Go-doc preformatted block.
func collectIndentedCodeBlock(lines []string, start int) ([]string, int, bool) {
	indent := leadingIndentColumns(lines[start])
	if indent < 2 || isMarkdownListLine(lines[start]) {
		return nil, start, false
	}

	end := start
	code := make([]string, 0, 4)
	for end < len(lines) {
		line := strings.TrimRight(lines[end], "\t")
		if strings.TrimSpace(line) == "" {
			code = append(code, "")
			end++
			continue
		}
		if leadingIndentColumns(line) < indent {
			break
		}
		code = append(code, trimLeadingIndent(line, indent))
		end++
	}
	code = trimTrailingBlankLines(code)
	if len(code) == 0 {
		return nil, start, false
	}

	return code, end, true
}

// leadingIndentColumns returns the visual width of leading spaces and tabs.
func leadingIndentColumns(line string) int {
	indent := 0
	for _, char := range line {
		switch char {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			return indent
		}
	}
	return indent
}

// trimLeadingIndent removes up to columns of leading indentation from line.
func trimLeadingIndent(line string, columns int) string {
	for columns > 0 && len(line) > 0 {
		switch line[0] {
		case ' ':
			line = line[1:]
			columns--
		case '\t':
			line = line[1:]
			columns -= 4
		default:
			return line
		}
	}
	return line
}

// trimTrailingBlankLines removes blank lines at the end of a block.
func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// markdownCodeFence returns a fence that cannot be closed by code content.
func markdownCodeFence(lines []string) string {
	width := 3
	for _, line := range lines {
		for index := 0; index < len(line); {
			if line[index] != '`' {
				index++
				continue
			}
			end := index
			for end < len(line) && line[end] == '`' {
				end++
			}
			if end-index >= width {
				width = end - index + 1
			}
			index = end
		}
	}
	return strings.Repeat("`", width)
}

// markdownFence returns the marker and width of a Markdown fence line.
func markdownFence(line string) (byte, int, bool) {
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return 0, 0, false
	}
	marker := line[0]
	width := 0
	for width < len(line) && line[width] == marker {
		width++
	}
	return marker, width, width >= 3
}

// wrapParagraph wraps plain text at width Unicode code points.
func wrapParagraph(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	if width <= 0 {
		return []string{strings.Join(words, " ")}
	}
	out := make([]string, 0, 2)
	var line strings.Builder
	line.WriteString(words[0])
	lineLen := utf8.RuneCountInString(words[0])
	for _, word := range words[1:] {
		wordLen := utf8.RuneCountInString(word)
		if lineLen+1+wordLen <= width {
			line.WriteByte(' ')
			line.WriteString(word)
			lineLen += 1 + wordLen
			continue
		}
		out = append(out, line.String())
		line.Reset()
		line.WriteString(word)
		lineLen = wordLen
	}
	return append(out, line.String())
}

// normalizeLineEndings converts CRLF/CR to LF.
func normalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

// normalizeMarkdownSpacing collapses extra blank lines outside fenced blocks.
func normalizeMarkdownSpacing(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	started := false
	writeLine := func(line string) {
		if started {
			out.WriteByte('\n')
		}
		out.WriteString(line)
		started = true
	}

	inFence := false
	fenceMarker := byte(0)
	fenceWidth := 0
	needsBlankAfterFence := false
	blankCount := 0
	for start := 0; ; {
		lineEnd := strings.IndexByte(text[start:], '\n')
		rawLine := text[start:]
		if lineEnd >= 0 {
			rawLine = text[start : start+lineEnd]
		}

		line := rawLine
		trimmed := strings.TrimSpace(line)

		if marker, width, ok := markdownFence(trimmed); ok {
			switch {
			case !inFence:
				inFence, fenceMarker, fenceWidth = true, marker, width
				writeLine(line)
				needsBlankAfterFence = false
				blankCount = 0
			case marker == fenceMarker && width >= fenceWidth:
				inFence = false
				writeLine(line)
				needsBlankAfterFence = true
				blankCount = 0
			default:
				writeLine(line)
			}
		} else {
			if !inFence && trimmed == "" {
				if blankCount == 0 {
					writeLine("")
				}
				blankCount++
				needsBlankAfterFence = false
			} else {
				if !inFence && needsBlankAfterFence {
					writeLine("")
					needsBlankAfterFence = false
				}
				blankCount = 0
				writeLine(line)
			}
		}

		if lineEnd < 0 {
			break
		}
		start += lineEnd + 1
	}

	return strings.TrimRight(out.String(), "\n")
}

// trimTrailingWhitespace removes trailing spaces and tabs from each line.
func trimTrailingWhitespace(text string) string {
	var out strings.Builder
	trimmed := false
	lineStart := 0

	for lineEnd := 0; lineEnd <= len(text); lineEnd++ {
		if lineEnd < len(text) && text[lineEnd] != '\n' {
			continue
		}

		contentEnd := lineEnd
		for contentEnd > lineStart {
			last := text[contentEnd-1]
			if last != ' ' && last != '\t' {
				break
			}
			contentEnd--
		}

		if !trimmed {
			if contentEnd == lineEnd {
				lineStart = lineEnd + 1
				continue
			}

			out.Grow(len(text))
			out.WriteString(text[:contentEnd])
			trimmed = true
		} else {
			out.WriteString(text[lineStart:contentEnd])
		}

		if lineEnd < len(text) {
			out.WriteByte('\n')
		}
		lineStart = lineEnd + 1
	}

	if !trimmed {
		return text
	}

	return out.String()
}

// escapeInline escapes backticks in inline code markdown segments.
func escapeInline(value string) string {
	return strings.ReplaceAll(value, "`", "\\`")
}

// ensureTrailingNewline guarantees exactly one trailing newline in output.
func ensureTrailingNewline(value string) string {
	if strings.HasSuffix(value, "\n") &&
		(value == "\n" || !strings.HasSuffix(value[:len(value)-1], "\n")) {
		return value
	}

	value = strings.TrimRight(value, "\n")
	return value + "\n"
}
