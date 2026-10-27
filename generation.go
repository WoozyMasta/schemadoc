// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"maps"
	"slices"
)

// generationContext separates constructive constraints from full acceptance semantics.
type generationContext struct {
	constraints effectiveSchema
	validation  effectiveSchema
}

// searchBudget bounds speculative work across all nested generation locations.
// Ordinary accepted fields do not spend retries; rejected candidates and repairs do.
type searchBudget struct {
	plans      int
	repairs    int
	candidates int
}

// dependencyActivation identifies a dependency in an append-only plan term list.
type dependencyActivation struct {
	keyword string
	trigger string
	term    int
}

// generationSearchError reports only locally proven contradictions as unsatisfiable.
func generationSearchError(schema effectiveSchema, path schemaPath) error {
	code := MaterializationCodeUnsupported
	category := MaterializationCategoryUnsupportedMaterialization
	cause := ErrUnsupportedMaterialization
	if isFalseEffectiveSchema(schema) || isDefinitelyUnsatisfiable(schema) {
		code = MaterializationCodeUnsatisfiable
		category = MaterializationCategoryUnsatisfiableSchema
		cause = ErrMaterializationUnsatisfiable
	}

	errorPath := unsatisfiableConstraintPath(schema, path)
	if code == MaterializationCodeUnsupported {
		switch {
		case len(schema.compositionGroups) > 0:
			keyword := "anyOf"
			if schema.compositionGroups[0].kind == schemaCompositionOneOf {
				keyword = "oneOf"
			}
			errorPath = materializationKeywordPath(path, keyword)

		case hasSchemaKeyword(schema, "pattern"):
			errorPath = materializationKeywordPath(path, "pattern")
		}
	}

	return newMaterializationError(code, category, errorPath, cause)
}

// searchEffective retains reference guards while searching full-schema candidates.
func (m *exampleMaterializer) searchEffective(
	whole effectiveSchema,
	path schemaPath,
	accept func(any) bool,
) (bool, error) {
	if m.depth >= maxSearchDepth || m.budget.candidates <= 0 {
		return false, generationSearchError(effectiveSchema{}, path)
	}

	m.depth++
	defer func() { m.depth-- }()

	if len(whole.deferredRefs) > 0 {
		value, err := m.materializeDeferredReference(whole, path)
		return err == nil && accept(value), err
	}

	for i, ref := range whole.referenceKeys {
		if slices.Contains(whole.referenceKeys[:i], ref) {
			continue
		}

		if m.activeRefs[ref] > 0 {
			return false, newMaterializationError(
				MaterializationCodeReferenceRecursion,
				MaterializationCategoryReferenceCycle,
				materializationKeywordPath(path, "$ref"),
				ErrSchemaReferenceCycle,
			)
		}

		m.activeRefs[ref]++
		defer func() { m.activeRefs[ref]-- }()
	}

	acceptWhole := func(value any) bool {
		if len(path.segments) == 0 && m.validateRoot(value) != nil {
			m.budget.candidates--
			return false
		}
		return accept(value)
	}

	var lastErr error
	found, err := m.generationPlans(whole, path, func(plan effectiveSchema) (bool, error) {
		found, err := m.searchPlan(generationContext{constraints: plan, validation: whole}, path, acceptWhole)
		if err != nil {
			lastErr = err
		}
		// A failed constructive plan does not disprove other alternatives.
		return found, nil
	})
	if found || err != nil {
		return found, err
	}
	if !m.hasGenerationChoices(whole) && lastErr != nil {
		return false, lastErr
	}

	return false, generationSearchError(whole, path)
}

// generationPlans lazily expands alternatives and conditionals in source order.
// Conditional alternatives keep their own condition, never accumulating both branches.
func (m *exampleMaterializer) generationPlans(
	schema effectiveSchema,
	path schemaPath,
	yield func(effectiveSchema) (bool, error),
) (bool, error) {
	var lastErr error
	var walk func(effectiveSchema, int) (bool, error)
	walk = func(current effectiveSchema, depth int) (bool, error) {
		if depth >= maxSearchDepth {
			return false, nil
		}

		if len(current.compositionGroups) > 0 {
			group := current.compositionGroups[0]
			base := current
			base.compositionGroups = slices.Clone(current.compositionGroups[1:])
			for _, branch := range group.branches {
				if m.budget.plans <= 0 {
					break
				}

				m.budget.plans--
				found, err := walk(combineEffectiveSchemas(base, branch), depth+1)
				if found || err != nil {
					return found, err
				}
			}

			return false, nil
		}

		if m.semantic.dialect != schemaDialectDraft4Compatible && m.semantic.dialect != schemaDialectDraft6 {
			for i, term := range current.terms {
				condition, exists := term.Object["if"]
				if !exists {
					continue
				}

				base := current
				base.terms = slices.Clone(current.terms)
				base.terms[i] = schemaValue{Object: withoutSchemaKeywords(term.Object, "if", "then", "else")}
				for _, keyword := range []string{"then", "else"} {
					if m.budget.plans <= 0 {
						break
					}

					m.budget.plans--
					raw := condition
					if keyword == "else" {
						raw = map[string]any{"not": condition}
					}
					branch := map[string]any{"allOf": []any{raw}}
					if selected, ok := term.Object[keyword]; ok {
						branch["allOf"] = append(branch["allOf"].([]any), selected)
					}

					nested, err := m.semantic.effective(schemaValue{Object: branch})
					if err != nil {
						return false, materializationReferenceError(err, path)
					}

					found, err := walk(combineEffectiveSchemas(base, nested), depth+1)
					if found || err != nil {
						return found, err
					}
				}

				return false, nil
			}
		}

		found, err := yield(current)
		if err != nil {
			lastErr = err
		}
		return found, nil
	}

	found, err := walk(schema, 0)
	if found || err != nil {
		return found, err
	}
	if m.hasGenerationChoices(schema) {
		return false, generationSearchError(schema, path)
	}

	return false, lastErr
}

// hasGenerationChoices reports whether local failure is only one constructive alternative.
func (m *exampleMaterializer) hasGenerationChoices(schema effectiveSchema) bool {
	return len(schema.compositionGroups) > 0 ||
		(hasSchemaKeyword(schema, "if") &&
			m.semantic.dialect != schemaDialectDraft4Compatible && m.semantic.dialect != schemaDialectDraft6)
}

// searchPlan shares explicit candidate ordering and ordinary type builders.
func (m *exampleMaterializer) searchPlan(ctx generationContext, path schemaPath, accept func(any) bool) (bool, error) {
	schema := ctx.constraints
	try := func(value any) bool {
		if m.budget.candidates <= 0 {
			return false
		}

		if validateEffectiveInstance(m.semantic, schema, value, path) != nil ||
			validateEffectiveInstance(m.semantic, ctx.validation, value, path) != nil {
			m.budget.candidates--
			return false
		}

		return accept(value)
	}

	for _, candidate := range m.explicitCandidates(schema) {
		if m.budget.candidates <= 0 {
			break
		}

		if try(candidate.value) {
			return true, nil
		}
	}

	if len(schema.deferredRefs) > 0 {
		value, err := m.materializeDeferredReference(schema, path)
		return err == nil && try(value), err
	}

	properties, err := schema.properties(m.semantic)
	if err != nil {
		return false, materializationReferenceError(err, path)
	}

	typeName := effectiveSchemaType(schema)
	switch {
	case typeName == "object" ||
		(typeName == "" &&
			(len(properties) > 0 ||
				len(schema.required()) > 0 ||
				hasSchemaKeyword(schema, "additionalProperties") ||
				hasSchemaKeyword(schema, "propertyNames") ||
				hasSchemaKeyword(schema, "patternProperties"))):
		return m.searchObject(ctx, path, try)

	case typeName == "array" || (typeName == "" && m.hasArrayItems(schema)):
		value, err := m.synthesizeArray(schema, path)
		return err == nil && try(value), err

	case typeName == "string":
		for _, value := range stringCandidates(schema) {
			if try(value) {
				return true, nil
			}
		}

	case typeName == "integer" || typeName == "number":
		if slices.ContainsFunc(numberCandidates(schema), try) {
			return true, nil
		}

	default:
		if value, ok := scalarPlaceholder(typeName); ok {
			if try(value) {
				return true, nil
			}
			if typeName == "boolean" && try(true) {
				return true, nil
			}
		} else if try(nil) {
			return true, nil
		}
	}

	return false, generationSearchError(schema, path)
}

// searchObject visits subsets by distance from the requested property preference.
func (m *exampleMaterializer) searchObject(
	ctx generationContext,
	path schemaPath,
	accept func(any) bool,
) (bool, error) {
	schema := ctx.constraints
	properties, err := schema.properties(m.semantic)
	if err != nil {
		return false, materializationReferenceError(err, path)
	}

	required := schema.required()
	values := make(map[string]schemaValue, len(properties))
	for name, property := range properties {
		if !isFalseEffectiveSchema(property) {
			values[name] = property.asSchemaValue()
		}
	}

	var optional []string
	for _, name := range propertyOrder(required, values) {
		if !slices.Contains(required, name) {
			optional = append(optional, name)
		}
	}

	return m.searchObjectKeys(ctx, required, optional, path, nil, accept)
}

// searchObjectKeys shares subset ordering with properties introduced by dependencies.
func (m *exampleMaterializer) searchObjectKeys(
	ctx generationContext,
	required, optional []string,
	path schemaPath,
	active map[dependencyActivation]bool,
	accept func(any) bool,
) (bool, error) {
	var lastErr error
	for distance := 0; distance <= len(optional); distance++ {
		found, err := m.visitSubsets(len(optional), distance, func(changed []int) (bool, error) {
			if distance > 0 {
				if m.budget.repairs <= 0 {
					return false, nil
				}

				m.budget.repairs--
			}

			keys := slices.Clone(required)
			for i, name := range optional {
				selected := slices.Contains(changed, i)
				if m.mode == ExampleModeAll {
					selected = !selected
				}
				if selected {
					keys = append(keys, name)
				}
			}

			found, err := m.searchObjectShape(ctx, keys, path, active, accept)
			if err != nil {
				lastErr = err
			}

			return found, nil
		})

		if found || err != nil {
			return found, err
		}
		if m.budget.repairs <= 0 || m.budget.candidates <= 0 {
			break
		}
	}

	if len(optional) == 0 && lastErr != nil {
		return false, lastErr
	}

	return false, generationSearchError(ctx.constraints, path)
}

// visitSubsets enumerates fixed-size edits lazily, without a Cartesian-product allocation.
func (m *exampleMaterializer) visitSubsets(size, count int, yield func([]int) (bool, error)) (bool, error) {
	var walk func(int, []int) (bool, error)
	walk = func(start int, selected []int) (bool, error) {
		if m.budget.repairs <= 0 || m.budget.candidates <= 0 {
			return false, nil
		}
		if len(selected) == count {
			return yield(selected)
		}

		for i := start; i <= size-(count-len(selected)); i++ {
			found, err := walk(i+1, append(selected, i))
			if found || err != nil {
				return found, err
			}
		}

		return false, nil
	}

	return walk(0, nil)
}

// searchObjectShape activates complete dependencies to a fixed point before values.
func (m *exampleMaterializer) searchObjectShape(
	ctx generationContext,
	keys []string,
	path schemaPath,
	active map[dependencyActivation]bool,
	accept func(any) bool,
) (bool, error) {
	schema := ctx.constraints
	for _, name := range schema.required() {
		if !slices.Contains(keys, name) {
			keys = append(slices.Clone(keys), name)
		}
	}

	for i, term := range schema.terms {
		for _, keyword := range m.semantic.dependencyKeywords() {
			dependencies, _ := term.Object[keyword].(map[string]any)

			for _, trigger := range sortedKeys(dependencies) {
				id := dependencyActivation{term: i, keyword: keyword, trigger: trigger}
				if active[id] || !slices.Contains(keys, trigger) {
					continue
				}

				if m.budget.repairs <= 0 {
					return false, generationSearchError(effectiveSchema{}, path)
				}

				m.budget.repairs--
				nextActive := maps.Clone(active)
				if nextActive == nil {
					nextActive = make(map[dependencyActivation]bool)
				}

				nextActive[id] = true
				raw := dependencies[trigger]
				if _, array := raw.([]any); array {
					raw = map[string]any{"required": raw}
				}

				node, valid := toSchemaValue(raw)
				if !valid {
					continue
				}

				nested, err := m.semantic.effective(node)
				if err != nil {
					return false, materializationReferenceError(err, path)
				}

				return m.generationPlans(combineEffectiveSchemas(schema, nested), path, func(plan effectiveSchema) (bool, error) {
					before, err := schema.properties(m.semantic)
					if err != nil {
						return false, materializationReferenceError(err, path)
					}

					after, err := plan.properties(m.semantic)
					if err != nil {
						return false, materializationReferenceError(err, path)
					}

					values := make(map[string]schemaValue)
					for name, property := range after {
						if _, exists := before[name]; !exists && !slices.Contains(keys, name) &&
							!slices.Contains(plan.required(), name) && !isFalseEffectiveSchema(property) {
							values[name] = property.asSchemaValue()
						}
					}

					return m.searchObjectKeys(
						generationContext{constraints: plan, validation: ctx.validation},
						keys, propertyOrder(nil, values), path, nextActive, accept,
					)
				})
			}
		}
	}

	minimum, _, maximum, hasMaximum := objectBounds(schema)
	if hasMaximum && len(keys) > maximum {
		return false, generationSearchError(schema, path)
	}

	propertyNames, err := schema.propertyNames(m.semantic)
	if err != nil {
		return false, materializationReferenceError(err, path)
	}

	additional, err := schema.additionalProperties(m.semantic)
	if err != nil {
		return false, materializationReferenceError(err, path)
	}

	if len(additional) == 0 {
		additional, err = schema.unevaluatedProperties(m.semantic)
	}

	if err != nil {
		return false, materializationReferenceError(err, path)
	}

	dynamic, hasDynamic := combinedEffectiveSchema(additional)
	if hasDynamic &&
		!isFalseEffectiveSchema(dynamic) &&
		(len(keys) < minimum || (m.mode == ExampleModeAll && len(keys) == 0)) {
		for _, name := range dynamicPropertyNames(propertyNames, m.semantic, path) {
			if !slices.Contains(keys, name) &&
				validatePropertyName(m.semantic, propertyNames, name, path) == nil {
				if m.budget.repairs <= 0 {
					break
				}

				m.budget.repairs--
				found, err := m.searchObjectShape(ctx, append(slices.Clone(keys), name), path, active, accept)
				if found || isMaterializationReferenceError(err) {
					return found, err
				}
			}
		}
	}
	if len(keys) < minimum {
		return false, generationSearchError(schema, path)
	}

	return m.searchObjectValues(schema, keys, dynamic, path, accept)
}

// searchObjectValues materializes one preferred value per selected property.
// Property shapes are repaired here; deep value backtracking remains unsupported.
func (m *exampleMaterializer) searchObjectValues(
	schema effectiveSchema,
	keys []string,
	dynamic effectiveSchema,
	path schemaPath,
	accept func(any) bool,
) (bool, error) {
	propertyNames, err := schema.propertyNames(m.semantic)
	if err != nil {
		return false, materializationReferenceError(err, path)
	}

	result := make(map[string]any, len(keys))
	for _, name := range keys {
		if err := validatePropertyName(m.semantic, propertyNames, name, path); err != nil {
			if isMaterializationReferenceError(err) {
				return false, err
			}

			return false, generationSearchError(schema, path)
		}

		property, found, err := schema.property(m.semantic, name)
		if err != nil {
			return false, materializationReferenceError(err, path.appendProperty(name))
		}

		if !found {
			property = dynamic
		}

		value, err := m.materializeEffective(property, path.appendProperty(name))
		if err != nil {
			return false, err
		}

		result[name] = value
	}

	return accept(result), nil
}

// dependencyKeywords preserves the dialect's dependency vocabulary.
func (view *schemaSemanticView) dependencyKeywords() []string {
	if view.dialect == schemaDialect201909 || view.dialect == schemaDialect202012 {
		return []string{"dependentRequired", "dependentSchemas"}
	}

	return []string{"dependencies"}
}

// annotationSensitiveSchema marks uncertain branch counts without sharing annotations
// across subschema scopes. The root validator resolves their actual applicability.
func annotationSensitiveSchema(schema effectiveSchema) bool {
	for _, term := range schema.terms {
		if annotationSensitiveValue(term.Object) {
			return true
		}
	}

	for _, group := range schema.compositionGroups {
		if slices.ContainsFunc(group.branches, annotationSensitiveSchema) {
			return true
		}
	}

	return false
}

// annotationSensitiveValue conservatively detects annotation-dependent assertions.
func annotationSensitiveValue(value any) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, nested := range value {
			if key == "unevaluatedProperties" || key == "unevaluatedItems" {
				return true
			}
			if key != "$defs" && key != "definitions" && annotationSensitiveValue(nested) {
				return true
			}
		}

	case []any:
		if slices.ContainsFunc(value, annotationSensitiveValue) {
			return true
		}
	}

	return false
}
