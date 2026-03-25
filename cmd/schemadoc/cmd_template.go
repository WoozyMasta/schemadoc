// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"fmt"
	"strings"

	"github.com/woozymasta/schemadoc"
)

// Execute runs template subcommand.
func (command *templateCommand) Execute(_ []string) error {
	return command.runner.runTemplate(command.TemplateFlags.TemplateName, command.Args.Output)
}

// runTemplate writes selected built-in template to stdout or file.
func (runner *cliRunner) runTemplate(templateName, outputPath string) error {
	runner.logf(
		"template: name=%s output=%s",
		templateName,
		firstNonEmpty(strings.TrimSpace(outputPath), "-"),
	)
	tpl, err := schemadoc.BuiltinTemplate(templateName)
	if err != nil {
		return fmt.Errorf("load built-in template %q: %w", templateName, err)
	}

	if err := writeString(runner.stdout, outputPath, tpl, "template"); err != nil {
		return err
	}

	runner.logf("template: ok")
	return nil
}
