// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import "errors"

var (
	// ErrReadSchemaFile is returned when schema file loading fails.
	ErrReadSchemaFile = errors.New("read schema file")
	// ErrExecuteMarkdownTemplate is returned when markdown template execution fails.
	ErrExecuteMarkdownTemplate = errors.New("execute markdown template")
	// ErrUnknownBuiltinTemplate is returned when requested built-in template name is not registered.
	ErrUnknownBuiltinTemplate = errors.New("unknown built-in template")
	// ErrReadBuiltinTemplate is returned when built-in template file loading fails.
	ErrReadBuiltinTemplate = errors.New("read built-in template")
	// ErrDecodeSchema is returned when schema JSON decoding fails.
	ErrDecodeSchema = errors.New("decode schema")
	// ErrSchemaRootType is returned when schema root is not object or boolean.
	ErrSchemaRootType = errors.New("schema root must be object or boolean")
	// ErrParseBuiltinTemplate is returned when built-in template parsing fails.
	ErrParseBuiltinTemplate = errors.New("parse built-in template")
)
