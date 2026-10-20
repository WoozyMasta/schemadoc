// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import "errors"

var (
	// errSuspendedThreadNotFound reports a Windows startup failure before target execution.
	errSuspendedThreadNotFound = errors.New("suspended subprocess thread not found")

	// errProcessTreeCleanupTimeout reports incomplete cleanup of a terminated process tree.
	errProcessTreeCleanupTimeout = errors.New("subprocess tree cleanup timed out")
)
