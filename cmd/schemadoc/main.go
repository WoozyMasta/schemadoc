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
	"time"

	"github.com/woozymasta/flags"
)

var (
	Version    = "dev"
	Commit     = "unknown"
	BuildTime  time.Time
	URL        = "https://github.com/woozymasta/schemadoc"
	_buildTime string
)

// cliRunner executes CLI operations with custom IO streams.
type cliRunner struct {
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	programName string
}

func init() {
	if _buildTime == "" {
		return
	}

	parsed, err := time.Parse(time.RFC3339, _buildTime)
	if err != nil {
		return
	}

	BuildTime = parsed.UTC()
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

	parser := flags.NewParser(
		options,
		flags.HelpFlag|
			flags.HelpCommand|
			flags.VersionCommand|
			flags.CompletionCommand|
			flags.DocsCommand|
			flags.KeepDescriptionWhitespace|
			flags.PrintHelpOnInputErrors|
			flags.ShowRepeatableInHelp|
			flags.DetectShellFlagStyle|
			flags.DetectShellEnvStyle,
	)
	parser.Name = runner.programName

	fields := flags.VersionFieldsCore
	if BuildTime.IsZero() {
		fields &^= flags.VersionFieldBuilt
	}

	parser.SetVersionFields(fields)
	parser.SetVersionInfo(flags.VersionInfo{
		File:         os.Args[0],
		Version:      Version,
		Revision:     Commit,
		RevisionTime: BuildTime,
		URL:          URL,
	})

	if err := parser.EnsureBuiltinCommands(); err != nil {
		writeCLIError(runner.stderr, err)
		return 1
	}

	configureDescriptions(parser, options)

	_, err := parser.ParseArgs(args)
	if err == nil {
		return 0
	}

	var flagErr *flags.Error
	if errors.As(err, &flagErr) {
		if flagErr.Type == flags.ErrHelp || flagErr.Type == flags.ErrVersion {
			writeCLIError(runner.stdout, err)
			return 0
		}

		writeCLIError(runner.stderr, err)
		return 2
	}

	writeCLIError(runner.stderr, err)
	return 1
}
