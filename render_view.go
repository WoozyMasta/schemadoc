// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"errors"
	"path"
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
		Description:        sanitizeText(strings.TrimSpace(opt.Description)),
		SourceSchema:       escapeInline(sourcePath),
		SourceFileURL:      buildSourceFileURL(sourcePath, doc.ID),
		SchemaSourceURL:    buildSchemaSourceURL(doc.ID, sourcePath),
		SchemaBrowserURL:   buildSchemaBrowserURL(doc.ID, sourcePath),
		SchemaID:           escapeInline(orNone(doc.ID)),
		SchemaDraft:        escapeInline(orNone(doc.Schema)),
		SchemaDraftSupport: draftSupportText(doc.Draft),
		RootRef:            escapeInline(orNone(doc.Ref)),
		ListMarker:         listMarker,
		FooterToolName:     sanitizeText(strings.TrimSpace(opt.FooterToolName)),
		FooterToolURL:      sanitizeText(strings.TrimSpace(opt.FooterToolURL)),
		FooterVersion:      sanitizeText(strings.TrimSpace(opt.FooterVersion)),
		FooterCommit:       sanitizeText(strings.TrimSpace(opt.FooterCommit)),
		Contents:           buildContents(definitions, rootDefinition, defOrder),
		Definitions:        make([]definitionView, 0, len(defOrder)),
	}
	pathAnchors := make(map[string]string)
	ambiguousPathAnchors := make(map[string]struct{})

	for _, defName := range defOrder {
		node := definitions[defName]
		if node.isZero() {
			continue
		}

		definition := definitionView{
			Name:        escapeInline(defName),
			Description: formatDescriptionMarkdown(nodeDescription(node), wrapWidth, listMarker),
			Attributes:  schemaAttributes(node, nil, opt.HideExtraKeywords, opt.ShowInternalKeywords),
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
			headingText := defName + "." + propertyHeadingName(propName, prop)
			headingAnchor := markdownHeadingAnchor(headingText)
			allPaths := buildPropertyPaths(basePaths, propName, false)
			paths := filterPropertyPaths(allPaths, propertyPathFilterOptions{
				HideRootPath: isRootDefinition,
				RootPath:     strings.TrimSpace(propName),
			})
			indexPathAnchors(pathAnchors, ambiguousPathAnchors, allPaths, headingAnchor)

			definition.Properties = append(definition.Properties, propertyView{
				Heading:     escapeInline(headingText),
				Name:        escapeInline(propName),
				Paths:       paths,
				Description: formatDescriptionMarkdown(nodeDescription(prop), wrapWidth, listMarker),
				Attributes:  schemaAttributes(prop, &propRequired, opt.HideExtraKeywords, opt.ShowInternalKeywords),
			})
		}

		view.Definitions = append(view.Definitions, definition)
	}

	if len(view.Definitions) == 0 {
		return renderView{}, errors.New("schema has no renderable definitions")
	}
	applyPathLinks(&view, pathAnchors)

	return view, nil
}

// buildSourceFileURL returns best clickable URL for source file.
func buildSourceFileURL(sourcePath, schemaID string) string {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" || sourcePath == "(stdin)" || sourcePath == "(memory)" {
		return ""
	}

	if browser := buildSchemaBrowserURL(schemaID, sourcePath); browser != "" {
		return browser
	}

	if strings.Contains(sourcePath, ":\\") || strings.HasPrefix(sourcePath, "/") {
		return ""
	}

	return strings.ReplaceAll(sourcePath, "\\", "/")
}

// buildSchemaSourceURL builds raw GitHub URL for source schema file when possible.
func buildSchemaSourceURL(schemaID, sourcePath string) string {
	return buildGitHubFileURL(schemaID, sourcePath, true)
}

// SchemaSourceURL returns a raw source URL for a file-backed schema when possible.
func SchemaSourceURL(schemaBytes []byte, sourcePath string) string {
	doc, err := parseDocument(schemaBytes)
	if err != nil {
		return ""
	}

	return buildSchemaSourceURL(doc.ID, sourcePath)
}

// buildSchemaBrowserURL builds browser-friendly GitHub file URL when possible.
func buildSchemaBrowserURL(schemaID, sourcePath string) string {
	return buildGitHubFileURL(schemaID, sourcePath, false)
}

// buildGitHubFileURL builds GitHub file URL using HEAD for default branch.
func buildGitHubFileURL(schemaID, sourcePath string, raw bool) string {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" || sourcePath == "(memory)" {
		return ""
	}
	if strings.Contains(sourcePath, ":\\") || strings.HasPrefix(sourcePath, "/") {
		return ""
	}

	schemaID = strings.TrimSpace(schemaID)
	if schemaID == "" {
		return ""
	}

	owner, repo, ok := parseGitHubOwnerRepo(schemaID)
	if !ok {
		return ""
	}

	sourcePath = strings.ReplaceAll(sourcePath, "\\", "/")
	sourcePath = strings.TrimPrefix(path.Clean("/"+sourcePath), "/")
	if sourcePath == "" || sourcePath == "." {
		return ""
	}

	if raw {
		return "https://raw.githubusercontent.com/" + owner + "/" + repo + "/HEAD/" + sourcePath
	}

	return "https://github.com/" + owner + "/" + repo + "/blob/HEAD/" + sourcePath
}

// parseGitHubOwnerRepo extracts owner/repo from common GitHub URL forms.
func parseGitHubOwnerRepo(schemaID string) (string, string, bool) {
	schemaID = strings.TrimSpace(schemaID)
	if schemaID == "" {
		return "", "", false
	}

	var rest string
	var ok bool
	for _, prefix := range []string{
		"https://github.com/",
		"http://github.com/",
		"github.com/",
	} {
		rest, ok = cutPrefixFold(schemaID, prefix)
		if ok {
			break
		}
	}
	if !ok {
		return "", "", false
	}

	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", "", false
	}

	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return "", "", false
	}

	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", "", false
	}

	return owner, repo, true
}

// cutPrefixFold removes prefix when it matches case-insensitively.
func cutPrefixFold(value, prefix string) (string, bool) {
	if len(value) < len(prefix) {
		return "", false
	}

	if !strings.EqualFold(value[:len(prefix)], prefix) {
		return "", false
	}

	return value[len(prefix):], true
}

// buildContents builds a deterministic nested TOC from reference graph.
func buildContents(definitions map[string]schemaValue, rootDefinition string, definitionOrder []string) []tocEntry {
	if len(definitionOrder) == 0 {
		return nil
	}

	adjacency := make(map[string][]string, len(definitions))
	for _, name := range definitionOrder {
		node := definitions[name]
		edges := definitionEdges(node)

		seenTargets := make(map[string]struct{}, len(edges))
		targets := make([]string, 0, len(edges))
		for _, edge := range edges {
			target := strings.TrimSpace(edge.Target)
			if target == "" || target == name {
				continue
			}
			if _, ok := definitions[target]; !ok {
				continue
			}
			if _, ok := seenTargets[target]; ok {
				continue
			}

			seenTargets[target] = struct{}{}
			targets = append(targets, target)
		}

		adjacency[name] = targets
	}

	entries := make([]tocEntry, 0, len(definitionOrder))
	visited := make(map[string]struct{}, len(definitionOrder))
	var walk func(name string, depth int)
	walk = func(name string, depth int) {
		if _, ok := visited[name]; ok {
			return
		}
		if _, ok := definitions[name]; !ok {
			return
		}

		visited[name] = struct{}{}
		indentDepth := max(depth, 0)
		entries = append(entries, tocEntry{
			Name:   escapeInline(name),
			Anchor: markdownHeadingAnchor(name),
			Indent: strings.Repeat("  ", indentDepth),
			Depth:  indentDepth,
		})

		for _, target := range adjacency[name] {
			walk(target, depth+1)
		}
	}

	walk(rootDefinition, 0)
	for _, name := range definitionOrder {
		walk(name, 0)
	}

	return entries
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

// propertyPathFilterOptions defines path filtering options for rendered properties.
type propertyPathFilterOptions struct {
	RootPath     string
	HideRootPath bool
}

// filterPropertyPaths removes root-level property path when requested.
func filterPropertyPaths(paths []string, opts propertyPathFilterOptions) []string {
	if len(paths) == 0 {
		return nil
	}
	if !opts.HideRootPath {
		return paths
	}

	rootPath := strings.TrimSpace(opts.RootPath)
	if rootPath == "" {
		return paths
	}

	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == rootPath {
			continue
		}

		out = append(out, path)
	}

	return out
}

// indexPathAnchors stores deterministic path prefix anchors and skips ambiguous mappings.
func indexPathAnchors(pathAnchors map[string]string, ambiguous map[string]struct{}, paths []string, anchor string) {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" || len(paths) == 0 {
		return
	}

	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		if _, blocked := ambiguous[path]; blocked {
			continue
		}
		if existing, ok := pathAnchors[path]; ok && existing != anchor {
			delete(pathAnchors, path)
			ambiguous[path] = struct{}{}
			continue
		}

		pathAnchors[path] = anchor
	}
}

// applyPathLinks converts property paths to per-segment markdown links where possible.
func applyPathLinks(view *renderView, pathAnchors map[string]string) {
	if view == nil || len(view.Definitions) == 0 || len(pathAnchors) == 0 {
		return
	}

	for i := range view.Definitions {
		definition := &view.Definitions[i]
		for j := range definition.Properties {
			property := &definition.Properties[j]
			if len(property.Paths) == 0 {
				continue
			}

			linkedPaths := make([]string, 0, len(property.Paths))
			for _, path := range property.Paths {
				linkedPaths = append(linkedPaths, buildLinkedPath(path, pathAnchors))
			}

			property.Paths = linkedPaths
		}
	}
}

// buildLinkedPath renders one dotted path with links on intermediate segments.
func buildLinkedPath(path string, pathAnchors map[string]string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return "`" + escapeInline(path) + "`"
	}

	var builder strings.Builder
	prefix := make([]string, 0, len(parts))
	last := len(parts) - 1

	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString(".")
		}

		prefix = append(prefix, part)
		segment := "`" + escapeInline(part) + "`"

		if index < last {
			anchor, ok := pathAnchors[strings.Join(prefix, ".")]
			if ok && strings.TrimSpace(anchor) != "" {
				builder.WriteString("[")
				builder.WriteString(segment)
				builder.WriteString("](#")
				builder.WriteString(anchor)
				builder.WriteString(")")
				continue
			}
		}

		builder.WriteString(segment)
	}

	return builder.String()
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

	out := make([]definitionEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		out = append(out, edge)
	}

	orderedProperties := propertyOrder(nodeRequired(node), properties)
	propertyRank := make(map[string]int, len(orderedProperties))
	for index, name := range orderedProperties {
		propertyRank[name] = index
	}

	sort.SliceStable(out, func(left, right int) bool {
		leftProperty, _, _ := strings.Cut(out[left].Path, ".")
		rightProperty, _, _ := strings.Cut(out[right].Path, ".")
		leftRank, leftOK := propertyRank[leftProperty]
		rightRank, rightOK := propertyRank[rightProperty]

		if leftOK && rightOK && leftRank != rightRank {
			return leftRank < rightRank
		}
		if leftOK != rightOK {
			return leftOK
		}
		if out[left].Path != out[right].Path {
			return out[left].Path < out[right].Path
		}

		return out[left].Target < out[right].Target
	})

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
	visited := make(map[string]struct{}, len(keys))
	var walk func(string)
	walk = func(name string) {
		if _, ok := visited[name]; ok {
			return
		}
		if _, ok := defs[name]; !ok {
			return
		}

		visited[name] = struct{}{}
		out = append(out, name)
		for _, edge := range definitionEdges(defs[name]) {
			walk(edge.Target)
		}
	}

	walk(root)
	for _, name := range keys {
		walk(name)
	}

	return out
}

// propertyOrder returns required properties first, then optional sorted properties.
func propertyOrder(required []string, properties map[string]schemaValue) []string {
	if len(properties) == 0 {
		return nil
	}

	keys := make([]string, 0, len(properties))
	hasSchemaOrder := false
	for key, property := range properties {
		keys = append(keys, key)
		if property.Object == nil {
			continue
		}

		if _, ok := asNumber(property.Object["x-order"]); ok {
			hasSchemaOrder = true
		}
	}

	if hasSchemaOrder {
		return sortKeysBySchemaOrder(keys, properties)
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
