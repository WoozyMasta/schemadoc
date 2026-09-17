// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// schemaReferenceErrorKind identifies a local-reference resolution failure.
type schemaReferenceErrorKind string

const (
	schemaReferenceExternal    schemaReferenceErrorKind = "external"
	schemaReferenceUnresolved  schemaReferenceErrorKind = "unresolved"
	schemaReferenceInvalid     schemaReferenceErrorKind = "invalid-pointer"
	schemaReferenceUnsupported schemaReferenceErrorKind = "unsupported"
	schemaReferenceCycle       schemaReferenceErrorKind = "cycle"
)

// schemaReferenceError carries machine-readable reference failure details.
type schemaReferenceError struct {
	Cause     error
	Kind      schemaReferenceErrorKind
	Reference string
	Path      schemaPath
}

// Error implements error.
func (err *schemaReferenceError) Error() string {
	if err == nil {
		return "<nil>"
	}

	message := "resolve schema reference"

	switch err.Kind {
	case schemaReferenceExternal:
		message = "external schema reference is unsupported"
	case schemaReferenceUnresolved:
		message = "schema reference target was not found"
	case schemaReferenceInvalid:
		message = "schema reference contains an invalid JSON pointer"
	case schemaReferenceUnsupported:
		message = "schema reference form is unsupported"
	case schemaReferenceCycle:
		message = "schema reference cycle detected"
	}

	if err.Reference != "" {
		message += " " + strconv.Quote(err.Reference)
	}
	if err.Cause != nil {
		message += ": " + err.Cause.Error()
	}

	return message
}

// Is maps structured failures to public sentinel errors.
func (err *schemaReferenceError) Is(target error) bool {
	switch target {
	case ErrExternalSchemaReference:
		return err.Kind == schemaReferenceExternal
	case ErrUnresolvedSchemaReference:
		return err.Kind == schemaReferenceUnresolved
	case ErrInvalidSchemaPointer:
		return err.Kind == schemaReferenceInvalid
	case ErrUnsupportedSchemaReference:
		return err.Kind == schemaReferenceUnsupported
	case ErrSchemaReferenceCycle:
		return err.Kind == schemaReferenceCycle
	default:
		return false
	}
}

// localSchemaResolver resolves local JSON Pointer references in one document.
type localSchemaResolver struct {
	root any
}

// newLocalSchemaResolver creates a resolver without network access.
func newLocalSchemaResolver(root any) localSchemaResolver {
	return localSchemaResolver{
		root: root,
	}
}

// lookup resolves one local JSON Pointer without following a target $ref.
func (resolver localSchemaResolver) lookup(ref string) (schemaValue, error) {
	ref = strings.TrimSpace(ref)
	path, err := parseLocalJSONPointer(ref)
	if err != nil {
		return schemaValue{}, err
	}

	current := resolver.root
	for _, segment := range path.segments {
		switch typed := current.(type) {
		case map[string]any:
			value, ok := typed[segment.value]
			if !ok {
				return schemaValue{}, &schemaReferenceError{
					Kind:      schemaReferenceUnresolved,
					Reference: ref,
					Path:      path,
				}
			}
			current = value

		case []any:
			index, indexErr := jsonPointerArrayIndex(segment.value, len(typed))
			if indexErr != nil {
				return schemaValue{}, &schemaReferenceError{
					Kind:      schemaReferenceUnresolved,
					Reference: ref,
					Path:      path,
					Cause:     indexErr,
				}
			}
			current = typed[index]

		default:
			return schemaValue{}, &schemaReferenceError{
				Kind:      schemaReferenceUnresolved,
				Reference: ref,
				Path:      path,
			}
		}
	}

	value, ok := toSchemaValue(current)
	if !ok {
		return schemaValue{}, &schemaReferenceError{
			Kind:      schemaReferenceUnresolved,
			Reference: ref,
			Path:      path,
			Cause:     fmt.Errorf("target has type %T", current),
		}
	}

	return value, nil
}

// parseLocalJSONPointer validates and decodes a local JSON Pointer reference.
func parseLocalJSONPointer(ref string) (schemaPath, error) {
	if ref == "#" {
		return schemaPath{}, nil
	}

	if !strings.HasPrefix(ref, "#/") {
		if strings.HasPrefix(ref, "#") {
			return schemaPath{}, &schemaReferenceError{
				Kind:      schemaReferenceUnsupported,
				Reference: ref,
			}
		}

		return schemaPath{}, &schemaReferenceError{
			Kind:      schemaReferenceExternal,
			Reference: ref,
		}
	}

	fragment, err := url.PathUnescape(strings.TrimPrefix(ref, "#"))
	if err != nil || !utf8.ValidString(fragment) {
		return schemaPath{}, &schemaReferenceError{
			Kind:      schemaReferenceInvalid,
			Reference: ref,
			Cause:     err,
		}
	}

	path := schemaPath{}
	for rawToken := range strings.SplitSeq(strings.TrimPrefix(fragment, "/"), "/") {
		token, err := decodeJSONPointerToken(rawToken)
		if err != nil {
			return schemaPath{}, &schemaReferenceError{
				Kind:      schemaReferenceInvalid,
				Reference: ref,
				Cause:     err,
			}
		}

		path = path.appendProperty(token)
	}

	return path, nil
}

// decodeJSONPointerToken validates and decodes one RFC 6901 token.
func decodeJSONPointerToken(token string) (string, error) {
	if !strings.Contains(token, "~") {
		return token, nil
	}

	var builder strings.Builder
	for index := 0; index < len(token); index++ {
		if token[index] != '~' {
			builder.WriteByte(token[index])
			continue
		}

		if index+1 >= len(token) || (token[index+1] != '0' && token[index+1] != '1') {
			return "", fmt.Errorf("invalid escape at byte %d", index)
		}

		if token[index+1] == '0' {
			builder.WriteByte('~')
		} else {
			builder.WriteByte('/')
		}
		index++
	}

	return builder.String(), nil
}

// jsonPointerArrayIndex parses the strict array-index form used by JSON Pointer.
func jsonPointerArrayIndex(token string, length int) (int, error) {
	if token == "" || (len(token) > 1 && token[0] == '0') {
		return 0, fmt.Errorf("invalid array index %q", token)
	}

	index, err := strconv.Atoi(token)
	if err != nil || index < 0 || index >= length {
		return 0, fmt.Errorf("array index %q is out of range", token)
	}

	return index, nil
}
