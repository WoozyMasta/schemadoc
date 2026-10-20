// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

//go:build !unix && !windows

package modschema

import "os/exec"

// runProcessTree uses standard cancellation on platforms without process-tree control.
func runProcessTree(command *exec.Cmd) error {
	return command.Run()
}
