// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package common

// Config is the first external definition.
type Config struct {
	A    string  `json:"a"`              // Identifies the first package.
	Next *Config `json:"next,omitempty"` // Reuses the same named type through a pointer.
}
