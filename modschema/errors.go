// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import "errors"

var (
	// ErrInvalidRootID reports an invalid canonical root schema identifier.
	ErrInvalidRootID = errors.New("invalid root schema id")

	// ErrMainPackageUnsupported reports unsupported reflection from package main.
	ErrMainPackageUnsupported = errors.New("package main is not supported for module reflection")
)
