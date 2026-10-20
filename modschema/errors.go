// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package modschema

import "errors"

var (
	// ErrInvalidModuleFile reports missing or invalid Go module syntax.
	ErrInvalidModuleFile = errors.New("invalid Go module file")

	// ErrInvalidTypeNamerMode reports an unsupported schema definition naming mode.
	ErrInvalidTypeNamerMode = errors.New("invalid type namer mode")

	// ErrGeneratorClosed reports generation attempted after session cleanup.
	ErrGeneratorClosed = errors.New("schema generator is closed")

	// ErrInvalidCommentMode reports an unsupported Go comment mode.
	ErrInvalidCommentMode = errors.New("invalid comment mode")

	// ErrInvalidRootID reports an invalid canonical root schema identifier.
	ErrInvalidRootID = errors.New("invalid root schema id")

	// ErrMainPackageUnsupported reports unsupported reflection from package main.
	ErrMainPackageUnsupported = errors.New("package main is not supported for module reflection")
)
