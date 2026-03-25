// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package merge

import "errors"

var (
	// ErrExpectedObject indicates operation expects JSON object node.
	ErrExpectedObject = errors.New("expected JSON object")
	// ErrInvalidJSONPointer indicates malformed target JSON pointer.
	ErrInvalidJSONPointer = errors.New("invalid JSON pointer")
	// ErrSourcePathRequired indicates missing source schema path.
	ErrSourcePathRequired = errors.New("source path is required")
	// errSetRootExpectedAny indicates invalid root pointer assignment.
	errSetRootExpectedAny = errors.New("set root pointer: expected *any")
	// errAppendTokenNotSupported indicates unsupported array append token.
	errAppendTokenNotSupported = errors.New("append token '-' is not supported in target pointer")
	// errCloneArrayExpectedSlice indicates invalid cloned array value.
	errCloneArrayExpectedSlice = errors.New("clone merged array: expected []any")
)
