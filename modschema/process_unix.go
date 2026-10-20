// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

//go:build unix

package modschema

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// runProcessTree cancels the command's own process group, including go run helpers.
// Target code can escape a process group; this is lifecycle control, not a sandbox.
func runProcessTree(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}

		return err
	}

	err := command.Run()
	if command.Process == nil {
		return err
	}

	// The root may exit before cancellation while descendants still hold its pipes.
	cleanupErr := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	if errors.Is(cleanupErr, syscall.ESRCH) {
		cleanupErr = nil
	}

	return errors.Join(err, cleanupErr)
}
