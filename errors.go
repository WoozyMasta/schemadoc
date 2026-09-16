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
	// ErrParseTemplateMetadata is returned when template metadata is invalid.
	ErrParseTemplateMetadata = errors.New("parse template metadata")
	// ErrUnknownTemplatePostProcessor is returned for an unregistered processor.
	ErrUnknownTemplatePostProcessor = errors.New("unknown template post-processor")
	// ErrDuplicateTemplatePostProcessor is returned for a repeated processor.
	ErrDuplicateTemplatePostProcessor = errors.New("duplicate template post-processor")
	// ErrInvalidTemplateExtension is returned for an invalid output extension.
	ErrInvalidTemplateExtension = errors.New("invalid template output extension")
	// ErrUnknownExampleMode is returned when example generation mode is not supported.
	ErrUnknownExampleMode = errors.New("unknown example mode")
	// ErrUnknownExampleFormat is returned when example generation format is not supported.
	ErrUnknownExampleFormat = errors.New("unknown example format")
	// ErrEncodeExampleJSON is returned when generated example JSON encoding fails.
	ErrEncodeExampleJSON = errors.New("encode example json")
	// ErrEncodeExampleYAML is returned when generated example YAML encoding fails.
	ErrEncodeExampleYAML = errors.New("encode example yaml")
	// ErrExternalSchemaReference is returned when a reference leaves the document.
	ErrExternalSchemaReference = errors.New("external schema reference")
	// ErrUnresolvedSchemaReference is returned when a local reference target is missing.
	ErrUnresolvedSchemaReference = errors.New("unresolved schema reference")
	// ErrInvalidSchemaPointer is returned when a local reference is not a JSON Pointer.
	ErrInvalidSchemaPointer = errors.New("invalid schema JSON pointer")
	// ErrSchemaReferenceCycle is returned when local references form a cycle.
	ErrSchemaReferenceCycle = errors.New("schema reference cycle")
	// ErrMaterializationUnsatisfiable is returned when no valid example exists.
	ErrMaterializationUnsatisfiable = errors.New("materialization is unsatisfiable")
	// ErrUnsupportedMaterialization is returned when generation lacks a safe strategy.
	ErrUnsupportedMaterialization = errors.New("materialization is unsupported")
	// ErrUnsupportedRequiredSemantics is returned for required constraints without a usable schema.
	ErrUnsupportedRequiredSemantics = errors.New("required semantics are unsupported")
	// ErrGeneratedValueInvalid is returned when synthesis produces an invalid value.
	ErrGeneratedValueInvalid = errors.New("generated value is invalid")
)
