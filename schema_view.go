// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"maps"
	"regexp"
)

// schemaSemanticView expands schema structure while retaining simultaneous constraints.
type schemaSemanticView struct {
	resolver localSchemaResolver
	dialect  schemaDialect
}

// effectiveSchema is the semantic view of one schema location.
//
// Terms are conjunctive: every term applies at the same location.
// Composition branches remain grouped so a later materializer
// can evaluate them instead of selecting one syntactically first.
// Terms describe validation semantics;
// annotation accessors below describe schemadoc presentation precedence.
type effectiveSchema struct {
	terms             []schemaValue
	compositionGroups []schemaComposition
	referenceKeys     []string
}

// schemaComposition is one anyOf or oneOf group preserved for later evaluation.
type schemaComposition struct {
	branches []effectiveSchema
	kind     schemaCompositionKind
}

// schemaCompositionKind identifies one composition operator.
type schemaCompositionKind uint8

const (
	schemaCompositionAnyOf schemaCompositionKind = iota + 1
	schemaCompositionOneOf
)

// schemaArrayCapabilities describe independent array-keyword capabilities.
//
// Draft 2019-09 keeps the tuple form of items
// while adding the newer contains-count and unevaluated-items keywords.
// Draft 2020-12 changes the tuple syntax,
// so these capabilities cannot be represented by one legacy flag.
type schemaArrayCapabilities struct {
	TupleItems             bool
	AdditionalItems        bool
	PrefixItems            bool
	SchemaItems            bool
	ContainsBounds         bool
	UnevaluatedItems       bool
	ContainsEvaluatesItems bool
}

// effectiveArrayItems describes one term's dialect-specific array keywords.
type effectiveArrayItems = struct {
	PrefixItems     *[]effectiveSchema
	Items           *effectiveSchema
	AdditionalItems *effectiveSchema
}

// effectiveArrayContains describes one simultaneous contains requirement.
type effectiveArrayContains struct {
	Schema     effectiveSchema
	Minimum    int
	Maximum    int
	HasMaximum bool
}

// asSchemaValue returns a loss-aware schema representation for legacy callers.
// Multiple terms are kept under allOf so simultaneous constraints are not overwritten
// while the semantic accessors remain authoritative.
func (schema effectiveSchema) asSchemaValue() schemaValue {
	if len(schema.terms) == 1 && len(schema.compositionGroups) == 0 {
		return schema.terms[0]
	}

	allOfBranches := make([]any, 0, len(schema.terms)+len(schema.compositionGroups))
	for _, term := range schema.terms {
		allOfBranches = append(allOfBranches, rawSchemaValue(term))
	}

	for _, keyword := range []string{"anyOf", "oneOf"} {
		for _, effectiveBranches := range schema.compositions(keyword) {
			branches := make([]any, 0, len(effectiveBranches))
			for _, branch := range effectiveBranches {
				branches = append(branches, rawSchemaValue(branch.asSchemaValue()))
			}
			allOfBranches = append(allOfBranches, map[string]any{keyword: branches})
		}
	}

	object := map[string]any{"allOf": allOfBranches}
	return schemaValue{Object: object}
}

// rawSchemaValue converts a normalized schema value back to JSON-like data.
func rawSchemaValue(value schemaValue) any {
	if value.Bool != nil {
		return *value.Bool
	}

	return value.Object
}

// newSchemaSemanticView creates a semantic view for one parsed document.
func newSchemaSemanticView(doc schemaDocument) schemaSemanticView {
	return schemaSemanticView{
		resolver: newLocalSchemaResolver(doc.Raw),
		dialect:  doc.Dialect,
	}
}

// newSchemaSemanticViewPointer creates a view suitable for an example builder.
func newSchemaSemanticViewPointer(doc schemaDocument) *schemaSemanticView {
	view := newSchemaSemanticView(doc)
	return &view
}

// effective expands one schema value into a keyword-aware semantic view.
func (view *schemaSemanticView) effective(node schemaValue) (effectiveSchema, error) {
	return view.expand(node, make(map[string]struct{}))
}

// expand resolves references and composition without flattening constraints.
func (view *schemaSemanticView) expand(node schemaValue, active map[string]struct{}) (effectiveSchema, error) {
	if node.Bool != nil {
		return effectiveSchema{terms: []schemaValue{node}}, nil
	}
	if node.Object == nil {
		return effectiveSchema{}, nil
	}

	object := node.Object
	ref := asString(object["$ref"])
	if ref != "" {
		if _, exists := active[ref]; exists {
			return effectiveSchema{}, &schemaReferenceError{
				Kind:      schemaReferenceCycle,
				Reference: ref,
			}
		}

		active[ref] = struct{}{}
		target, err := view.resolver.lookup(ref)
		if err != nil {
			delete(active, ref)
			return effectiveSchema{}, err
		}

		resolved, err := view.expand(target, active)
		delete(active, ref)
		if err != nil {
			return effectiveSchema{}, err
		}
		resolved.referenceKeys = append(resolved.referenceKeys, ref)

		if !view.refSiblingsApply() {
			return resolved, nil
		}

		local := schemaValue{Object: withoutSchemaKeywords(object, "$ref")}
		localView, err := view.expandLocal(local, active)
		if err != nil {
			return effectiveSchema{}, err
		}

		return combineEffectiveSchemas(resolved, localView), nil
	}

	return view.expandLocal(node, active)
}

// expandLocal expands composition keywords from one non-reference object.
func (view *schemaSemanticView) expandLocal(node schemaValue, active map[string]struct{}) (effectiveSchema, error) {
	if node.Object == nil {
		return effectiveSchema{terms: []schemaValue{node}}, nil
	}

	object := node.Object
	local := schemaValue{Object: withoutSchemaKeywords(object, "allOf", "anyOf", "oneOf")}
	result := effectiveSchema{terms: []schemaValue{local}}

	for _, raw := range asSlice(object["allOf"]) {
		branch, ok := toSchemaValue(raw)
		if !ok {
			continue
		}

		expanded, err := view.expand(branch, active)
		if err != nil {
			return effectiveSchema{}, err
		}
		result = combineEffectiveSchemas(result, expanded)
	}

	for _, keyword := range []string{"anyOf", "oneOf"} {
		branches := make([]effectiveSchema, 0)
		for _, raw := range asSlice(object[keyword]) {
			branch, ok := toSchemaValue(raw)
			if !ok {
				continue
			}

			expanded, err := view.expand(branch, active)
			if err != nil {
				return effectiveSchema{}, err
			}
			branches = append(branches, expanded)
		}
		if len(branches) > 0 {
			result.compositionGroups = append(result.compositionGroups, schemaComposition{
				kind:     compositionKind(keyword),
				branches: branches,
			})
		}
	}

	return result, nil
}

// refSiblingsApply reports whether this dialect treats $ref as an applicator.
func (view *schemaSemanticView) refSiblingsApply() bool {
	return view.dialect == schemaDialect201909 || view.dialect == schemaDialect202012
}

// arrayCapabilities returns the independent array semantics for the dialect.
// Unknown schemas retain the historical Draft 7-compatible behavior.
func (view *schemaSemanticView) arrayCapabilities() schemaArrayCapabilities {
	switch view.dialect {
	case schemaDialect201909:
		return schemaArrayCapabilities{
			TupleItems:             true,
			AdditionalItems:        true,
			SchemaItems:            true,
			ContainsBounds:         true,
			UnevaluatedItems:       true,
			ContainsEvaluatesItems: false,
		}

	case schemaDialect202012:
		return schemaArrayCapabilities{
			PrefixItems:            true,
			SchemaItems:            true,
			ContainsBounds:         true,
			UnevaluatedItems:       true,
			ContainsEvaluatesItems: true,
		}

	default:
		return schemaArrayCapabilities{
			TupleItems:      true,
			AdditionalItems: true,
			SchemaItems:     true,
		}
	}
}

// combineEffectiveSchemas joins two conjunctive views without overwriting terms.
func combineEffectiveSchemas(left, right effectiveSchema) effectiveSchema {
	combined := effectiveSchema{
		terms:             make([]schemaValue, 0, len(left.terms)+len(right.terms)),
		compositionGroups: make([]schemaComposition, 0, len(left.compositionGroups)+len(right.compositionGroups)),
		referenceKeys:     make([]string, 0, len(left.referenceKeys)+len(right.referenceKeys)),
	}
	combined.terms = append(combined.terms, left.terms...)
	combined.terms = append(combined.terms, right.terms...)
	combined.compositionGroups = append(combined.compositionGroups, left.compositionGroups...)
	combined.compositionGroups = append(combined.compositionGroups, right.compositionGroups...)
	combined.referenceKeys = append(combined.referenceKeys, left.referenceKeys...)
	combined.referenceKeys = append(combined.referenceKeys, right.referenceKeys...)

	return combined
}

// properties returns property schemas combined by name without losing terms.
func (schema effectiveSchema) properties(view *schemaSemanticView) (map[string]effectiveSchema, error) {
	properties := make(map[string]effectiveSchema)
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		for name, raw := range mapSchemaValues(term.Object["properties"]) {
			expanded, err := view.expand(raw, make(map[string]struct{}))
			if err != nil {
				return nil, err
			}

			if current, exists := properties[name]; exists {
				properties[name] = combineEffectiveSchemas(current, expanded)
			} else {
				properties[name] = expanded
			}
		}
	}

	return properties, nil
}

// property resolves the schema terms that apply to one emitted object key.
//
// A declared property and every matching pattern property are conjunctive.
// additionalProperties applies only for a term where neither of those rules matched;
// this mirrors JSON Schema object evaluation for dynamic keys.
func (schema effectiveSchema) property(view *schemaSemanticView, name string) (effectiveSchema, bool, error) {
	selected := make([]effectiveSchema, 0)
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		properties := mapSchemaValues(term.Object["properties"])
		property, declared := properties[name]
		matchedPattern := false

		if declared {
			expanded, err := view.expand(property, make(map[string]struct{}))
			if err != nil {
				return effectiveSchema{}, false, err
			}
			selected = append(selected, expanded)
		}

		patterns, _ := term.Object["patternProperties"].(map[string]any)
		for _, pattern := range sortedKeys(patterns) {
			compiled, err := regexp.Compile(pattern)
			if err != nil || !compiled.MatchString(name) {
				continue
			}

			raw, exists := patterns[pattern]
			if !exists {
				continue
			}
			patternSchema, valid := toSchemaValue(raw)
			if !valid {
				continue
			}

			matchedPattern = true
			expanded, err := view.expand(patternSchema, make(map[string]struct{}))
			if err != nil {
				return effectiveSchema{}, false, err
			}
			selected = append(selected, expanded)
		}

		if declared || matchedPattern {
			continue
		}

		additional, exists := toSchemaValue(term.Object["additionalProperties"])
		if !exists {
			continue
		}

		expanded, err := view.expand(additional, make(map[string]struct{}))
		if err != nil {
			return effectiveSchema{}, false, err
		}

		selected = append(selected, expanded)
	}

	for _, group := range schema.compositionGroups {
		for _, branch := range group.branches {
			property, found, err := branch.property(view, name)
			if err != nil {
				return effectiveSchema{}, false, err
			}
			if found {
				selected = append(selected, property)
			}
		}
	}

	if len(selected) == 0 {
		return effectiveSchema{}, false, nil
	}

	combined := selected[0]
	for _, item := range selected[1:] {
		combined = combineEffectiveSchemas(combined, item)
	}

	return combined, true, nil
}

// required returns a stable union because all terms apply simultaneously.
func (schema effectiveSchema) required() []string {
	seen := make(map[string]struct{})
	var result []string
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		for _, name := range asStringSlice(term.Object["required"]) {
			if _, exists := seen[name]; exists {
				continue
			}

			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	return result
}

// annotation returns the last declared annotation in semantic source order.
// This is presentation precedence, not a validation merge rule.
func (schema effectiveSchema) annotation(keyword string) (any, bool) {
	var value any
	found := false
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}
		if candidate, exists := term.Object[keyword]; exists {
			value, found = candidate, true
		}
	}

	return value, found
}

// types returns every declared type constraint in semantic source order.
func (schema effectiveSchema) types() []any {
	return schema.keywordValues("type")
}

// consts returns every declared const constraint in semantic source order.
func (schema effectiveSchema) consts() []any {
	return schema.keywordValues("const")
}

// enums returns every declared enum constraint in semantic source order.
func (schema effectiveSchema) enums() []any {
	return schema.keywordValues("enum")
}

// keywordValues returns raw values for a keyword from all conjunctive terms.
func (schema effectiveSchema) keywordValues(keyword string) []any {
	values := make([]any, 0)
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		if value, exists := term.Object[keyword]; exists {
			values = append(values, value)
		}
	}

	return values
}

// arrayItems returns dialect-specific item views for each simultaneous term.
func (schema effectiveSchema) arrayItems(view *schemaSemanticView) ([]effectiveArrayItems, error) {
	capabilities := view.arrayCapabilities()
	items := make([]effectiveArrayItems, 0)
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		current := effectiveArrayItems{}
		applicable := false
		if capabilities.PrefixItems {
			if prefix := asSlice(term.Object["prefixItems"]); len(prefix) > 0 {
				expanded, err := expandSchemaSlice(view, prefix)
				if err != nil {
					return nil, err
				}
				current.PrefixItems = &expanded
				applicable = true
			}
		}

		if rawItems, exists := term.Object["items"]; exists {
			if tuple, ok := rawItems.([]any); ok {
				if capabilities.TupleItems {
					expanded, err := expandSchemaSlice(view, tuple)
					if err != nil {
						return nil, err
					}
					current.PrefixItems = &expanded
					applicable = true
				}
			} else if capabilities.SchemaItems {
				item, ok := toSchemaValue(rawItems)
				if !ok {
					continue
				}

				expanded, err := view.expand(item, make(map[string]struct{}))
				if err == nil {
					current.Items = &expanded
					applicable = true
				} else {
					return nil, err
				}
			}
		}

		if capabilities.AdditionalItems {
			if rawAdditional, exists := term.Object["additionalItems"]; exists {
				if additional, ok := toSchemaValue(rawAdditional); ok {
					expanded, err := view.expand(additional, make(map[string]struct{}))
					if err == nil {
						current.AdditionalItems = &expanded
						applicable = true
					} else {
						return nil, err
					}
				}
			}
		}
		if applicable {
			items = append(items, current)
		}
	}

	return items, nil
}

// arrayContains returns all active contains requirements in source order.
func (schema effectiveSchema) arrayContains(view *schemaSemanticView) ([]effectiveArrayContains, error) {
	capabilities := view.arrayCapabilities()
	result := make([]effectiveArrayContains, 0)
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		raw, exists := term.Object["contains"]
		if !exists {
			continue
		}

		contains, ok := toSchemaValue(raw)
		if !ok {
			continue
		}
		expanded, err := view.expand(contains, make(map[string]struct{}))
		if err != nil {
			return nil, err
		}

		minimum := 1
		maximum := 0
		hasMaximum := false
		if capabilities.ContainsBounds {
			if value, ok := integerKeyword(term.Object, "minContains"); ok {
				minimum = value
			}
			if value, ok := integerKeyword(term.Object, "maxContains"); ok {
				maximum = value
				hasMaximum = true
			}
		}

		result = append(result, effectiveArrayContains{
			Schema:     expanded,
			Minimum:    minimum,
			Maximum:    maximum,
			HasMaximum: hasMaximum,
		})
	}

	return result, nil
}

// unevaluatedItems returns active modern unevaluated-item schemas.
func (schema effectiveSchema) unevaluatedItems(view *schemaSemanticView) ([]effectiveSchema, error) {
	if !view.arrayCapabilities().UnevaluatedItems {
		return nil, nil
	}

	return schema.expandKeywordSchemas(view, "unevaluatedItems")
}

// additionalProperties returns all applicable additional-property schemas.
func (schema effectiveSchema) additionalProperties(view *schemaSemanticView) ([]effectiveSchema, error) {
	return schema.expandKeywordSchemas(view, "additionalProperties")
}

// propertyNames returns all applicable property-name schemas.
func (schema effectiveSchema) propertyNames(view *schemaSemanticView) ([]effectiveSchema, error) {
	return schema.expandKeywordSchemas(view, "propertyNames")
}

// patternProperties returns deterministic pattern-property schema maps.
func (schema effectiveSchema) patternProperties(view *schemaSemanticView) ([]map[string]effectiveSchema, error) {
	result := make([]map[string]effectiveSchema, 0)
	for _, term := range schema.terms {
		patterns := make(map[string]effectiveSchema)
		if term.Object == nil {
			continue
		}

		for pattern, raw := range mapSchemaValues(term.Object["patternProperties"]) {
			expanded, err := view.expand(raw, make(map[string]struct{}))
			if err == nil {
				patterns[pattern] = expanded
			} else {
				return nil, err
			}
		}
		if len(patterns) > 0 {
			result = append(result, patterns)
		}
	}

	return result, nil
}

// compositions returns preserved anyOf/oneOf groups in source order.
func (schema effectiveSchema) compositions(kind string) [][]effectiveSchema {
	result := make([][]effectiveSchema, 0)
	for _, composition := range schema.compositionGroups {
		if composition.kind == compositionKind(kind) {
			result = append(result, composition.branches)
		}
	}

	return result
}

// compositionKind converts a composition keyword to its internal kind.
func compositionKind(keyword string) schemaCompositionKind {
	if keyword == "oneOf" {
		return schemaCompositionOneOf
	}

	return schemaCompositionAnyOf
}

// expandKeywordSchemas expands schema-valued occurrences of one keyword.
func (schema effectiveSchema) expandKeywordSchemas(view *schemaSemanticView, keyword string) ([]effectiveSchema, error) {
	result := make([]effectiveSchema, 0)
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		value, exists := toSchemaValue(term.Object[keyword])
		if !exists {
			continue
		}

		expanded, err := view.expand(value, make(map[string]struct{}))
		if err == nil {
			result = append(result, expanded)
		} else {
			return nil, err
		}
	}

	return result, nil
}

// expandSchemaSlice expands valid schema values from one raw array.
func expandSchemaSlice(view *schemaSemanticView, raw []any) ([]effectiveSchema, error) {
	result := make([]effectiveSchema, 0, len(raw))
	for _, item := range raw {
		node, ok := toSchemaValue(item)
		if !ok {
			continue
		}

		expanded, err := view.expand(node, make(map[string]struct{}))
		if err == nil {
			result = append(result, expanded)
		} else {
			return nil, err
		}
	}

	return result, nil
}

// withoutSchemaKeywords copies an object while removing selected schema keywords.
func withoutSchemaKeywords(object map[string]any, keywords ...string) map[string]any {
	if len(keywords) == 0 {
		return object
	}

	removed := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		removed[keyword] = struct{}{}
	}

	result := make(map[string]any, len(object))
	maps.Copy(result, object)
	for keyword := range removed {
		delete(result, keyword)
	}

	return result
}
