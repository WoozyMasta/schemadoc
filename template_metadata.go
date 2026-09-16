// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

const templateMetadataPrefix = "schemadoc:"

// templateMetadata declares output details and ordered post-processors.
type templateMetadata struct {
	Output      *templateOutputMetadata `yaml:"output"`
	Postprocess []string                `yaml:"postprocess"`
}

// templateOutputMetadata contains metadata for derived output paths.
type templateOutputMetadata struct {
	Extension *string `yaml:"extension"`
}

// templatePostProcessor transforms rendered template output deterministically.
type templatePostProcessor func(string) (string, error)

var templatePostProcessorRegistry = map[string]templatePostProcessor{
	"normalize-line-endings": func(value string) (string, error) {
		return normalizeLineEndings(value), nil
	},
	"normalize-markdown-spacing": func(value string) (string, error) {
		return normalizeMarkdownSpacing(value), nil
	},
	"trim-trailing-whitespace": func(value string) (string, error) {
		return trimTrailingWhitespace(value), nil
	},
	"trailing-newline": func(value string) (string, error) {
		return ensureTrailingNewline(value), nil
	},
}

// parseTemplateMetadata parses metadata from the first non-output Go comment.
func parseTemplateMetadata(source string) (templateMetadata, error) {
	comment, found, err := leadingTemplateComment(source)
	if err != nil || !found {
		return templateMetadata{}, err
	}

	comment = strings.TrimSpace(comment)
	if !strings.HasPrefix(comment, templateMetadataPrefix) {
		return templateMetadata{}, nil
	}

	if strings.TrimSpace(strings.TrimPrefix(comment, templateMetadataPrefix)) == "" {
		return templateMetadata{}, fmt.Errorf("%w: empty metadata", ErrParseTemplateMetadata)
	}

	var document struct {
		SchemaDoc *templateMetadata `yaml:"schemadoc"`
	}
	decoder := yaml.NewDecoder(strings.NewReader(comment))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return templateMetadata{}, fmt.Errorf("%w: %w", ErrParseTemplateMetadata, err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return templateMetadata{}, fmt.Errorf("%w: multiple documents", ErrParseTemplateMetadata)
		}

		return templateMetadata{}, fmt.Errorf("%w: %w", ErrParseTemplateMetadata, err)
	}

	if document.SchemaDoc == nil {
		return templateMetadata{}, fmt.Errorf("%w: missing schemadoc mapping", ErrParseTemplateMetadata)
	}

	metadata := *document.SchemaDoc
	if err := validateTemplateMetadata(&metadata); err != nil {
		return templateMetadata{}, err
	}

	return metadata, nil
}

// leadingTemplateComment returns the first non-output Go template comment.
func leadingTemplateComment(source string) (string, bool, error) {
	trimmed := strings.TrimLeft(source, " \t\r\n")
	if !strings.HasPrefix(trimmed, "{{/*") {
		return "", false, nil
	}

	const openingLength = len("{{/*")
	closingOffset := strings.Index(trimmed[openingLength:], "*/")
	if closingOffset < 0 {
		return "", true, fmt.Errorf("%w: unterminated leading comment", ErrParseTemplateMetadata)
	}

	commentEnd := openingLength + closingOffset + len("*/")
	closing := trimmed[commentEnd:]
	if !validTemplateCommentClosing(closing) {
		return "", true, fmt.Errorf("%w: invalid comment closing", ErrParseTemplateMetadata)
	}

	return trimmed[openingLength : openingLength+closingOffset], true, nil
}

// validTemplateCommentClosing accepts Go template closing delimiters
// with an optional whitespace trim marker.
func validTemplateCommentClosing(value string) bool {
	if strings.HasPrefix(value, "}}") {
		return true
	}

	return len(value) >= 3 &&
		strings.ContainsRune(" \t\r\n", rune(value[0])) &&
		strings.HasPrefix(value[1:], "-}}")
}

// validateTemplateMetadata validates extension and processor declarations.
func validateTemplateMetadata(metadata *templateMetadata) error {
	if metadata == nil {
		return fmt.Errorf("%w: nil metadata", ErrParseTemplateMetadata)
	}

	if metadata.Output != nil && metadata.Output.Extension != nil {
		extension := strings.TrimSpace(*metadata.Output.Extension)
		if !validTemplateExtension(extension) {
			return fmt.Errorf("%w: %q", ErrInvalidTemplateExtension, *metadata.Output.Extension)
		}

		*metadata.Output.Extension = extension
	}

	seen := make(map[string]struct{}, len(metadata.Postprocess))
	for index, name := range metadata.Postprocess {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("%w: empty processor at index %d", ErrParseTemplateMetadata, index)
		}
		if _, exists := templatePostProcessorRegistry[name]; !exists {
			return fmt.Errorf("%w %q", ErrUnknownTemplatePostProcessor, name)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("%w %q", ErrDuplicateTemplatePostProcessor, name)
		}

		seen[name] = struct{}{}
		metadata.Postprocess[index] = name
	}

	return nil
}

// validTemplateExtension reports whether extension is safe for a file suffix.
func validTemplateExtension(extension string) bool {
	if len(extension) < 2 || extension[0] != '.' || extension == ".." {
		return false
	}

	return !strings.ContainsAny(extension, "\\/ \t\r\n")
}

// applyTemplatePostProcessors executes declared processors in declaration order.
func applyTemplatePostProcessors(value string, metadata templateMetadata) (string, error) {
	for _, name := range metadata.Postprocess {
		processor, ok := templatePostProcessorRegistry[name]
		if !ok {
			return "", fmt.Errorf("%w %q", ErrUnknownTemplatePostProcessor, name)
		}

		processed, err := processor(value)
		if err != nil {
			return "", fmt.Errorf("apply template post-processor %q: %w", name, err)
		}

		value = processed
	}

	return value, nil
}
