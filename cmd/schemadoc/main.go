// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

// schemadoc generates CommonMark docs from JSON Schema.
package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
)

// cliRunner executes CLI operations with custom IO streams.
type cliRunner struct {
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	programName string
}

func main() {
	os.Exit(runWithIO(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// runWithIO executes CLI logic with custom stdin, for tests.
func runWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	programName := strings.TrimSpace(os.Args[0])
	if programName == "" {
		programName = "schemadoc"
	}

	programName = filepath.Base(programName)
	runner := cliRunner{
		programName: programName,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
	}
	options := &cliOptions{
		ModuleToSchema:   moduleToSchemaCommand{runner: &runner},
		SchemaToJSON:     schemaToJSONCommand{runner: &runner},
		SchemaToYAML:     schemaToYAMLCommand{runner: &runner},
		Config:           configCommand{runner: &runner},
		Template:         templateCommand{runner: &runner},
		ModuleToMarkdown: moduleToMarkdownCommand{runner: &runner},
		SchemaToDoc:      schemaToDocCommand{runner: &runner},
		SchemaMerge:      schemaMergeCommand{runner: &runner},
		Build:            buildCommand{runner: &runner},
	}

	parser := flags.NewParser(options, flags.HelpFlag)
	parser.Name = runner.programName
	parser.LongDescription = "schemadoc helps you build JSON Schema, docs, and example configs." +
		"You can generate docs from schema files, reflect Go types into schema," +
		"merge schema fragments, and run multi-step jobs from config."

	applyCommandLongDescriptions(parser, runner.programName)

	_, err := parser.ParseArgs(args)
	if err == nil {
		return 0
	}

	var flagErr *flags.Error
	if errors.As(err, &flagErr) {
		if flagErr.Type == flags.ErrHelp {
			writeCLIError(runner.stdout, err)
			return 0
		}

		writeCLIError(runner.stderr, err)
		return 2
	}

	writeCLIError(runner.stderr, err)
	return 1
}
