// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

//go:build !unix && !windows

package modschema

// testProcessRunning is unused on platforms without process-tree assertions.
func testProcessRunning(_ int) bool { return false }
