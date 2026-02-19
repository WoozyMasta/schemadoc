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
		out = append(out, attributeView{Name: "Reference", Value: fmt.Sprintf("`%s`", escapeInline(value))})
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
		out = append(out, attributeView{Name: "Default", Value: fmt.Sprintf("`%s`", escapeInline(mustJSONInline(value)))})
	}

	if enum := asSlice(obj["enum"]); len(enum) > 0 {
		out = append(out, attributeView{Name: "Enum", Value: jsonList(enum)})
	}

	if value, ok := obj["const"]; ok {
		out = append(out, attributeView{Name: "Const", Value: fmt.Sprintf("`%s`", escapeInline(mustJSONInline(value)))})
	}

	if examples := asSlice(obj["examples"]); len(examples) > 0 {
		out = append(out, attributeView{Name: "Examples", Value: jsonList(examples)})
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
		out = append(out, attributeView{Name: "Content schema", Value: summarizeSchemaLike(value)})
	}

	if value, ok := obj["items"]; ok {
		out = append(out, attributeView{Name: "Items", Value: summarizeSchemaLike(value)})
	}

	if value, ok := obj["prefixItems"]; ok {
		out = append(out, attributeView{Name: "Prefix items", Value: summarizeSchemaLike(value)})
	}

	if value, ok := obj["additionalItems"]; ok {
		out = append(out, attributeView{Name: "Additional items", Value: summarizeSchemaLike(value)})
	}

	if value, ok := obj["contains"]; ok {
		out = append(out, attributeView{Name: "Contains", Value: summarizeSchemaLike(value)})
	}

	if value, ok := obj["unevaluatedItems"]; ok {
		out = append(out, attributeView{Name: "Unevaluated items", Value: summarizeSchemaLike(value)})
	}

	if properties := mapSchemaValues(obj["properties"]); len(properties) > 0 {
		out = append(out, attributeView{Name: "Properties", Value: strconv.Itoa(len(properties))})
	}

	if properties := mapSchemaValues(obj["patternProperties"]); len(properties) > 0 {
		out = append(out, attributeView{Name: "Pattern properties", Value: strconv.Itoa(len(properties))})
	}

	if value, ok := obj["additionalProperties"]; ok {
		out = append(out, attributeView{Name: "Additional properties", Value: summarizeSchemaLike(value)})
	}

	if value, ok := obj["unevaluatedProperties"]; ok {
		out = append(out, attributeView{Name: "Unevaluated properties", Value: summarizeSchemaLike(value)})
	}

	if value, ok := obj["propertyNames"]; ok {
		out = append(out, attributeView{Name: "Property names", Value: summarizeSchemaLike(value)})
	}

	if value, ok := obj["dependentRequired"]; ok {
		out = append(out, attributeView{Name: "Dependent required", Value: fmt.Sprintf("`%s`", escapeInline(mustJSONInline(value)))})
	}

	if values := mapSchemaValues(obj["dependentSchemas"]); len(values) > 0 {
		out = append(out, attributeView{Name: "Dependent schemas", Value: strconv.Itoa(len(values))})
	}

	if value, ok := obj["dependencies"]; ok {
		out = append(out, attributeView{Name: "Dependencies", Value: fmt.Sprintf("`%s`", escapeInline(mustJSONInline(value)))})
	}

	if composition := compositionSummary(obj); composition != "" {
		out = append(out, attributeView{Name: "Composition", Value: composition})
	}

	if conditional := conditionalSummary(obj); conditional != "" {
		out = append(out, attributeView{Name: "Conditional", Value: conditional})
	}

	if _, ok := obj["not"]; ok {
		out = append(out, attributeView{Name: "Not", Value: summarizeSchemaLike(obj["not"])})
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

// summarizeSchemaLike provides compact markdown text for schema-like value.
func summarizeSchemaLike(value any) string {
	switch typed := value.(type) {
	case bool:
		return "boolean schema=" + strconv.FormatBool(typed)
	case map[string]any:
		if ref := asString(typed["$ref"]); ref != "" {
			return "reference `" + escapeInline(ref) + "`"
		}

		if ref := asString(typed["$dynamicRef"]); ref != "" {
			return "dynamicRef `" + escapeInline(ref) + "`"
		}

		if ref := asString(typed["$recursiveRef"]); ref != "" {
			return "recursiveRef `" + escapeInline(ref) + "`"
		}

		if typedType := typeString(typed["type"]); typedType != "" {
			return "schema type `" + escapeInline(typedType) + "`"
		}

		return "inline schema"
	case []any:
		return "schema list (" + strconv.Itoa(len(typed)) + ")"
	default:
		return fmt.Sprintf("`%s`", escapeInline(mustJSONInline(typed)))
	}
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
			out = append(out, key+"="+mustJSONInline(value))
			continue
		}

		out = append(out, key+"="+mustJSONInline(value))
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

		out = append(out, key+"="+mustJSONInline(node[key]))
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

// jsonList renders JSON values list into comma-separated inline code tokens.
func jsonList(values []any) string {
	parts := make([]string, 0, len(values))
	for _, item := range values {
		parts = append(parts, fmt.Sprintf("`%s`", escapeInline(mustJSONInline(item))))
	}

	return strings.Join(parts, ", ")
}
