// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import (
	"errors"

	"golang.org/x/sys/windows"
)

// testProcessRunning checks the original process without signalling an unrelated PID.
func testProcessRunning(pid int) bool {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return !errors.Is(err, windows.ERROR_INVALID_PARAMETER)
	}

	defer windows.CloseHandle(process)
	status, err := windows.WaitForSingleObject(process, 0)

	return err != nil || status == uint32(windows.WAIT_TIMEOUT)
}
