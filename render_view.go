// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"errors"
	"slices"
	"sort"
	"strings"
)

// definitionEdge is one graph edge from a definition property path to another definition.
type definitionEdge struct {
	Path   string
	Target string
}

// definitionPathState is one BFS queue item for definition path traversal.
type definitionPathState struct {
	Definition string
	Prefix     string
	Depth      int
}

// buildRenderView prepares data for markdown template rendering.
func buildRenderView(doc schemaDocument, opt Options) (renderView, error) {
	title := strings.TrimSpace(opt.Title)
	if title == "" {
		title = defaultTitle
	}

	wrapWidth := normalizeWrapWidth(opt.WrapWidth)
	listMarker := normalizeListMarker(opt.ListMarker)

	sourcePath := strings.TrimSpace(opt.SourcePath)
	if sourcePath == "" {
		sourcePath = "(memory)"
	}

	rootName := rootDefinitionName(doc.Ref)
	definitions := renderDefinitions(doc, rootName)
	defOrder := definitionOrder(definitions, rootName)
	if len(defOrder) == 0 {
		return renderView{}, errors.New("schema has no definitions to render")
	}

	rootDefinition := defOrder[0]
	definitionPaths := buildDefinitionPaths(definitions, rootDefinition)

	view := renderView{
		Title:              sanitizeText(title),
		SourceSchema:       escapeInline(sourcePath),
		SchemaID:           escapeInline(orNone(doc.ID)),
		SchemaDraft:        escapeInline(orNone(doc.Schema)),
		SchemaDraftSupport: draftSupportText(doc.Draft),
		RootRef:            escapeInline(orNone(doc.Ref)),
		ListMarker:         listMarker,
		Definitions:        make([]definitionView, 0, len(defOrder)),
	}

	for _, defName := range defOrder {
		node := definitions[defName]
		if node.isZero() {
			continue
		}

		definition := definitionView{
			Name:        escapeInline(defName),
			Description: formatDescriptionMarkdown(nodeDescription(node), wrapWidth, listMarker),
			Attributes:  schemaAttributes(node, nil),
		}

		properties := nodeProperties(node)
		required := nodeRequired(node)
		order := propertyOrder(required, properties)
		definition.HasProperties = len(order) > 0
		definition.Properties = make([]propertyView, 0, len(order))

		basePaths := definitionPaths[defName]
		isRootDefinition := defName == rootDefinition
		for _, propName := range order {
			prop := properties[propName]
			propRequired := isRequired(required, propName)

			paths := buildPropertyPaths(basePaths, propName, isRootDefinition)
			escapedPaths := make([]string, 0, len(paths))
			for _, path := range paths {
				escapedPaths = append(escapedPaths, escapeInline(path))
			}

			definition.Properties = append(definition.Properties, propertyView{
				Heading:     escapeInline(defName + "." + propertyHeadingName(propName, prop)),
				Name:        escapeInline(propName),
				Paths:       escapedPaths,
				Description: formatDescriptionMarkdown(nodeDescription(prop), wrapWidth, listMarker),
				Attributes:  schemaAttributes(prop, &propRequired),
			})
		}

		view.Definitions = append(view.Definitions, definition)
	}

	if len(view.Definitions) == 0 {
		return renderView{}, errors.New("schema has no renderable definitions")
	}

	return view, nil
}

// propertyHeadingName selects property heading suffix based on referenced definition name.
func propertyHeadingName(key string, prop schemaValue) string {
	if prop.Object == nil {
		return key
	}

	refName := rootDefinitionName(asString(prop.Object["$ref"]))
	if refName != "" {
		return refName
	}

	return key
}

// buildDefinitionPaths finds all reachable JSON paths for every definition from root definition.
func buildDefinitionPaths(definitions map[string]schemaValue, rootDefinition string) map[string][]string {
	if strings.TrimSpace(rootDefinition) == "" {
		return nil
	}

	if _, ok := definitions[rootDefinition]; !ok {
		return nil
	}

	paths := map[string][]string{
		rootDefinition: {""},
	}
	seen := map[string]struct{}{
		rootDefinition + "\x00": {},
	}

	queue := []definitionPathState{{
		Definition: rootDefinition,
		Depth:      0,
		Prefix:     "",
	}}

	const maxDepth = 20

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.Depth >= maxDepth {
			continue
		}

		node := definitions[current.Definition]
		edges := definitionEdges(node)
		for _, edge := range edges {
			if edge.Target == rootDefinition {
				continue
			}

			nextPrefix := appendPath(current.Prefix, edge.Path)
			if strings.TrimSpace(nextPrefix) == "" {
				continue
			}

			seenKey := edge.Target + "\x00" + nextPrefix
			if _, ok := seen[seenKey]; ok {
				continue
			}

			seen[seenKey] = struct{}{}
			paths[edge.Target] = append(paths[edge.Target], nextPrefix)
			queue = append(queue, definitionPathState{
				Definition: edge.Target,
				Depth:      current.Depth + 1,
				Prefix:     nextPrefix,
			})
		}
	}

	for name, values := range paths {
		sort.Strings(values)
		paths[name] = values
	}

	return paths
}

// buildPropertyPaths builds normalized root-relative JSON paths for one property.
func buildPropertyPaths(basePaths []string, propertyName string, hideRootPath bool) []string {
	propertyName = strings.TrimSpace(propertyName)
	if propertyName == "" {
		return nil
	}

	if len(basePaths) == 0 {
		return nil
	}

	dedup := make(map[string]struct{}, len(basePaths))
	for _, base := range basePaths {
		path := appendPath(base, propertyName)
		if strings.TrimSpace(path) == "" {
			continue
		}

		if hideRootPath && path == propertyName {
			continue
		}

		dedup[path] = struct{}{}
	}

	if len(dedup) == 0 {
		return nil
	}

	out := make([]string, 0, len(dedup))
	for path := range dedup {
		out = append(out, path)
	}

	sort.Strings(out)
	return out
}

// definitionEdges extracts graph edges from one definition object.
func definitionEdges(node schemaValue) []definitionEdge {
	if node.Object == nil {
		return nil
	}

	properties := nodeProperties(node)
	if len(properties) == 0 {
		return nil
	}

	edgeMap := make(map[string]definitionEdge)
	for _, name := range sortedSchemaValueKeys(properties) {
		collectDefinitionEdges(properties[name], name, edgeMap)
	}

	if len(edgeMap) == 0 {
		return nil
	}

	keys := make([]string, 0, len(edgeMap))
	for key := range edgeMap {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	out := make([]definitionEdge, 0, len(keys))
	for _, key := range keys {
		out = append(out, edgeMap[key])
	}

	return out
}

// collectDefinitionEdges recursively collects all referenced definitions under one schema node.
func collectDefinitionEdges(schema schemaValue, path string, edgeMap map[string]definitionEdge) {
	if schema.Object == nil {
		return
	}

	object := schema.Object
	if target := rootDefinitionName(asString(object["$ref"])); target != "" {
		addDefinitionEdge(edgeMap, path, target)
	}

	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		for _, value := range asSlice(object[keyword]) {
			collectDefinitionEdgesAny(value, path, edgeMap)
		}
	}

	for _, keyword := range []string{"if", "then", "else", "not", "contentSchema"} {
		collectDefinitionEdgesAny(object[keyword], path, edgeMap)
	}

	for _, keyword := range []string{"items", "prefixItems", "contains", "additionalItems", "unevaluatedItems"} {
		collectDefinitionEdgesAny(object[keyword], appendPath(path, "[]"), edgeMap)
	}

	for _, keyword := range []string{"additionalProperties", "unevaluatedProperties"} {
		collectDefinitionEdgesAny(object[keyword], appendPath(path, "[]"), edgeMap)
	}

	if nested := mapSchemaValues(object["properties"]); len(nested) > 0 {
		for _, key := range sortedSchemaValueKeys(nested) {
			collectDefinitionEdges(nested[key], appendPath(path, key), edgeMap)
		}
	}

	if nested := mapSchemaValues(object["patternProperties"]); len(nested) > 0 {
		for _, key := range sortedSchemaValueKeys(nested) {
			collectDefinitionEdges(nested[key], appendPath(path, key), edgeMap)
		}
	}
}

// collectDefinitionEdgesAny unwraps arrays and forwards schema-like values to edge collector.
func collectDefinitionEdgesAny(raw any, path string, edgeMap map[string]definitionEdge) {
	switch typed := raw.(type) {
	case []any:
		for _, value := range typed {
			collectDefinitionEdgesAny(value, path, edgeMap)
		}
	default:
		value, ok := toSchemaValue(raw)
		if !ok {
			return
		}

		collectDefinitionEdges(value, path, edgeMap)
	}
}

// addDefinitionEdge stores one unique edge key in edge map.
func addDefinitionEdge(edgeMap map[string]definitionEdge, path, target string) {
	path = strings.TrimSpace(path)
	target = strings.TrimSpace(target)
	if path == "" || target == "" {
		return
	}

	edge := definitionEdge{Path: path, Target: target}
	edgeMap[target+"\x00"+path] = edge
}

// appendPath joins path segments with a dot while preserving empty root prefix.
func appendPath(base, segment string) string {
	base = strings.TrimSpace(base)
	segment = strings.TrimSpace(segment)
	if base == "" {
		return segment
	}

	if segment == "" {
		return base
	}

	return base + "." + segment
}

// sortedSchemaValueKeys returns deterministic sorted keys for schema maps.
func sortedSchemaValueKeys(values map[string]schemaValue) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}

	sort.Strings(out)
	return out
}

// renderDefinitions returns definitions map and synthesizes root when schema has none.
func renderDefinitions(doc schemaDocument, rootName string) map[string]schemaValue {
	if len(doc.Defs) > 0 {
		return doc.Defs
	}

	name := strings.TrimSpace(rootName)
	if name == "" {
		name = "Root"
	}

	return map[string]schemaValue{name: doc.Root}
}

// draftSupportText formats draft support marker for markdown metadata block.
func draftSupportText(info DraftInfo) string {
	if !info.Supported {
		if strings.TrimSpace(info.Canonical) != "" {
			return "unknown (" + escapeInline(info.Canonical) + ")"
		}

		return "unknown"
	}

	return "supported (" + escapeInline(info.Canonical) + ")"
}

// definitionOrder returns deterministic definition rendering order with root first.
func definitionOrder(defs map[string]schemaValue, rootName string) []string {
	keys := make([]string, 0, len(defs))
	for name := range defs {
		keys = append(keys, name)
	}

	sort.Strings(keys)
	if len(keys) == 0 {
		return nil
	}

	root := strings.TrimSpace(rootName)
	if root == "" {
		if _, ok := defs["Config"]; ok {
			root = "Config"
		} else {
			root = keys[0]
		}
	}

	if _, ok := defs[root]; !ok {
		return keys
	}

	out := make([]string, 0, len(keys))
	out = append(out, root)
	for _, name := range keys {
		if name == root {
			continue
		}

		out = append(out, name)
	}

	return out
}

// propertyOrder returns required properties first, then optional sorted properties.
func propertyOrder(required []string, properties map[string]schemaValue) []string {
	if len(properties) == 0 {
		return nil
	}

	out := make([]string, 0, len(properties))
	seen := make(map[string]struct{}, len(properties))

	for _, name := range required {
		if _, ok := properties[name]; !ok {
			continue
		}

		if _, exists := seen[name]; exists {
			continue
		}

		seen[name] = struct{}{}
		out = append(out, name)
	}

	optional := make([]string, 0, len(properties))
	for name := range properties {
		if _, exists := seen[name]; exists {
			continue
		}

		optional = append(optional, name)
	}

	sort.Strings(optional)
	out = append(out, optional...)
	return out
}

// rootDefinitionName extracts definition name from local JSON pointer reference.
func rootDefinitionName(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}

	for _, prefix := range []string{"#/$defs/", "#/definitions/"} {
		if !strings.HasPrefix(ref, prefix) {
			continue
		}

		path := strings.TrimPrefix(ref, prefix)
		if path == "" {
			return ""
		}

		parts := strings.Split(path, "/")
		return parts[0]
	}

	return ""
}

// isRequired reports whether property key is present in required list.
func isRequired(required []string, key string) bool {
	return slices.Contains(required, key)
}

// nodeDescription extracts description from schema node object.
func nodeDescription(node schemaValue) string {
	if node.Object == nil {
		return ""
	}

	return asString(node.Object["description"])
}

// nodeProperties extracts child property schemas from schema node object.
func nodeProperties(node schemaValue) map[string]schemaValue {
	if node.Object == nil {
		return nil
	}

	return mapSchemaValues(node.Object["properties"])
}

// nodeRequired extracts required property list from schema node object.
func nodeRequired(node schemaValue) []string {
	if node.Object == nil {
		return nil
	}

	return asStringSlice(node.Object["required"])
}
