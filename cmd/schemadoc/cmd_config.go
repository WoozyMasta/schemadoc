// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"strings"

	"github.com/woozymasta/schemadoc"
)

// Execute runs config subcommand.
func (command *configCommand) Execute(_ []string) error {
	return command.runner.runConfigExample(
		command.Mode,
		string(schemadoc.ExampleFormatYAML),
		command.Args.Output,
	)
}

// runConfigExample renders config example output.
func (runner *cliRunner) runConfigExample(modeRaw, formatRaw, outputPath string) error {
	runner.logf(
		"config: mode=%s format=%s output=%s",
		firstNonEmpty(strings.TrimSpace(modeRaw), "all"),
		firstNonEmpty(strings.TrimSpace(formatRaw), "yaml"),
		firstNonEmpty(strings.TrimSpace(outputPath), "-"),
	)
	mode, err := parseExampleMode(modeRaw)
	if err != nil {
		return err
	}

	format, err := parseExampleFormat(formatRaw)
	if err != nil {
		return err
	}

	content, err := renderBuildConfigExample(mode, format)
	if err != nil {
		return err
	}

	if err := writeBytes(runner.stdout, outputPath, content, "example"); err != nil {
		return err
	}

	runner.logf("config: ok")
	return nil
}
