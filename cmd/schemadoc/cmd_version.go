// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"fmt"
	"os"
	"time"
)

var (
	// Version stores CLI version value, overridden by linker flags.
	Version = "dev"

	// Commit stores git revision value, overridden by linker flags.
	Commit = "unknown"

	// BuildTime stores build timestamp value, overridden by linker flags.
	BuildTime = time.Unix(0, 0).UTC()

	// URL stores project URL shown by version command.
	URL = "https://github.com/woozymasta/schemadoc"

	// _buildTime stores raw build time input for init-time parsing.
	_buildTime string
)

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

// printVersionInfo prints CLI build metadata.
func printVersionInfo() {
	fmt.Printf(`url:      %s
file:     %s
version:  %s
commit:   %s
built:    %s
`, URL, os.Args[0], Version, Commit, BuildTime)
}

// Execute runs version subcommand.
func (command *versionCommand) Execute(_ []string) error {
	printVersionInfo()
	return nil
}
