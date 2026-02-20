// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
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
	activeRefs map[string]int
	mode       ExampleMode
	doc        schemaDocument
}

// GenerateExampleJSON returns generated example payload encoded as pretty JSON.
func GenerateExampleJSON(schemaBytes []byte, mode ExampleMode) ([]byte, error) {
	value, err := generateExampleValue(schemaBytes, mode)
	if err != nil {
		return nil, err
	}

	data, err := marshalExampleJSON(value)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncodeExampleJSON, err)
	}

	return data, nil
}

// GenerateExampleYAML returns generated example payload encoded as YAML.
func GenerateExampleYAML(schemaBytes []byte, mode ExampleMode) ([]byte, error) {
	mode, err := normalizeExampleMode(mode)
	if err != nil {
		return nil, err
	}

	doc, err := parseDocument(schemaBytes)
	if err != nil {
		return nil, err
	}

	builder := exampleBuilder{
		doc:        doc,
		mode:       mode,
		activeRefs: make(map[string]int),
	}

	value := builder.buildNode(doc.Root)
	rootNode, err := yamlNodeForValue(value)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncodeExampleYAML, err)
	}

	builder.annotateYAMLNode(rootNode, doc.Root)

	data, err := marshalExampleYAMLNode(rootNode)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncodeExampleYAML, err)
	}

	return data, nil
}

// GenerateExample returns generated example payload encoded in selected format.
func GenerateExample(schemaBytes []byte, mode ExampleMode, format ExampleFormat) ([]byte, error) {
	format, err := normalizeExampleFormat(format)
	if err != nil {
		return nil, err
	}

	switch format {
	case ExampleFormatJSON:
		return GenerateExampleJSON(schemaBytes, mode)
	case ExampleFormatYAML:
		return GenerateExampleYAML(schemaBytes, mode)
	default:
		return nil, fmt.Errorf("%w %q", ErrUnknownExampleFormat, format)
	}
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

	builder := exampleBuilder{
		doc:        doc,
		mode:       mode,
		activeRefs: make(map[string]int),
	}

	return builder.buildNode(doc.Root), nil
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

// buildNode recursively builds example value for one schema node.
func (builder *exampleBuilder) buildNode(node schemaValue) any {
	if node.Bool != nil {
		return nil
	}

	if node.Object == nil {
		return nil
	}

	object := node.Object
	if resolved, release, handled := builder.resolvedObjectForReference(object); handled {
		if release != nil {
			defer release()
		}

		if resolved == nil {
			return nil
		}

		return builder.buildNode(schemaValue{Object: resolved})
	}

	return builder.buildFromObject(object)
}

// buildFromObject builds example from non-boolean schema object.
func (builder *exampleBuilder) buildFromObject(object map[string]any) any {
	schemaType := schemaTypeName(object)
	properties, required := builder.collectObjectShape(schemaValue{Object: object})

	if schemaType == "object" || len(properties) > 0 || len(required) > 0 {
		return builder.buildObjectFromShape(properties, required)
	}

	if schemaType == "array" || hasArrayShape(object) {
		return builder.buildArrayFromObject(object)
	}

	if value, ok := explicitExampleValue(object); ok {
		return cloneJSONValue(value)
	}

	if value, ok := constExampleValue(object); ok {
		return cloneJSONValue(value)
	}

	if value, ok := enumExampleValue(object); ok {
		return cloneJSONValue(value)
	}

	if value, ok := builder.buildCompositionFallback(object); ok {
		return value
	}

	if value, ok := scalarPlaceholder(schemaType); ok {
		return value
	}

	return nil
}

// buildObjectFromShape materializes object value from collected property shape.
func (builder *exampleBuilder) buildObjectFromShape(properties map[string]schemaValue, required []string) map[string]any {
	out := make(map[string]any)
	if len(properties) == 0 {
		return out
	}

	order := propertyOrder(required, properties)
	if builder.mode == ExampleModeRequired {
		order = requiredPropertyOrder(required, properties)
	}

	for _, key := range order {
		value := builder.buildNode(properties[key])
		out[key] = value
	}

	return out
}

// buildArrayFromObject materializes array value from schema items/prefixItems.
func (builder *exampleBuilder) buildArrayFromObject(object map[string]any) []any {
	if value, ok := explicitExampleValue(object); ok {
		items, ok := value.([]any)
		if ok {
			return cloneJSONValue(items).([]any)
		}
	}

	if value, ok := constExampleValue(object); ok {
		items, ok := value.([]any)
		if ok {
			return cloneJSONValue(items).([]any)
		}
	}

	if value, ok := enumExampleValue(object); ok {
		items, ok := value.([]any)
		if ok {
			return cloneJSONValue(items).([]any)
		}
	}

	prefixItems := asSlice(object["prefixItems"])
	if len(prefixItems) > 0 {
		out := make([]any, 0, len(prefixItems))
		for _, raw := range prefixItems {
			item, ok := toSchemaValue(raw)
			if !ok {
				out = append(out, nil)
				continue
			}

			out = append(out, builder.buildNode(item))
		}

		return out
	}

	item, ok := toSchemaValue(object["items"])
	if ok {
		return []any{builder.buildNode(item)}
	}

	return []any{}
}

// collectObjectShape returns merged object properties and required keys for node.
func (builder *exampleBuilder) collectObjectShape(node schemaValue) (map[string]schemaValue, []string) {
	if node.Object == nil {
		return nil, nil
	}

	if resolved, release, handled := builder.resolvedObjectForReference(node.Object); handled {
		if release != nil {
			defer release()
		}

		if resolved == nil {
			return nil, nil
		}

		return builder.collectObjectShape(schemaValue{Object: resolved})
	}

	return builder.collectObjectShapeFromObject(node.Object)
}

// collectObjectShapeFromObject merges local properties and allOf object overlays.
func (builder *exampleBuilder) collectObjectShapeFromObject(object map[string]any) (map[string]schemaValue, []string) {
	properties := mapSchemaValues(object["properties"])
	required := asStringSlice(object["required"])

	for _, raw := range asSlice(object["allOf"]) {
		schema, ok := toSchemaValue(raw)
		if !ok {
			continue
		}

		nestedProperties, nestedRequired := builder.collectObjectShape(schema)
		properties = mergePropertySchemas(properties, nestedProperties)
		required = mergeRequiredKeys(required, nestedRequired)
	}

	return properties, required
}

// mergePropertySchemas merges schema property maps while preserving existing keys.
func mergePropertySchemas(left, right map[string]schemaValue) map[string]schemaValue {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}

	if len(left) == 0 {
		out := make(map[string]schemaValue, len(right))
		maps.Copy(out, right)

		return out
	}

	out := make(map[string]schemaValue, len(left)+len(right))
	maps.Copy(out, left)

	for key, value := range right {
		if _, exists := out[key]; exists {
			continue
		}

		out[key] = value
	}

	return out
}

// mergeRequiredKeys appends unique required keys while preserving first-seen order.
func mergeRequiredKeys(left, right []string) []string {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(left)+len(right))
	out := make([]string, 0, len(left)+len(right))

	for _, key := range append(left, right...) {
		key = strings.TrimSpace(key)
		if key == "" {
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

// buildCompositionFallback builds value from first schema of oneOf/anyOf/allOf.
func (builder *exampleBuilder) buildCompositionFallback(object map[string]any) (any, bool) {
	for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
		items := asSlice(object[keyword])
		for _, item := range items {
			schema, ok := toSchemaValue(item)
			if !ok {
				continue
			}

			return builder.buildNode(schema), true
		}
	}

	return nil, false
}

// resolvedObjectForReference resolves local ref and merges sibling override keywords.
func (builder *exampleBuilder) resolvedObjectForReference(object map[string]any) (map[string]any, func(), bool) {
	ref := asString(object["$ref"])
	if ref == "" {
		return nil, nil, false
	}

	stripAndContinue := stripReferenceKeyword(object)
	resolved, ok := builder.resolveLocalReference(ref)
	if !ok || resolved.Object == nil {
		return stripAndContinue, nil, true
	}

	release, ok := builder.enterReference(ref)
	if !ok {
		return nil, nil, true
	}

	return mergeSchemaObjects(resolved.Object, object), release, true
}

// resolveLocalReference resolves local JSON pointer references against root schema.
func (builder *exampleBuilder) resolveLocalReference(ref string) (schemaValue, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || !strings.HasPrefix(ref, "#") {
		return schemaValue{}, false
	}

	if len(builder.doc.RawKeywords) == 0 {
		return schemaValue{}, false
	}

	raw, ok := resolveJSONPointer(builder.doc.RawKeywords, ref)
	if !ok {
		return schemaValue{}, false
	}

	return toSchemaValue(raw)
}

// resolveJSONPointer resolves JSON pointer token path from root document value.
func resolveJSONPointer(root any, ref string) (any, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "#" {
		return root, true
	}

	if !strings.HasPrefix(ref, "#/") {
		return nil, false
	}

	current := root
	tokens := strings.SplitSeq(strings.TrimPrefix(ref, "#/"), "/")
	for token := range tokens {
		token = decodeJSONPointerToken(token)

		switch typed := current.(type) {
		case map[string]any:
			next, exists := typed[token]
			if !exists {
				return nil, false
			}

			current = next
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}

			current = typed[index]
		default:
			return nil, false
		}
	}

	return current, true
}

// decodeJSONPointerToken unescapes one JSON pointer token.
func decodeJSONPointerToken(token string) string {
	token = strings.ReplaceAll(token, "~1", "/")
	token = strings.ReplaceAll(token, "~0", "~")
	return token
}

// enterReference registers active local ref and returns release callback.
func (builder *exampleBuilder) enterReference(ref string) (func(), bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, true
	}

	if builder.activeRefs[ref] > 0 {
		return nil, false
	}

	builder.activeRefs[ref]++
	return func() {
		builder.activeRefs[ref]--
		if builder.activeRefs[ref] <= 0 {
			delete(builder.activeRefs, ref)
		}
	}, true
}

// schemaTypeName returns first non-null type value from schema "type" keyword.
func schemaTypeName(object map[string]any) string {
	typeValue, exists := object["type"]
	if !exists {
		return ""
	}

	if text := strings.ToLower(asString(typeValue)); text != "" {
		return text
	}

	items := asSlice(typeValue)
	for _, item := range items {
		text := strings.ToLower(asString(item))
		if text == "" || text == "null" {
			continue
		}

		return text
	}

	for _, item := range items {
		text := strings.ToLower(asString(item))
		if text == "null" {
			return text
		}
	}

	return ""
}

// hasArrayShape reports whether schema has array structure keywords.
func hasArrayShape(object map[string]any) bool {
	if _, ok := toSchemaValue(object["items"]); ok {
		return true
	}

	return len(asSlice(object["prefixItems"])) > 0
}

// explicitExampleValue returns preferred explicit example value from schema object.
func explicitExampleValue(object map[string]any) (any, bool) {
	if value, ok := object["default"]; ok {
		return value, true
	}

	if value := asSlice(object["examples"]); len(value) > 0 {
		return value[0], true
	}

	if value, ok := object["example"]; ok {
		return value, true
	}

	return nil, false
}

// constExampleValue returns const value as example when available.
func constExampleValue(object map[string]any) (any, bool) {
	value, ok := object["const"]
	return value, ok
}

// enumExampleValue returns first enum value as example when available.
func enumExampleValue(object map[string]any) (any, bool) {
	values := asSlice(object["enum"])
	if len(values) == 0 {
		return nil, false
	}

	return values[0], true
}

// scalarPlaceholder returns fallback placeholder for known scalar schema types.
func scalarPlaceholder(schemaType string) (any, bool) {
	value, ok := exampleScalarPlaceholders[schemaType]
	return value, ok
}

// stripReferenceKeyword returns shallow copy without $ref keyword.
func stripReferenceKeyword(object map[string]any) map[string]any {
	out := make(map[string]any, len(object))
	for key, value := range object {
		if key == "$ref" {
			continue
		}

		out[key] = value
	}

	return out
}

// mergeSchemaObjects merges resolved reference object with sibling keyword overrides.
func mergeSchemaObjects(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	maps.Copy(out, base)

	for key, value := range overlay {
		if key == "$ref" {
			continue
		}

		out[key] = value
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

// marshalExampleJSON serializes example payload as pretty JSON.
func marshalExampleJSON(value any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// marshalExampleYAML serializes example payload as YAML.
func marshalExampleYAMLNode(node *yaml.Node) ([]byte, error) {
	document := &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{node},
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)

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
	resolved, release := builder.resolveSchemaValue(schema)
	if release != nil {
		defer release()
	}

	switch node.Kind {
	case yaml.MappingNode:
		properties := nodeProperties(resolved)
		for index := 0; index+1 < len(node.Content); index += 2 {
			keyNode := node.Content[index]
			valueNode := node.Content[index+1]

			property, ok := properties[keyNode.Value]
			if !ok {
				continue
			}

			if comment := schemaKeyComment(property); comment != "" {
				keyNode.HeadComment = comment
			}

			builder.annotateYAMLNode(valueNode, property)
		}
	case yaml.SequenceNode:
		if len(node.Content) == 0 || resolved.Object == nil {
			return
		}

		itemSchema := sequenceItemSchema(resolved)
		for _, item := range node.Content {
			builder.annotateYAMLNode(item, itemSchema)
		}
	}
}

// resolveSchemaValue expands local references for schema node and preserves release callback.
func (builder *exampleBuilder) resolveSchemaValue(schema schemaValue) (schemaValue, func()) {
	if schema.Object == nil {
		return schema, nil
	}

	resolved, release, handled := builder.resolvedObjectForReference(schema.Object)
	if !handled {
		return schema, nil
	}

	if resolved == nil {
		return schemaValue{}, release
	}

	return schemaValue{Object: resolved}, release
}

// sequenceItemSchema selects best schema for sequence item annotations.
func sequenceItemSchema(schema schemaValue) schemaValue {
	if schema.Object == nil {
		return schemaValue{}
	}

	if item, ok := toSchemaValue(schema.Object["items"]); ok {
		return item
	}

	prefixItems := asSlice(schema.Object["prefixItems"])
	for _, raw := range prefixItems {
		item, ok := toSchemaValue(raw)
		if ok {
			return item
		}
	}

	return schemaValue{}
}

// schemaKeyComment builds YAML key comment from schema title and description.
func schemaKeyComment(schema schemaValue) string {
	if schema.Object == nil {
		return ""
	}

	title := strings.TrimSpace(asString(schema.Object["title"]))
	description := strings.TrimSpace(asString(schema.Object["description"]))

	switch {
	case title == "" && description == "":
		return ""
	case title == "":
		return normalizeYAMLComment(description)
	case description == "":
		return normalizeYAMLComment(title)
	default:
		if title == description {
			return normalizeYAMLComment(title)
		}

		return normalizeYAMLComment(title + "\n" + description)
	}
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
		return yamlScalarNode("!!float", strconv.FormatFloat(float64(typed), 'g', -1, 64)), nil

	case float64:
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
