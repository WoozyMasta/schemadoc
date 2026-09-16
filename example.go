// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	// ExampleModeAll builds example with all declared properties.
	ExampleModeAll ExampleMode = "all"
	// ExampleModeRequired builds example with required properties only.
	ExampleModeRequired ExampleMode = "required"
)

// ExampleMode configures example generation property coverage.
type ExampleMode string

const (
	// ExampleFormatJSON encodes example payload as JSON.
	ExampleFormatJSON ExampleFormat = "json"
	// ExampleFormatYAML encodes example payload as YAML.
	ExampleFormatYAML ExampleFormat = "yaml"
)

// ExampleFormat configures output format for generated example payload.
type ExampleFormat string

const (
	// ExampleIndentTypeSpace configures space-based JSON indentation.
	ExampleIndentTypeSpace = "space"
	// ExampleIndentTypeTab configures tab-based JSON indentation.
	ExampleIndentTypeTab = "tab"
)

const (
	// YAMLCommentExamplesNone disables YAML comments sourced from examples.
	YAMLCommentExamplesNone YAMLCommentExamples = "none"
	// YAMLCommentExamplesScalar includes only scalar and null examples.
	YAMLCommentExamplesScalar YAMLCommentExamples = "scalar"
	// YAMLCommentExamplesAll includes scalar and structured examples.
	YAMLCommentExamplesAll YAMLCommentExamples = "all"
	// YAMLCommentFormatInline renders annotation values on one comment line.
	YAMLCommentFormatInline YAMLCommentFormat = "inline"
	// YAMLCommentFormatBlock renders structured annotation values as YAML blocks.
	YAMLCommentFormatBlock YAMLCommentFormat = "block"
)

// YAMLCommentExamples controls which schema examples are copied to YAML comments.
type YAMLCommentExamples string

// YAMLCommentFormat controls rendering of structured annotation values.
type YAMLCommentFormat string

// YAMLCommentPolicy configures independent schema annotation comments in YAML.
//
// Boolean pointers distinguish an omitted option from an explicit false value.
// A zero policy is normalized to the documented defaults.
type YAMLCommentPolicy struct {
	// Titles enables comments sourced from schema titles.
	Titles *bool `json:"titles,omitempty" yaml:"titles,omitempty"`

	// Descriptions enables comments sourced from schema descriptions.
	Descriptions *bool `json:"descriptions,omitempty" yaml:"descriptions,omitempty"`

	// Defaults enables comments sourced from schema defaults.
	Defaults *bool `json:"defaults,omitempty" yaml:"defaults,omitempty"`

	// Enums enables comments listing schema enum values.
	Enums *bool `json:"enums,omitempty" yaml:"enums,omitempty"`

	// Examples selects scalar, all, or no schema examples.
	Examples YAMLCommentExamples `json:"examples,omitempty" yaml:"examples,omitempty"`

	// ExampleFormat selects inline or block annotation value formatting.
	ExampleFormat YAMLCommentFormat `json:"example_format,omitempty" yaml:"example_format,omitempty"`
}

// ExampleOptions configures example output formatting.
type ExampleOptions struct {
	// YAMLComments configures schema annotation comments in YAML output.
	YAMLComments YAMLCommentPolicy
	// JSONIndentType sets JSON indentation type: space or tab. Default is space.
	JSONIndentType string
	// JSONIndent sets JSON indentation width. Default is 2.
	JSONIndent int
	// YAMLIndent sets YAML indentation width. Default is 2.
	YAMLIndent int
	// JSONMinify enables minified JSON output.
	JSONMinify bool
}

// exampleScalarPlaceholders provides fallback values for scalar schema types.
var exampleScalarPlaceholders = map[string]any{
	"string":  "<string>",
	"number":  0,
	"integer": 0,
	"boolean": false,
	"null":    nil,
}

// exampleBuilder converts normalized schema tree into example values.
type exampleBuilder struct {
	semantic     *schemaSemanticView
	yamlComments YAMLCommentPolicy
}

// GenerateExampleJSON returns generated example payload encoded as pretty JSON.
func GenerateExampleJSON(schemaBytes []byte, mode ExampleMode) ([]byte, error) {
	return GenerateExampleJSONWithOptions(schemaBytes, mode, ExampleOptions{})
}

// GenerateExampleJSONWithOptions returns generated example payload encoded as JSON.
func GenerateExampleJSONWithOptions(
	schemaBytes []byte,
	mode ExampleMode,
	options ExampleOptions,
) ([]byte, error) {
	value, err := generateExampleValue(schemaBytes, mode)
	if err != nil {
		return nil, err
	}

	data, err := marshalExampleJSON(value, normalizeExampleOptions(options))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncodeExampleJSON, err)
	}

	return data, nil
}

// GenerateExampleYAML returns generated example payload encoded as YAML.
func GenerateExampleYAML(schemaBytes []byte, mode ExampleMode) ([]byte, error) {
	return GenerateExampleYAMLWithOptions(schemaBytes, mode, ExampleOptions{})
}

// GenerateExampleYAMLWithOptions returns generated example payload encoded as YAML.
func GenerateExampleYAMLWithOptions(
	schemaBytes []byte,
	mode ExampleMode,
	options ExampleOptions,
) ([]byte, error) {
	mode, err := normalizeExampleMode(mode)
	if err != nil {
		return nil, err
	}

	doc, err := parseDocument(schemaBytes)
	if err != nil {
		return nil, err
	}
	normalizedOptions := normalizeExampleOptions(options)

	materializer := newExampleMaterializer(doc, mode)
	value, err := materializer.materialize(doc.Root)
	if err != nil {
		return nil, err
	}

	builder := exampleBuilder{
		semantic:     materializer.semantic,
		yamlComments: normalizedOptions.YAMLComments,
	}
	rootNode, err := yamlNodeForValue(value)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncodeExampleYAML, err)
	}

	builder.annotateYAMLNode(rootNode, doc.Root)

	data, err := marshalExampleYAMLNode(rootNode, normalizedOptions.YAMLIndent)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncodeExampleYAML, err)
	}

	return data, nil
}

// GenerateExample returns generated example payload encoded in selected format.
func GenerateExample(schemaBytes []byte, mode ExampleMode, format ExampleFormat) ([]byte, error) {
	return GenerateExampleWithOptions(schemaBytes, mode, format, ExampleOptions{})
}

// GenerateExampleWithOptions returns generated example payload encoded in selected format.
func GenerateExampleWithOptions(
	schemaBytes []byte,
	mode ExampleMode,
	format ExampleFormat,
	options ExampleOptions,
) ([]byte, error) {
	format, err := normalizeExampleFormat(format)
	if err != nil {
		return nil, err
	}

	switch format {
	case ExampleFormatJSON:
		return GenerateExampleJSONWithOptions(schemaBytes, mode, options)
	case ExampleFormatYAML:
		return GenerateExampleYAMLWithOptions(schemaBytes, mode, options)
	default:
		return nil, fmt.Errorf("%w %q", ErrUnknownExampleFormat, format)
	}
}

// normalizeExampleOptions sets default formatting values for example generation.
func normalizeExampleOptions(options ExampleOptions) ExampleOptions {
	if options.JSONIndent < 1 {
		options.JSONIndent = 2
	}

	switch strings.ToLower(strings.TrimSpace(options.JSONIndentType)) {
	case "", ExampleIndentTypeSpace:
		options.JSONIndentType = ExampleIndentTypeSpace
	case ExampleIndentTypeTab:
		options.JSONIndentType = ExampleIndentTypeTab
	default:
		options.JSONIndentType = ExampleIndentTypeSpace
	}

	if options.YAMLIndent < 1 {
		options.YAMLIndent = 2
	}
	options.YAMLComments = normalizeYAMLCommentPolicy(options.YAMLComments)

	return options
}

// normalizeYAMLCommentPolicy applies defaults while preserving explicit false values.
func normalizeYAMLCommentPolicy(policy YAMLCommentPolicy) YAMLCommentPolicy {
	if policy.Titles == nil {
		policy.Titles = boolPointer(true)
	}
	if policy.Descriptions == nil {
		policy.Descriptions = boolPointer(true)
	}
	if policy.Defaults == nil {
		policy.Defaults = boolPointer(true)
	}
	if policy.Enums == nil {
		policy.Enums = boolPointer(true)
	}

	switch policy.Examples {
	case YAMLCommentExamplesNone, YAMLCommentExamplesScalar, YAMLCommentExamplesAll:
	default:
		policy.Examples = YAMLCommentExamplesScalar
	}

	switch policy.ExampleFormat {
	case YAMLCommentFormatInline, YAMLCommentFormatBlock:
	default:
		policy.ExampleFormat = YAMLCommentFormatBlock
	}

	return policy
}

// boolPointer returns a stable pointer for normalized boolean options.
func boolPointer(value bool) *bool {
	return &value
}

// generateExampleValue parses schema and builds example value for selected mode.
func generateExampleValue(schemaBytes []byte, mode ExampleMode) (any, error) {
	mode, err := normalizeExampleMode(mode)
	if err != nil {
		return nil, err
	}

	doc, err := parseDocument(schemaBytes)
	if err != nil {
		return nil, err
	}

	return newExampleMaterializer(doc, mode).materialize(doc.Root)
}

// normalizeExampleMode validates and normalizes caller mode value.
func normalizeExampleMode(mode ExampleMode) (ExampleMode, error) {
	normalized := ExampleMode(strings.ToLower(strings.TrimSpace(string(mode))))
	switch normalized {
	case ExampleModeAll, ExampleModeRequired:
		return normalized, nil
	default:
		return "", fmt.Errorf("%w %q", ErrUnknownExampleMode, mode)
	}
}

// normalizeExampleFormat validates and normalizes caller format value.
func normalizeExampleFormat(format ExampleFormat) (ExampleFormat, error) {
	normalized := ExampleFormat(strings.ToLower(strings.TrimSpace(string(format))))
	switch normalized {
	case ExampleFormatJSON, ExampleFormatYAML:
		return normalized, nil
	default:
		return "", fmt.Errorf("%w %q", ErrUnknownExampleFormat, format)
	}
}

// requiredPropertyOrder returns deterministic order for required properties only.
func requiredPropertyOrder(required []string, properties map[string]schemaValue) []string {
	if len(required) == 0 || len(properties) == 0 {
		return nil
	}

	out := make([]string, 0, len(required))
	seen := make(map[string]struct{}, len(required))
	for _, key := range required {
		if _, exists := properties[key]; !exists {
			continue
		}

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, key)
	}

	return out
}

// cloneJSONValue deep-copies maps and slices used as generated payload values.
func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = cloneJSONValue(item)
		}

		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneJSONValue(item))
		}

		return out
	default:
		return typed
	}
}

// marshalExampleJSON serializes example payload as JSON.
func marshalExampleJSON(value any, options ExampleOptions) ([]byte, error) {
	if options.JSONMinify {
		return json.Marshal(value)
	}

	indent := strings.Repeat(" ", options.JSONIndent)
	if options.JSONIndentType == ExampleIndentTypeTab {
		indent = strings.Repeat("\t", options.JSONIndent)
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)

	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// marshalExampleYAML serializes example payload as YAML.
func marshalExampleYAMLNode(node *yaml.Node, indent int) ([]byte, error) {
	document := &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{node},
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(indent)

	if err := encoder.Encode(document); err != nil {
		return nil, err
	}

	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// annotateYAMLNode assigns schema title/description comments to YAML map keys.
func (builder *exampleBuilder) annotateYAMLNode(node *yaml.Node, schema schemaValue) {
	if builder.semantic == nil {
		return
	}

	effective, err := builder.semantic.effective(schema)
	if err != nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		properties, err := effective.properties(builder.semantic)
		if err != nil {
			return
		}
		reorderMappingNodeByEffectiveSchema(node, properties)

		for index := 0; index+1 < len(node.Content); index += 2 {
			keyNode := node.Content[index]
			valueNode := node.Content[index+1]

			property, ok, err := effective.property(builder.semantic, keyNode.Value)
			if err != nil || !ok {
				continue
			}

			if comment := schemaKeyCommentEffective(property, builder.yamlComments); comment != "" {
				keyNode.HeadComment = comment
			}

			builder.annotateYAMLNode(valueNode, property.asSchemaValue())
		}

	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			return
		}

		for index, item := range node.Content {
			itemSchema := effectiveArrayItemSchemaAt(effective, index, builder.semantic)
			builder.annotateYAMLNode(item, itemSchema)
		}
	}
}

// reorderMappingNodeByEffectiveSchema reorders keys using effective x-order annotations.
func reorderMappingNodeByEffectiveSchema(node *yaml.Node, properties map[string]effectiveSchema) {
	if node == nil || node.Kind != yaml.MappingNode || len(node.Content) < 4 {
		return
	}

	keyPositions := make(map[string]int, len(node.Content)/2)
	keys := make([]string, 0, len(node.Content)/2)
	for index := 0; index+1 < len(node.Content); index += 2 {
		key := node.Content[index].Value
		keyPositions[key] = index
		keys = append(keys, key)
	}

	orderedKeys := sortKeysByEffectiveSchemaOrder(keys, properties)
	if len(orderedKeys) != len(keys) {
		return
	}

	orderedContent := make([]*yaml.Node, 0, len(node.Content))
	for _, key := range orderedKeys {
		pos, ok := keyPositions[key]
		if !ok || pos+1 >= len(node.Content) {
			continue
		}

		orderedContent = append(orderedContent, node.Content[pos], node.Content[pos+1])
	}

	if len(orderedContent) == len(node.Content) {
		node.Content = orderedContent
	}
}

// sortKeysByEffectiveSchemaOrder sorts keys by effective x-order, then name.
func sortKeysByEffectiveSchemaOrder(keys []string, properties map[string]effectiveSchema) []string {
	type effectiveKeyOrder struct {
		key      string
		order    float64
		hasOrder bool
	}

	list := make([]effectiveKeyOrder, 0, len(keys))
	for _, key := range keys {
		item := effectiveKeyOrder{key: key, order: math.MaxFloat64}
		property, ok := properties[key]
		if ok {
			if value, exists := property.annotation("x-order"); exists {
				if order, valid := asNumber(value); valid {
					item.order = order
					item.hasOrder = true
				}
			}
		}
		list = append(list, item)
	}

	sort.SliceStable(list, func(i, j int) bool {
		left, right := list[i], list[j]
		if left.hasOrder && right.hasOrder && left.order != right.order {
			return left.order < right.order
		}
		if left.hasOrder != right.hasOrder {
			return left.hasOrder
		}
		return left.key < right.key
	})

	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, item.key)
	}
	return out
}

// sortKeysBySchemaOrder sorts keys by `x-order`, then by key name.
func sortKeysBySchemaOrder(keys []string, properties map[string]schemaValue) []string {
	type keyOrder struct {
		key      string
		order    float64
		hasOrder bool
	}

	list := make([]keyOrder, 0, len(keys))
	for _, key := range keys {
		item := keyOrder{
			key:      key,
			order:    math.MaxFloat64,
			hasOrder: false,
		}

		property, ok := properties[key]
		if ok && property.Object != nil {
			if value, ok := asNumber(property.Object["x-order"]); ok {
				item.order = value
				item.hasOrder = true
			}
		}

		list = append(list, item)
	}

	sort.SliceStable(list, func(i, j int) bool {
		left := list[i]
		right := list[j]

		if left.hasOrder && right.hasOrder {
			if left.order == right.order {
				return left.key < right.key
			}

			return left.order < right.order
		}

		if left.hasOrder {
			return true
		}

		if right.hasOrder {
			return false
		}

		return left.key < right.key
	})

	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, item.key)
	}

	return out
}

// effectiveArrayItemSchemaAt selects the schema applicable to one array index.
func effectiveArrayItemSchemaAt(schema effectiveSchema, index int, view *schemaSemanticView) schemaValue {
	items, err := schema.arrayItems(view)
	if err != nil {
		return schemaValue{}
	}

	item, ok, err := arrayItemSchemaAt(items, index)
	if err != nil || !ok {
		return schemaValue{}
	}

	return item.asSchemaValue()
}

// schemaKeyComment builds YAML key comments from selected schema annotations.
func schemaKeyComment(schema schemaValue, policy YAMLCommentPolicy) string {
	if schema.Object == nil {
		return ""
	}

	policy = normalizeYAMLCommentPolicy(policy)

	title := ""
	if *policy.Titles {
		title = strings.TrimSpace(asString(schema.Object["title"]))
	}
	description := ""
	if *policy.Descriptions {
		description = strings.TrimSpace(asString(schema.Object["description"]))
	}

	baseComment := ""
	switch {
	case title == "" && description == "":
		baseComment = ""
	case title == "":
		baseComment = normalizeYAMLComment(description)
	case description == "":
		baseComment = normalizeYAMLComment(title)
	default:
		if title == description {
			baseComment = normalizeYAMLComment(title)
			break
		}

		baseComment = normalizeYAMLComment(title + "\n" + description)
	}

	lines := make([]string, 0, 6)
	if baseComment != "" {
		lines = append(lines, strings.Split(baseComment, "\n")...)
	}

	if value, ok := schema.Object["default"]; ok && *policy.Defaults {
		appendYAMLCommentValue(&lines, "Default:", value, policy.ExampleFormat)
	}

	if values := asSlice(schema.Object["examples"]); policy.Examples != YAMLCommentExamplesNone && len(values) > 0 {
		for _, value := range values {
			if policy.Examples == YAMLCommentExamplesScalar && !isScalarAnnotationValue(value) {
				continue
			}
			if defaultValue, hasDefault := schema.Object["default"]; hasDefault && equalJSONValue(value, defaultValue) {
				continue
			}

			appendYAMLCommentValue(&lines, "Example:", value, policy.ExampleFormat)
			break
		}
	}

	if values := schemaEnumValues(schema.Object, policy); len(values) > 0 && *policy.Enums {
		if policy.ExampleFormat == YAMLCommentFormatBlock && hasStructuredAnnotationValue(asSlice(schema.Object["enum"])) {
			appendYAMLCommentValue(&lines, "Allowed values:", asSlice(schema.Object["enum"]), policy.ExampleFormat)
		} else {
			lines = append(lines, "Allowed values: "+strings.Join(values, ", "))
		}
	}

	return normalizeYAMLComment(strings.Join(lines, "\n"))
}

// schemaKeyCommentEffective builds comments using presentation annotation precedence.
func schemaKeyCommentEffective(schema effectiveSchema, policy YAMLCommentPolicy) string {
	titleValue, _ := schema.annotation("title")
	descriptionValue, _ := schema.annotation("description")

	title := strings.TrimSpace(asString(titleValue))
	description := strings.TrimSpace(asString(descriptionValue))
	object := map[string]any{
		"title":       title,
		"description": description,
	}
	for _, keyword := range []string{"default", "examples", "enum"} {
		if value, exists := schema.annotation(keyword); exists {
			object[keyword] = value
		}
	}

	return schemaKeyComment(schemaValue{Object: object}, policy)
}

// schemaEnumValues returns normalized one-line enum values for comments.
func schemaEnumValues(object map[string]any, policy YAMLCommentPolicy) []string {
	values := asSlice(object["enum"])
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, formatYAMLCommentValue(value, policy.ExampleFormat, false))
	}

	return out
}

// appendYAMLCommentValue appends scalar values inline and structured values
// as readable continuation lines when block formatting is selected.
func appendYAMLCommentValue(lines *[]string, label string, value any, format YAMLCommentFormat) {
	formatted := formatYAMLCommentValue(value, format, true)
	if formatted == "" {
		return
	}
	if !strings.Contains(formatted, "\n") {
		*lines = append(*lines, label+" "+formatted)
		return
	}

	*lines = append(*lines, label)
	for line := range strings.SplitSeq(formatted, "\n") {
		*lines = append(*lines, "  "+line)
	}
}

// formatYAMLCommentValue formats one annotation
// without losing false, zero, or empty-string values.
func formatYAMLCommentValue(value any, format YAMLCommentFormat, allowBlock bool) string {
	if !allowBlock || format != YAMLCommentFormatBlock || isScalarAnnotationValue(value) {
		return normalizeInlineCommentValue(value)
	}

	encoded, err := yaml.Marshal(value)
	if err != nil {
		return normalizeInlineCommentValue(value)
	}

	return strings.TrimSpace(string(encoded))
}

// isScalarAnnotationValue identifies values suitable for scalar example mode.
func isScalarAnnotationValue(value any) bool {
	if value == nil {
		return true
	}

	switch value.(type) {
	case bool, string:
		return true
	default:
		_, ok := asNumber(value)
		return ok
	}
}

// hasStructuredAnnotationValue reports whether a collection contains a map or array.
func hasStructuredAnnotationValue(values []any) bool {
	for _, value := range values {
		if !isScalarAnnotationValue(value) {
			return true
		}
	}

	return false
}

// normalizeInlineCommentValue converts value to one-line safe YAML comment text.
func normalizeInlineCommentValue(value any) string {
	if value == nil {
		return "null"
	}

	switch typed := value.(type) {
	case string:
		if typed == "" {
			return `""`
		}
		return normalizeInlineWhitespace(typed)

	default:
		encoded, err := json.Marshal(typed)
		if err == nil {
			return normalizeInlineWhitespace(string(encoded))
		}

		return normalizeInlineWhitespace(fmt.Sprint(typed))
	}
}

// normalizeInlineWhitespace removes line breaks and repeated whitespace.
func normalizeInlineWhitespace(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}

// normalizeYAMLComment strips empty leading/trailing lines from comment body.
func normalizeYAMLComment(comment string) string {
	lines := strings.Split(comment, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	if start >= end {
		return ""
	}

	normalized := make([]string, 0, end-start)
	for _, line := range lines[start:end] {
		if strings.TrimSpace(line) == "" {
			continue
		}

		normalized = append(normalized, line)
	}

	if len(normalized) == 0 {
		return ""
	}

	return strings.Join(normalized, "\n")
}

// yamlNodeForValue builds deterministic yaml.Node tree from JSON-like value.
func yamlNodeForValue(value any) (*yaml.Node, error) {
	switch typed := value.(type) {
	case nil:
		return yamlScalarNode("!!null", "null"), nil

	case bool:
		return yamlScalarNode("!!bool", strconv.FormatBool(typed)), nil

	case string:
		return yamlScalarNode("!!str", typed), nil

	case json.Number:
		if int64Value, err := typed.Int64(); err == nil {
			return yamlScalarNode("!!int", strconv.FormatInt(int64Value, 10)), nil
		}
		float64Value, err := typed.Float64()
		if err != nil {
			return nil, err
		}
		return yamlScalarNode("!!float", strconv.FormatFloat(float64Value, 'g', -1, 64)), nil

	case int:
		return yamlScalarNode("!!int", strconv.Itoa(typed)), nil

	case int8:
		return yamlScalarNode("!!int", strconv.FormatInt(int64(typed), 10)), nil

	case int16:
		return yamlScalarNode("!!int", strconv.FormatInt(int64(typed), 10)), nil

	case int32:
		return yamlScalarNode("!!int", strconv.FormatInt(int64(typed), 10)), nil

	case int64:
		return yamlScalarNode("!!int", strconv.FormatInt(typed, 10)), nil

	case uint:
		return yamlScalarNode("!!int", strconv.FormatUint(uint64(typed), 10)), nil

	case uint8:
		return yamlScalarNode("!!int", strconv.FormatUint(uint64(typed), 10)), nil

	case uint16:
		return yamlScalarNode("!!int", strconv.FormatUint(uint64(typed), 10)), nil

	case uint32:
		return yamlScalarNode("!!int", strconv.FormatUint(uint64(typed), 10)), nil

	case uint64:
		return yamlScalarNode("!!int", strconv.FormatUint(typed, 10)), nil

	case float32:
		floatValue := float64(typed)
		if floatValue == math.Trunc(floatValue) {
			return yamlScalarNode("!!int", strconv.FormatInt(int64(floatValue), 10)), nil
		}

		return yamlScalarNode("!!float", strconv.FormatFloat(floatValue, 'g', -1, 64)), nil

	case float64:
		if typed == math.Trunc(typed) {
			return yamlScalarNode("!!int", strconv.FormatInt(int64(typed), 10)), nil
		}

		return yamlScalarNode("!!float", strconv.FormatFloat(typed, 'g', -1, 64)), nil

	case map[string]any:
		node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, key := range sortedKeys(typed) {
			valueNode, err := yamlNodeForValue(typed[key])
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, yamlScalarNode("!!str", key), valueNode)
		}
		return node, nil

	case []any:
		node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range typed {
			valueNode, err := yamlNodeForValue(item)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, valueNode)
		}
		return node, nil

	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil, err
		}
		var normalized any
		if err := json.Unmarshal(data, &normalized); err != nil {
			return nil, err
		}
		return yamlNodeForValue(normalized)
	}
}

// yamlScalarNode creates one scalar yaml.Node with explicit tag.
func yamlScalarNode(tag, value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   tag,
		Value: value,
	}
}
