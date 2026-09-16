// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"strconv"
	"strings"
	"unicode"
)

// schemaPathSegmentKind distinguishes semantic path segments that may have the
// same textual representation.
type schemaPathSegmentKind uint8

const (
	schemaPathProperty schemaPathSegmentKind = iota + 1
	schemaPathArrayItem
)

// schemaPath is a root-relative path represented as semantic segments.
type schemaPath struct {
	segments []schemaPathSegment
}

// schemaPathSegment is one semantic component of a schema path.
type schemaPathSegment struct {
	value string
	kind  schemaPathSegmentKind
}

// appendProperty returns a path extended with one property name.
func (path schemaPath) appendProperty(name string) schemaPath {
	return path.append(schemaPathSegment{kind: schemaPathProperty, value: name})
}

// appendArrayItem returns a path extended with an array item marker.
func (path schemaPath) appendArrayItem() schemaPath {
	return path.append(schemaPathSegment{kind: schemaPathArrayItem})
}

// append returns a path extended with one segment without mutating its input.
func (path schemaPath) append(segment schemaPathSegment) schemaPath {
	segments := make([]schemaPathSegment, len(path.segments), len(path.segments)+1)
	copy(segments, path.segments)
	segments = append(segments, segment)

	return schemaPath{segments: segments}
}

// equal reports whether two paths contain the same typed segments.
func (path schemaPath) equal(other schemaPath) bool {
	return path.key() == other.key()
}

// key returns an unambiguous identity key for maps and sets.
func (path schemaPath) key() string {
	var builder strings.Builder
	for _, segment := range path.segments {
		writeSchemaPathKey(&builder, segment)
	}

	return builder.String()
}

// writeSchemaPathKey appends one segment to an unambiguous path key.
func writeSchemaPathKey(builder *strings.Builder, segment schemaPathSegment) {
	builder.WriteByte(byte(segment.kind))
	builder.WriteByte(':')
	builder.WriteString(strconv.Itoa(len(segment.value)))
	builder.WriteByte(':')
	builder.WriteString(segment.value)
	builder.WriteByte(';')
}

// display converts a structured path to a readable, non-authoritative form.
func (path schemaPath) display() string {
	var builder strings.Builder
	for index, segment := range path.segments {
		builder.WriteString(path.segmentSeparator(index))
		builder.WriteString(segment.display())
	}

	return builder.String()
}

// segmentSeparator returns the presentation separator before one segment.
func (path schemaPath) segmentSeparator(index int) string {
	if index == 0 || (path.segments[index].kind == schemaPathProperty &&
		!isSimpleSchemaPathProperty(path.segments[index].value)) {
		return ""
	}

	return "."
}

// display returns one path segment in presentation form.
func (segment schemaPathSegment) display() string {
	if segment.kind == schemaPathArrayItem {
		return "[]"
	}

	if isSimpleSchemaPathProperty(segment.value) {
		return segment.value
	}

	return "[" + strconv.Quote(segment.value) + "]"
}

// isSimpleSchemaPathProperty reports whether dot notation is unambiguous.
func isSimpleSchemaPathProperty(value string) bool {
	if value == "" {
		return false
	}

	for _, character := range value {
		if character != '_' && character != '-' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}

	return true
}
