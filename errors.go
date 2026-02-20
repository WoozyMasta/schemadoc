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
	// ErrUnknownExampleMode is returned when example generation mode is not supported.
	ErrUnknownExampleMode = errors.New("unknown example mode")
	// ErrUnknownExampleFormat is returned when example generation format is not supported.
	ErrUnknownExampleFormat = errors.New("unknown example format")
	// ErrEncodeExampleJSON is returned when generated example JSON encoding fails.
	ErrEncodeExampleJSON = errors.New("encode example json")
	// ErrEncodeExampleYAML is returned when generated example YAML encoding fails.
	ErrEncodeExampleYAML = errors.New("encode example yaml")
)
