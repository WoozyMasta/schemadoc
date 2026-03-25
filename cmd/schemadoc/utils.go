// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/woozymasta/schemadoc"
	"go.yaml.in/yaml/v3"
)

// writeBytes writes content to stdout or file path when provided.
func writeBytes(stdout io.Writer, outputPath string, content []byte, label string) error {
	targetPath := strings.TrimSpace(outputPath)
	if targetPath == "" {
		if _, err := stdout.Write(content); err != nil {
			return fmt.Errorf("write %s to stdout: %w", label, err)
		}

		return nil
	}

	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		return fmt.Errorf("write %s file %q: %w", label, targetPath, err)
	}

	return nil
}

// writeString writes string content to stdout or file path when provided.
func writeString(stdout io.Writer, outputPath, content, label string) error {
	return writeBytes(stdout, outputPath, []byte(content), label)
}

// checkFileContent verifies that output file content matches expected bytes.
func checkFileContent(outputPath string, expected []byte, label string) error {
	targetPath := strings.TrimSpace(outputPath)
	if targetPath == "" {
		return fmt.Errorf("check %s: output path is required", label)
	}

	current, err := os.ReadFile(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check %s file %q: file does not exist", label, targetPath)
		}

		return fmt.Errorf("check %s file %q: %w", label, targetPath, err)
	}

	if !bytes.Equal(current, expected) {
		return fmt.Errorf("check %s file %q: content differs", label, targetPath)
	}

	return nil
}

// firstNonEmpty returns first non-empty trimmed value.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// writeCLIError writes a plain-text CLI error line to the selected stream.
func writeCLIError(output io.Writer, err error) {
	if err == nil {
		return
	}

	//nolint:gosec // CLI writes plain-text diagnostics to terminal streams, not HTTP responses.
	_, _ = fmt.Fprintln(output, err.Error())
}

// logf writes one plain progress line to stderr.
func (runner *cliRunner) logf(format string, args ...any) {
	if runner == nil || runner.stderr == nil {
		return
	}

	_, _ = fmt.Fprintf(runner.stderr, format+"\n", args...)
}

// readSchemaInput reads schema bytes from file or stdin and returns source marker.
func readSchemaInput(path string, stdin io.Reader) ([]byte, string, error) {
	path = strings.TrimSpace(path)
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("read schema file %q: %w", path, err)
		}

		normalized, err := normalizeSchemaInput(data)
		if err != nil {
			return nil, "", fmt.Errorf("decode schema file %q: %w", path, err)
		}

		return normalized, path, nil
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, "", fmt.Errorf("read schema from stdin: %w", err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, "", errors.New("read schema from stdin: empty input")
	}

	normalized, err := normalizeSchemaInput(data)
	if err != nil {
		return nil, "", fmt.Errorf("decode schema from stdin: %w", err)
	}

	return normalized, "(stdin)", nil
}

// normalizeSchemaInput converts JSON or YAML schema payload to JSON bytes.
func normalizeSchemaInput(content []byte) ([]byte, error) {
	if json.Valid(content) {
		return content, nil
	}

	var value any
	if err := yaml.Unmarshal(content, &value); err != nil {
		return nil, err
	}

	normalized := normalizeYAMLNode(value)
	return json.Marshal(normalized)
}

// parseExampleMode parses textual example mode.
func parseExampleMode(mode string) (schemadoc.ExampleMode, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "all":
		return schemadoc.ExampleModeAll, nil
	case "required":
		return schemadoc.ExampleModeRequired, nil
	default:
		return "", fmt.Errorf("unsupported example mode %q", mode)
	}
}

// parseExampleFormat parses textual example format.
func parseExampleFormat(format string) (schemadoc.ExampleFormat, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return schemadoc.ExampleFormatJSON, nil
	case "yaml":
		return schemadoc.ExampleFormatYAML, nil
	default:
		return "", fmt.Errorf("unsupported example format %q", format)
	}
}
