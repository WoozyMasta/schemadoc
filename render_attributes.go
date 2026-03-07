// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"fmt"
	"strconv"
	"strings"
)

// knownSchemaKeywords enumerates JSON Schema keywords excluded from "other keywords" listing.
var knownSchemaKeywords = map[string]struct{}{
	"$schema": {},
	"$id":     {},
	"id":      {},
	"$ref":    {},

	"$dynamicRef":      {},
	"$recursiveRef":    {},
	"$anchor":          {},
	"$dynamicAnchor":   {},
	"$recursiveAnchor": {},
	"$comment":         {},

	"$defs":       {},
	"definitions": {},
	"type":        {},
	"title":       {},
	"description": {},
	"default":     {},
	"examples":    {},
	"enum":        {},
	"const":       {},
	"format":      {},

	"allOf": {},
	"anyOf": {},
	"oneOf": {},
	"not":   {},
	"if":    {},
	"then":  {},
	"else":  {},

	"properties":            {},
	"patternProperties":     {},
	"additionalProperties":  {},
	"unevaluatedProperties": {},
	"propertyNames":         {},
	"required":              {},
	"dependentRequired":     {},
	"dependentSchemas":      {},
	"dependencies":          {},
	"minProperties":         {},
	"maxProperties":         {},

	"items":            {},
	"prefixItems":      {},
	"additionalItems":  {},
	"contains":         {},
	"unevaluatedItems": {},
	"minItems":         {},
	"maxItems":         {},
	"uniqueItems":      {},
	"minContains":      {},
	"maxContains":      {},

	"minimum":          {},
	"maximum":          {},
	"exclusiveMinimum": {},
	"exclusiveMaximum": {},
	"multipleOf":       {},
	"minLength":        {},
	"maxLength":        {},
	"pattern":          {},

	"readOnly":         {},
	"writeOnly":        {},
	"deprecated":       {},
	"contentEncoding":  {},
	"contentMediaType": {},
	"contentSchema":    {},
}

// schemaAttributes renders flat attribute list for one schema node.
func schemaAttributes(node schemaValue, required *bool) []attributeView {
	out := make([]attributeView, 0, 32)

	if node.Bool != nil {
		if required != nil {
			out = append(out, attributeView{Name: "Required", Value: yesNo(*required)})
		}

		out = append(out, attributeView{Name: "Boolean schema", Value: strconv.FormatBool(*node.Bool)})
		return out
	}

	obj := node.Object
	if obj == nil {
		if required != nil {
			out = append(out, attributeView{Name: "Required", Value: yesNo(*required)})
		}

		return out
	}

	if typeText := typeString(obj["type"]); typeText != "" {
		out = append(out, attributeView{Name: "Type", Value: fmt.Sprintf("`%s`", escapeInline(typeText))})
	}

	if required != nil {
		out = append(out, attributeView{Name: "Required", Value: yesNo(*required)})
	}

	if value := asString(obj["$ref"]); value != "" {
		out = append(out, attributeView{Name: "Reference", Value: formatReferenceValue(value)})
	}

	if value := asString(obj["$dynamicRef"]); value != "" {
		out = append(out, attributeView{Name: "Dynamic reference", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if value := asString(obj["$recursiveRef"]); value != "" {
		out = append(out, attributeView{Name: "Recursive reference", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if value := asString(obj["$anchor"]); value != "" {
		out = append(out, attributeView{Name: "Anchor", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if value := asString(obj["$dynamicAnchor"]); value != "" {
		out = append(out, attributeView{Name: "Dynamic anchor", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if value := asString(obj["$recursiveAnchor"]); value != "" {
		out = append(out, attributeView{Name: "Recursive anchor", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if value := asString(obj["title"]); value != "" {
		out = append(out, attributeView{Name: "Title", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if value, ok := obj["default"]; ok {
		out = append(out, attributeView{Name: "Default", Value: inlineCodeValue(value)})
	}

	if enum := asSlice(obj["enum"]); len(enum) > 0 {
		out = append(out, attributeView{Name: "Enum", Value: inlineValueList(enum)})
	}

	if value, ok := obj["const"]; ok {
		out = append(out, attributeView{Name: "Const", Value: inlineCodeValue(value)})
	}

	if examples := asSlice(obj["examples"]); len(examples) > 0 {
		out = append(out, attributeView{Name: "Examples", Value: inlineValueList(examples)})
	}

	if value := asString(obj["format"]); value != "" {
		out = append(out, attributeView{Name: "Format", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if value, ok := asBool(obj["readOnly"]); ok {
		out = append(out, attributeView{Name: "Read only", Value: yesNo(value)})
	}

	if value, ok := asBool(obj["writeOnly"]); ok {
		out = append(out, attributeView{Name: "Write only", Value: yesNo(value)})
	}

	if value, ok := asBool(obj["deprecated"]); ok {
		out = append(out, attributeView{Name: "Deprecated", Value: yesNo(value)})
	}

	if value := asString(obj["contentEncoding"]); value != "" {
		out = append(out, attributeView{Name: "Content encoding", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if value := asString(obj["contentMediaType"]); value != "" {
		out = append(out, attributeView{Name: "Content media type", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if value, ok := obj["contentSchema"]; ok {
		out = appendSchemaLikeAttributes(out, "Content schema", value)
	}

	if value, ok := obj["items"]; ok {
		out = appendSchemaLikeAttributes(out, "Items", value)
	}

	if value, ok := obj["prefixItems"]; ok {
		out = appendSchemaLikeAttributes(out, "Prefix items", value)
	}

	if value, ok := obj["additionalItems"]; ok {
		out = appendSchemaLikeAttributes(out, "Additional items", value)
	}

	if value, ok := obj["contains"]; ok {
		out = appendSchemaLikeAttributes(out, "Contains", value)
	}

	if value, ok := obj["unevaluatedItems"]; ok {
		out = appendSchemaLikeAttributes(out, "Unevaluated items", value)
	}

	if properties := mapSchemaValues(obj["properties"]); len(properties) > 0 {
		out = append(out, attributeView{Name: "Properties", Value: strconv.Itoa(len(properties))})
	}

	if properties := mapSchemaValues(obj["patternProperties"]); len(properties) > 0 {
		out = append(out, attributeView{Name: "Pattern properties", Value: strconv.Itoa(len(properties))})
	}

	if value, ok := obj["additionalProperties"]; ok {
		out = appendSchemaLikeAttributes(out, "Additional properties", value)
	}

	if value, ok := obj["unevaluatedProperties"]; ok {
		out = appendSchemaLikeAttributes(out, "Unevaluated properties", value)
	}

	if value, ok := obj["propertyNames"]; ok {
		out = appendSchemaLikeAttributes(out, "Property names", value)
	}

	if value, ok := obj["dependentRequired"]; ok {
		out = append(out, attributeView{Name: "Dependent required", Value: inlineCodeValue(value)})
	}

	if values := mapSchemaValues(obj["dependentSchemas"]); len(values) > 0 {
		out = append(out, attributeView{Name: "Dependent schemas", Value: strconv.Itoa(len(values))})
	}

	if value, ok := obj["dependencies"]; ok {
		out = append(out, attributeView{Name: "Dependencies", Value: inlineCodeValue(value)})
	}

	if composition := compositionSummary(obj); composition != "" {
		out = append(out, attributeView{Name: "Composition", Value: composition})
	}

	if conditional := conditionalSummary(obj); conditional != "" {
		out = append(out, attributeView{Name: "Conditional", Value: conditional})
	}

	if _, ok := obj["not"]; ok {
		out = appendSchemaLikeAttributes(out, "Not", obj["not"])
	}

	if constraints := constraintList(obj); len(constraints) > 0 {
		out = append(out, attributeView{Name: "Constraints", Value: strings.Join(constraints, "; ")})
	}

	if value := asString(obj["$comment"]); value != "" {
		out = append(out, attributeView{Name: "Comment", Value: fmt.Sprintf("`%s`", escapeInline(value))})
	}

	if other := otherKeywordList(obj); len(other) > 0 {
		out = append(out, attributeView{Name: "Other keywords", Value: strings.Join(other, "; ")})
	}

	return out
}

// appendSchemaLikeAttributes expands schema-like keywords into readable rows.
func appendSchemaLikeAttributes(out []attributeView, name string, value any) []attributeView {
	appendNamed := func(suffix string, value string) {
		key := name
		if suffix != "" {
			key += " " + suffix
		}

		out = append(out, attributeView{Name: key, Value: value})
	}

	switch typed := value.(type) {
	case bool:
		appendNamed("", "boolean schema="+strconv.FormatBool(typed))
		return out
	case map[string]any:
		appended := false

		if ref := asString(typed["$ref"]); ref != "" {
			appendNamed("reference", formatReferenceValue(ref))
			appended = true
		}

		if ref := asString(typed["$dynamicRef"]); ref != "" {
			appendNamed("dynamic reference", fmt.Sprintf("`%s`", escapeInline(ref)))
			appended = true
		}

		if ref := asString(typed["$recursiveRef"]); ref != "" {
			appendNamed("recursive reference", fmt.Sprintf("`%s`", escapeInline(ref)))
			appended = true
		}

		if typedType := typeString(typed["type"]); typedType != "" {
			appendNamed("type", fmt.Sprintf("`%s`", escapeInline(typedType)))
			appended = true
		}

		if value, ok := typed["default"]; ok {
			appendNamed("default", inlineCodeValue(value))
			appended = true
		}

		if value, ok := typed["const"]; ok {
			appendNamed("const", inlineCodeValue(value))
			appended = true
		}

		if enum := asSlice(typed["enum"]); len(enum) > 0 {
			appendNamed("enum", inlineValueList(enum))
			appended = true
		}

		if examples := asSlice(typed["examples"]); len(examples) > 0 {
			appendNamed("examples", inlineValueList(examples))
			appended = true
		}

		if format := asString(typed["format"]); format != "" {
			appendNamed("format", fmt.Sprintf("`%s`", escapeInline(format)))
			appended = true
		}

		if constraints := constraintList(typed); len(constraints) > 0 {
			appendNamed("constraints", strings.Join(constraints, "; "))
			appended = true
		}

		if composition := compositionSummary(typed); composition != "" {
			appendNamed("composition", composition)
			appended = true
		}

		if conditional := conditionalSummary(typed); conditional != "" {
			appendNamed("conditional", conditional)
			appended = true
		}

		if !appended {
			appendNamed("", "inline schema")
		}

		return out
	case []any:
		appendNamed("", summarizeSchemaList(typed))
		return out
	default:
		appendNamed("", inlineCodeValue(typed))
		return out
	}
}

// formatReferenceValue renders local definition reference in explicit readable form.
func formatReferenceValue(ref string) string {
	normalized := strings.TrimSpace(ref)
	if normalized == "" {
		return "``"
	}

	defName := rootDefinitionName(normalized)
	if defName == "" {
		return fmt.Sprintf("`%s`", escapeInline(normalized))
	}

	return fmt.Sprintf(
		"[`%s`](#%s) (`%s`)",
		escapeInline(defName),
		markdownHeadingAnchor(defName),
		escapeInline(normalized),
	)
}

// summarizeSchemaLike provides compact markdown text for schema-like value.
func summarizeSchemaLike(value any) string {
	switch typed := value.(type) {
	case bool:
		return "boolean schema=" + strconv.FormatBool(typed)
	case map[string]any:
		parts := make([]string, 0, 5)

		if ref := asString(typed["$ref"]); ref != "" {
			parts = append(parts, "reference `"+escapeInline(ref)+"`")
		}

		if ref := asString(typed["$dynamicRef"]); ref != "" {
			parts = append(parts, "dynamicRef `"+escapeInline(ref)+"`")
		}

		if ref := asString(typed["$recursiveRef"]); ref != "" {
			parts = append(parts, "recursiveRef `"+escapeInline(ref)+"`")
		}

		if typedType := typeString(typed["type"]); typedType != "" {
			parts = append(parts, "schema type `"+escapeInline(typedType)+"`")
		}

		if value, ok := typed["default"]; ok {
			parts = append(parts, "default "+inlineCodeValue(value))
		}

		if value, ok := typed["const"]; ok {
			parts = append(parts, "const "+inlineCodeValue(value))
		}

		if enum := asSlice(typed["enum"]); len(enum) > 0 {
			parts = append(parts, "enum "+inlineValueList(enum))
		}

		if examples := asSlice(typed["examples"]); len(examples) > 0 {
			parts = append(parts, "examples "+inlineValueList(examples))
		}

		if format := asString(typed["format"]); format != "" {
			parts = append(parts, "format `"+escapeInline(format)+"`")
		}

		if constraints := constraintList(typed); len(constraints) > 0 {
			parts = append(parts, "constraints "+strings.Join(constraints, ", "))
		}

		if composition := compositionSummary(typed); composition != "" {
			parts = append(parts, "composition "+composition)
		}

		if conditional := conditionalSummary(typed); conditional != "" {
			parts = append(parts, "conditional "+conditional)
		}

		if len(parts) == 0 {
			return "inline schema"
		}

		return strings.Join(parts, "; ")
	case []any:
		return summarizeSchemaList(typed)
	default:
		return inlineCodeValue(typed)
	}
}

// summarizeSchemaList provides compact markdown text for schema tuple lists.
func summarizeSchemaList(items []any) string {
	if len(items) == 0 {
		return "schema list (0)"
	}

	const previewLimit = 3
	preview := min(len(items), previewLimit)

	parts := make([]string, 0, preview+1)
	for index := range preview {
		parts = append(parts, "#"+strconv.Itoa(index+1)+" "+summarizeSchemaLike(items[index]))
	}

	if len(items) > preview {
		parts = append(parts, "... +"+strconv.Itoa(len(items)-preview))
	}

	return "schema list (" + strconv.Itoa(len(items)) + "): " + strings.Join(parts, "; ")
}

// compositionSummary renders one-line summary for allOf/anyOf/oneOf combinations.
func compositionSummary(node map[string]any) string {
	items := make([]string, 0, 3)
	if oneOf := asSlice(node["oneOf"]); len(oneOf) > 0 {
		items = append(items, "oneOf="+strconv.Itoa(len(oneOf)))
	}

	if anyOf := asSlice(node["anyOf"]); len(anyOf) > 0 {
		items = append(items, "anyOf="+strconv.Itoa(len(anyOf)))
	}

	if allOf := asSlice(node["allOf"]); len(allOf) > 0 {
		items = append(items, "allOf="+strconv.Itoa(len(allOf)))
	}

	return strings.Join(items, "; ")
}

// conditionalSummary renders one-line summary for if/then/else usage.
func conditionalSummary(node map[string]any) string {
	items := make([]string, 0, 3)
	if _, ok := node["if"]; ok {
		items = append(items, "if")
	}

	if _, ok := node["then"]; ok {
		items = append(items, "then")
	}

	if _, ok := node["else"]; ok {
		items = append(items, "else")
	}

	return strings.Join(items, ", ")
}

// constraintList renders numeric and string constraints as deterministic key/value pairs.
func constraintList(node map[string]any) []string {
	keys := []string{
		"minimum",
		"maximum",
		"exclusiveMinimum",
		"exclusiveMaximum",
		"multipleOf",
		"minLength",
		"maxLength",
		"pattern",
		"minItems",
		"maxItems",
		"uniqueItems",
		"minContains",
		"maxContains",
		"minProperties",
		"maxProperties",
	}

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		value, ok := node[key]
		if !ok {
			continue
		}

		if key == "pattern" {
			out = append(out, key+"="+inlineValueText(value))
			continue
		}

		out = append(out, key+"="+inlineValueText(value))
	}

	return out
}

// otherKeywordList lists non-standard keywords that were not rendered in known sections.
func otherKeywordList(node map[string]any) []string {
	if len(node) == 0 {
		return nil
	}

	out := make([]string, 0)
	for _, key := range sortedKeys(node) {
		if _, ok := knownSchemaKeywords[key]; ok {
			continue
		}

		out = append(out, key+"="+inlineValueText(node[key]))
	}

	return out
}

// typeString converts JSON Schema type field to display string.
func typeString(value any) string {
	if value == nil {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return typed
	default:
		return mustJSONInline(typed)
	}
}

// yesNo renders bool as "yes" or "no".
func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

// inlineValueList renders mixed values as inline code tokens.
// String values are rendered without JSON quotes for readability.
func inlineValueList(values []any) string {
	parts := make([]string, 0, len(values))
	for _, item := range values {
		parts = append(parts, inlineCodeValue(item))
	}

	return strings.Join(parts, ", ")
}

// inlineCodeValue renders value as inline code token.
func inlineCodeValue(value any) string {
	return fmt.Sprintf("`%s`", escapeInline(inlineValueText(value)))
}

// inlineValueText renders value text without JSON quotes for strings.
func inlineValueText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}

	return mustJSONInline(value)
}
