// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// MaterializationErrorCode identifies a stable example-generation failure.
type MaterializationErrorCode string

const (
	// MaterializationCodeUnsatisfiable means no valid instance was found.
	MaterializationCodeUnsatisfiable MaterializationErrorCode = "unsatisfiable"
	// MaterializationCodeUnresolvedReference means a local reference target is missing.
	MaterializationCodeUnresolvedReference MaterializationErrorCode = "unresolved_reference"
	// MaterializationCodeExternalReference means a network or external resource is required.
	MaterializationCodeExternalReference MaterializationErrorCode = "unsupported_external_reference"
	// MaterializationCodeReferenceRecursion means recursion has no finite generated value.
	MaterializationCodeReferenceRecursion MaterializationErrorCode = "reference_recursion"
	// MaterializationCodeUnsupportedRequired means required semantics cannot be materialized.
	MaterializationCodeUnsupportedRequired MaterializationErrorCode = "unsupported_required_semantics"
	// MaterializationCodeGeneratedValueInvalid means synthesis produced an invalid value.
	MaterializationCodeGeneratedValueInvalid MaterializationErrorCode = "generated_value_invalid"
	// MaterializationCodeUnsupported means no safe synthesis strategy is implemented.
	MaterializationCodeUnsupported MaterializationErrorCode = "unsupported_materialization"
)

// MaterializationErrorCategory groups a stable example-generation failure.
type MaterializationErrorCategory string

const (
	// MaterializationCategoryUnsatisfiableSchema identifies an empty valid-instance set.
	MaterializationCategoryUnsatisfiableSchema MaterializationErrorCategory = "unsatisfiable_schema"
	// MaterializationCategoryUnresolvedReference identifies a missing local target.
	MaterializationCategoryUnresolvedReference MaterializationErrorCategory = "unresolved_reference"
	// MaterializationCategoryUnsupportedExternalReference identifies a non-local target.
	MaterializationCategoryUnsupportedExternalReference MaterializationErrorCategory = "unsupported_external_reference"
	// MaterializationCategoryReferenceCycle identifies recursion without a finite solution.
	MaterializationCategoryReferenceCycle MaterializationErrorCategory = "reference_cycle"
	// MaterializationCategoryUnsupportedRequired identifies unsupported required semantics.
	MaterializationCategoryUnsupportedRequired MaterializationErrorCategory = "unsupported_required_semantics"
	// MaterializationCategoryGeneratedValueInvalid identifies invalid synthesis output.
	MaterializationCategoryGeneratedValueInvalid MaterializationErrorCategory = "generated_value_invalid"
	// MaterializationCategoryUnsupportedMaterialization identifies an unsupported strategy.
	MaterializationCategoryUnsupportedMaterialization MaterializationErrorCategory = "unsupported_materialization"
)

// MaterializationError describes a deterministic example-generation failure.
// Path is a root-relative JSON Schema location such as #/properties/name/type.
type MaterializationError struct {
	Cause    error
	Code     MaterializationErrorCode
	Category MaterializationErrorCategory
	Path     string
}

// Error implements error.
func (err *MaterializationError) Error() string {
	if err == nil {
		return "<nil>"
	}

	message := "materialize example: " + string(err.Category)
	if err.Path != "" {
		message += " at " + err.Path
	}
	if err.Cause != nil {
		message += ": " + err.Cause.Error()
	}

	return message
}

// Unwrap returns the underlying reference or validation failure.
func (err *MaterializationError) Unwrap() error {
	if err == nil {
		return nil
	}

	return err.Cause
}

// Is maps materialization codes to public sentinel errors.
func (err *MaterializationError) Is(target error) bool {
	if err == nil {
		return false
	}

	switch target {
	case ErrMaterializationUnsatisfiable:
		return err.Code == MaterializationCodeUnsatisfiable
	case ErrUnsupportedMaterialization:
		return err.Code == MaterializationCodeUnsupported
	case ErrUnsupportedRequiredSemantics:
		return err.Code == MaterializationCodeUnsupportedRequired
	case ErrGeneratedValueInvalid:
		return err.Code == MaterializationCodeGeneratedValueInvalid
	default:
		return false
	}
}

// exampleMaterializer selects and validates one deterministic example value.
type exampleMaterializer struct {
	semantic   *schemaSemanticView
	activeRefs map[string]int
	mode       ExampleMode
}

// newExampleMaterializer creates a materializer for one parsed document.
func newExampleMaterializer(doc schemaDocument, mode ExampleMode) *exampleMaterializer {
	return &exampleMaterializer{
		semantic:   newSchemaSemanticViewPointer(doc),
		mode:       mode,
		activeRefs: make(map[string]int),
	}
}

// materialize builds one valid value or returns a structured failure.
func (materializer *exampleMaterializer) materialize(node schemaValue) (any, error) {
	return materializer.materializeAt(node, schemaPath{})
}

// materializeAt builds a value at one instance/schema location.
func (materializer *exampleMaterializer) materializeAt(node schemaValue, path schemaPath) (any, error) {
	effective, err := materializer.semantic.effective(node)
	if err != nil {
		return nil, materializationReferenceError(err, path)
	}

	return materializer.materializeEffective(effective, path)
}

// materializeEffective builds a value while retaining expanded reference keys.
func (materializer *exampleMaterializer) materializeEffective(effective effectiveSchema, path schemaPath) (any, error) {
	for _, ref := range effective.referenceKeys {
		if materializer.activeRefs[ref] > 0 {
			return nil, newMaterializationError(
				MaterializationCodeReferenceRecursion,
				MaterializationCategoryReferenceCycle,
				materializationKeywordPath(path, "$ref"),
				ErrSchemaReferenceCycle,
			)
		}

		materializer.activeRefs[ref]++
		defer func(ref string) {
			materializer.activeRefs[ref]--
			if materializer.activeRefs[ref] <= 0 {
				delete(materializer.activeRefs, ref)
			}
		}(ref)
	}

	for _, term := range effective.terms {
		if term.Bool != nil && !*term.Bool {
			return nil, newMaterializationError(
				MaterializationCodeUnsatisfiable,
				MaterializationCategoryUnsatisfiableSchema,
				pathPointer(path),
				ErrMaterializationUnsatisfiable,
			)
		}
	}

	for _, candidate := range materializer.explicitCandidates(effective) {
		if err := validateEffectiveInstance(
			materializer.semantic,
			effective,
			candidate.value,
			path,
		); err == nil {
			return cloneJSONValue(candidate.value), nil
		} else if isMaterializationReferenceError(err) {
			return nil, err
		}
	}

	value, err := materializer.synthesize(effective, path)
	if err != nil {
		return nil, err
	}

	if err := validateEffectiveInstance(materializer.semantic, effective, value, path); err != nil {
		if isMaterializationReferenceError(err) {
			return nil, err
		}

		if validation, ok := err.(*candidateValidationError); ok &&
			validation.keyword == "pattern" {
			return nil, newMaterializationError(
				MaterializationCodeUnsupported,
				MaterializationCategoryUnsupportedMaterialization,
				validation.path,
				err,
			)
		}

		if isDefinitelyUnsatisfiable(effective) {
			return nil, newMaterializationError(
				MaterializationCodeUnsatisfiable,
				MaterializationCategoryUnsatisfiableSchema,
				validationPath(err, path),
				err,
			)
		}

		return nil, newMaterializationError(
			MaterializationCodeGeneratedValueInvalid,
			MaterializationCategoryGeneratedValueInvalid,
			validationPath(err, path),
			err,
		)
	}

	return value, nil
}

// explicitCandidates returns candidates in the public selection order.
func (materializer *exampleMaterializer) explicitCandidates(schema effectiveSchema) []materializationCandidate {
	var candidates []materializationCandidate

	for _, value := range schema.consts() {
		candidates = append(candidates, materializationCandidate{value: value, source: "const"})
	}

	for _, raw := range schema.keywordValues("examples") {
		for _, value := range asSlice(raw) {
			candidates = append(candidates, materializationCandidate{value: value, source: "examples"})
		}
	}

	if value, ok := schema.annotation("default"); ok {
		candidates = append(candidates, materializationCandidate{value: value, source: "default"})
	}

	for _, raw := range schema.enums() {
		for _, value := range asSlice(raw) {
			candidates = append(candidates, materializationCandidate{value: value, source: "enum"})
		}
	}

	return candidates
}

// materializationCandidate records an explicit value and its source keyword.
type materializationCandidate struct {
	value  any
	source string
}

// synthesize creates a deterministic fallback after explicit candidates fail.
func (materializer *exampleMaterializer) synthesize(schema effectiveSchema, path schemaPath) (any, error) {
	if len(schema.compositionGroups) > 0 {
		return materializer.synthesizeComposition(schema, path)
	}

	properties, err := schema.properties(materializer.semantic)
	if err != nil {
		return nil, materializationReferenceError(err, path)
	}
	required := schema.required()
	typeName := effectiveSchemaType(schema)
	additionalProperties, err := schema.additionalProperties(materializer.semantic)
	if err != nil {
		return nil, materializationReferenceError(err, path)
	}
	propertyNames, err := schema.propertyNames(materializer.semantic)
	if err != nil {
		return nil, materializationReferenceError(err, path)
	}
	patterns, err := schema.patternProperties(materializer.semantic)
	if err != nil {
		return nil, materializationReferenceError(err, path)
	}

	if typeName == "object" || len(properties) > 0 || len(required) > 0 ||
		len(additionalProperties) > 0 || len(propertyNames) > 0 || len(patterns) > 0 {
		return materializer.synthesizeObject(
			schema,
			properties,
			required,
			additionalProperties,
			propertyNames,
			path,
		)
	}

	if typeName == "array" || materializer.hasArrayItems(schema) {
		return materializer.synthesizeArray(schema, path)
	}

	if typeName == "string" && hasSchemaKeyword(schema, "pattern") {
		if value, ok := knownPatternExample(schema); ok {
			return value, nil
		}
		return nil, newMaterializationError(
			MaterializationCodeUnsupported,
			MaterializationCategoryUnsupportedMaterialization,
			materializationKeywordPath(path, "pattern"),
			ErrUnsupportedMaterialization,
		)
	}

	if value, ok := scalarPlaceholder(typeName); ok {
		return value, nil
	}

	return nil, nil
}

// synthesizeComposition evaluates branch-generated candidates against the whole schema.
func (materializer *exampleMaterializer) synthesizeComposition(schema effectiveSchema, path schemaPath) (any, error) {
	var selected any
	for _, group := range schema.compositionGroups {
		valid := 0
		for _, branch := range group.branches {
			value, err := materializer.materializeEffective(branch, path)
			if err != nil {
				continue
			}
			if err := validateEffectiveInstance(materializer.semantic, schema, value, path); err != nil {
				continue
			}

			valid++
			selected = value
			if group.kind == schemaCompositionAnyOf {
				return value, nil
			}
		}

		if group.kind == schemaCompositionOneOf && valid == 1 {
			return selected, nil
		}
		if group.kind == schemaCompositionOneOf && valid > 1 {
			return nil, newMaterializationError(
				MaterializationCodeUnsatisfiable,
				MaterializationCategoryUnsatisfiableSchema,
				materializationKeywordPath(path, "oneOf"),
				ErrMaterializationUnsatisfiable,
			)
		}
	}

	return nil, newMaterializationError(
		MaterializationCodeUnsatisfiable,
		MaterializationCategoryUnsatisfiableSchema,
		pathPointer(path),
		ErrMaterializationUnsatisfiable,
	)
}

// synthesizeObject builds an object in two phases: declared fields first,
// then at most one dynamic field when all-mode needs a representative map entry.
// Declared fields remain authoritative;
// a field is removed only when another conjunctive object term makes its presence invalid.
func (materializer *exampleMaterializer) synthesizeObject(
	schema effectiveSchema,
	properties map[string]effectiveSchema,
	required []string,
	additionalProperties []effectiveSchema,
	propertyNames []effectiveSchema,
	path schemaPath,
) (map[string]any, error) {
	requiredSet := make(map[string]struct{}, len(required))
	for _, name := range required {
		// A required field cannot be skipped, including when its schema is false.
		requiredSet[name] = struct{}{}
		property, ok := properties[name]
		if !ok {
			return nil, newMaterializationError(
				MaterializationCodeUnsupportedRequired,
				MaterializationCategoryUnsupportedRequired,
				materializationKeywordPath(path, "required"),
				ErrUnsupportedRequiredSemantics,
			)
		}

		if isFalseEffectiveSchema(property) {
			return nil, newMaterializationError(
				MaterializationCodeUnsatisfiable,
				MaterializationCategoryUnsatisfiableSchema,
				pathPointer(path.appendProperty(name)),
				ErrMaterializationUnsatisfiable,
			)
		}
	}

	values := make(map[string]schemaValue, len(properties))
	for name, property := range properties {
		// False optional properties describe forbidden names, not values to emit.
		if isFalseEffectiveSchema(property) {
			continue
		}

		values[name] = property.asSchemaValue()
	}

	keys := propertyOrder(required, values)
	if materializer.mode == ExampleModeRequired {
		keys = requiredPropertyOrder(required, values)
	}

	minimum, hasMinimum, maximum, hasMaximum := objectBounds(schema)
	if hasMinimum && hasMaximum && minimum > maximum {
		return nil, newMaterializationError(
			MaterializationCodeUnsatisfiable,
			MaterializationCategoryUnsatisfiableSchema,
			materializationKeywordPath(path, "minProperties"),
			ErrMaterializationUnsatisfiable,
		)
	}

	if hasMaximum && len(required) > maximum {
		return nil, newMaterializationError(
			MaterializationCodeUnsatisfiable,
			MaterializationCategoryUnsatisfiableSchema,
			materializationKeywordPath(path, "maxProperties"),
			ErrMaterializationUnsatisfiable,
		)
	}

	if hasMaximum && len(keys) > maximum {
		// Keep every required field and fill the remaining budget in schema order.
		selected := make([]string, 0, maximum)
		for _, name := range keys {
			if _, ok := requiredSet[name]; ok {
				selected = append(selected, name)
			}
		}

		for _, name := range keys {
			if _, ok := requiredSet[name]; ok || len(selected) >= maximum {
				continue
			}

			selected = append(selected, name)
		}
		keys = selected
	}

	result := make(map[string]any, len(keys))
	for _, name := range keys {
		value, err := materializer.materializeEffective(properties[name], path.appendProperty(name))
		if err != nil {
			return nil, err
		}

		result[name] = value
	}
	result = pruneInvalidOptionalProperties(
		materializer.semantic,
		schema,
		result,
		keys,
		requiredSet,
		path,
	)

	additional, hasAdditional := combinedEffectiveSchema(additionalProperties)
	if materializer.mode == ExampleModeAll && hasAdditional &&
		!isFalseEffectiveSchema(additional) && (len(result) == 0 || len(result) < minimum) {
		// A dynamic key is illustrative only.
		// Candidate validation must satisfy propertyNames, patternProperties, and additionalProperties together.
		for _, name := range dynamicPropertyNames(propertyNames, materializer.semantic, path) {
			if _, declared := properties[name]; declared {
				continue
			}
			if _, exists := result[name]; exists {
				continue
			}

			if hasMaximum && len(result) >= maximum {
				break
			}

			if err := validatePropertyName(materializer.semantic, propertyNames, name, path); err != nil {
				if isMaterializationReferenceError(err) {
					return nil, err
				}
				continue
			}

			value, err := materializer.materializeEffective(additional, path.appendProperty(name))
			if err != nil {
				if isMaterializationReferenceError(err) {
					return nil, err
				}
				continue
			}

			result[name] = value
			if err := validateEffectiveInstance(materializer.semantic, schema, result, path); err != nil {
				delete(result, name)
				continue
			}
			break
		}
	}

	if hasMinimum && len(result) < minimum {
		// No safe key/value pair was available to meet the lower bound.
		return nil, newMaterializationError(
			MaterializationCodeUnsupported,
			MaterializationCategoryUnsupportedMaterialization,
			materializationKeywordPath(path, "minProperties"),
			ErrUnsupportedMaterialization,
		)
	}

	return result, nil
}

// pruneInvalidOptionalProperties repairs an initially invalid object
// by trying to remove optional fields from the end of the deterministic order.
// A minProperties failure is deferred because a dynamic field may still satisfy it;
// required fields are never removed.
func pruneInvalidOptionalProperties(
	view *schemaSemanticView,
	schema effectiveSchema,
	result map[string]any,
	keys []string,
	required map[string]struct{},
	path schemaPath,
) map[string]any {
	for {
		err := validateEffectiveInstance(view, schema, result, path)
		if err == nil || isMaterializationReferenceError(err) {
			return result
		}

		if candidateValidationKeyword(err) == "minProperties" {
			return result
		}

		removed := false
		for _, name := range slices.Backward(keys) {
			if _, isRequired := required[name]; isRequired {
				continue
			}
			if _, exists := result[name]; !exists {
				continue
			}

			candidate := make(map[string]any, len(result)-1)
			maps.Copy(candidate, result)
			delete(candidate, name)

			candidateErr := validateEffectiveInstance(view, schema, candidate, path)
			if isMaterializationReferenceError(candidateErr) {
				return result
			}

			if candidateErr == nil || candidateValidationKeyword(candidateErr) != "required" {
				result = candidate
				removed = true
				break
			}
		}

		if !removed {
			return result
		}
	}
}

// combinedEffectiveSchema joins all declared schemas for one object keyword.
func combinedEffectiveSchema(schemas []effectiveSchema) (effectiveSchema, bool) {
	if len(schemas) == 0 {
		return effectiveSchema{}, false
	}

	combined := schemas[0]
	for _, schema := range schemas[1:] {
		combined = combineEffectiveSchemas(combined, schema)
	}

	return combined, true
}

// dynamicPropertyNames returns annotation-driven names without synthesizing regex matches.
func dynamicPropertyNames(
	propertyNames []effectiveSchema,
	view *schemaSemanticView,
	path schemaPath,
) []string {
	nameSchema, hasNameSchema := combinedEffectiveSchema(propertyNames)
	if !hasNameSchema {
		return []string{"example"}
	}

	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, raw := range nameSchema.keywordValues("examples") {
		for _, value := range asSlice(raw) {
			name, ok := value.(string)
			if !ok || name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}

			if err := validateEffectiveInstance(view, nameSchema, name, path.appendProperty(name)); err != nil {
				if isMaterializationReferenceError(err) {
					continue
				}
				continue
			}

			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	if _, exists := seen["example"]; !exists {
		result = append(result, "example")
	}

	return result
}

// validatePropertyName applies all effective property-name schemas to one candidate.
func validatePropertyName(
	view *schemaSemanticView,
	propertyNames []effectiveSchema,
	name string,
	path schemaPath,
) error {
	nameSchema, ok := combinedEffectiveSchema(propertyNames)
	if !ok {
		return nil
	}

	return validateEffectiveInstance(view, nameSchema, name, path.appendProperty(name))
}

// objectBounds returns the most restrictive simultaneous object-size bounds.
func objectBounds(schema effectiveSchema) (minimum int, hasMinimum bool, maximum int, hasMaximum bool) {
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		if value, ok := integerKeyword(term.Object, "minProperties"); ok &&
			(!hasMinimum || value > minimum) {
			minimum, hasMinimum = value, true
		}
		if value, ok := integerKeyword(term.Object, "maxProperties"); ok &&
			(!hasMaximum || value < maximum) {
			maximum, hasMaximum = value, true
		}
	}

	return minimum, hasMinimum, maximum, hasMaximum
}

// scalarPlaceholder returns fallback value for a known scalar type.
func scalarPlaceholder(schemaType string) (any, bool) {
	value, ok := exampleScalarPlaceholders[schemaType]
	return value, ok
}

// synthesizeArray first materializes every mandatory tuple position,
// then satisfies contains and item-count requirements without exceeding maxItems.
// Each index receives the conjunction of all applicable item schemas,
// so heterogeneous tuple positions are not accidentally validated as homogeneous.
func (materializer *exampleMaterializer) synthesizeArray(schema effectiveSchema, path schemaPath) ([]any, error) {
	items, err := schema.arrayItems(materializer.semantic)
	if err != nil {
		return nil, materializationReferenceError(err, path)
	}

	contains, err := schema.arrayContains(materializer.semantic)
	if err != nil {
		return nil, materializationReferenceError(err, path)
	}

	unevaluated, err := schema.unevaluatedItems(materializer.semantic)
	if err != nil {
		return nil, materializationReferenceError(err, path)
	}

	minimum, maximum := arrayBounds(schema)
	if minimum > maximum || arrayContainsContradiction(contains) {
		return nil, newMaterializationError(
			MaterializationCodeUnsatisfiable,
			MaterializationCategoryUnsatisfiableSchema,
			materializationKeywordPath(path, "minItems"),
			ErrMaterializationUnsatisfiable,
		)
	}

	prefixLength := arrayPrefixLength(items)
	if prefixLength > maximum {
		return nil, newMaterializationError(
			MaterializationCodeUnsatisfiable,
			MaterializationCategoryUnsatisfiableSchema,
			materializationKeywordPath(path, "maxItems"),
			ErrMaterializationUnsatisfiable,
		)
	}
	result := make([]any, 0)
	for index := range prefixLength {
		// Tuple positions are mandatory even when no minimum length is declared.
		item, hasItem, err := arrayItemSchemaAt(items, index)
		if err != nil {
			return nil, err
		}

		value, err := materializer.materializeArrayValue(
			item,
			hasItem,
			result,
			schema,
			path,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}

	if len(contains) == 0 {
		// Item examples are useful for ordinary homogeneous arrays.
		// Contains arrays are built separately so their match-count bounds stay explicit.
		for _, example := range arrayItemExamples(items) {
			if len(result) >= maximum {
				break
			}

			item, hasItem, err := arrayItemSchemaAt(items, len(result))
			if err != nil {
				return nil, err
			}

			if hasItem && validateEffectiveInstance(materializer.semantic, item, example, path.appendArrayItem()) != nil {
				continue
			}
			if !hasUniqueArrayItem(schema, result, example) {
				continue
			}

			result = append(result, cloneJSONValue(example))
		}
	}

	if len(result) == 0 && prefixLength == 0 && len(contains) == 0 && maximum > 0 {
		item, hasItem, err := arrayItemSchemaAt(items, 0)
		if err != nil {
			return nil, err
		}

		if hasItem {
			value, err := materializer.materializeArrayValue(item, true, result, schema, path)
			if err != nil {
				return nil, err
			}

			result = append(result, value)
		}
	}

	for _, requirement := range contains {
		// Prefer an existing match; only append or replace a non-prefix item
		// when the requirement still lacks enough matching values.
		for containsCount(materializer.semantic, requirement.Schema, result, path) < requirement.Minimum {
			if len(result) >= maximum {
				if index := replaceableArrayItemIndex(result, prefixLength, materializer.semantic, requirement.Schema, path); index >= 0 {
					item, hasItem, err := arrayItemSchemaAt(items, index)
					if err != nil {
						return nil, err
					}

					candidateSchema := combineOptionalArraySchemas(item, hasItem, requirement.Schema)
					value, err := materializer.materializeArrayValue(candidateSchema, true, result[:index], schema, path)
					if err != nil {
						return nil, err
					}

					result[index] = value
					continue
				}

				return nil, newMaterializationError(
					MaterializationCodeUnsupported,
					MaterializationCategoryUnsupportedMaterialization,
					materializationKeywordPath(path, "contains"),
					ErrUnsupportedMaterialization,
				)
			}

			item, hasItem, err := arrayItemSchemaAt(items, len(result))
			if err != nil {
				return nil, err
			}

			candidateSchema := combineOptionalArraySchemas(item, hasItem, requirement.Schema)
			value, err := materializer.materializeArrayValue(candidateSchema, true, result, schema, path)
			if err != nil {
				return nil, err
			}

			if err := validateEffectiveInstance(materializer.semantic, requirement.Schema, value, path.appendArrayItem()); err != nil {
				return nil, newMaterializationError(
					MaterializationCodeUnsupported,
					MaterializationCategoryUnsupportedMaterialization,
					materializationKeywordPath(path, "contains"),
					ErrUnsupportedMaterialization,
				)
			}
			result = append(result, value)
		}
	}

	for len(result) < minimum {
		// Trailing positions use items/additionalItems first.
		// In modern drafts, unevaluatedItems is the fallback for positions outside prefixItems.
		item, hasItem, err := arrayItemSchemaAt(items, len(result))
		if err != nil {
			return nil, err
		}

		if !hasItem {
			item, hasItem, err = combinedOptionalArraySchemas(unevaluated)
			if err != nil {
				return nil, err
			}
		}

		value, err := materializer.materializeArrayValue(item, hasItem, result, schema, path)
		if err != nil {
			return nil, err
		}

		result = append(result, value)
	}

	if len(result) > maximum {
		// Only non-tuple values may be trimmed; tuple prefix positions are fixed.
		if prefixLength > maximum {
			return nil, newMaterializationError(
				MaterializationCodeUnsatisfiable,
				MaterializationCategoryUnsatisfiableSchema,
				materializationKeywordPath(path, "maxItems"),
				ErrMaterializationUnsatisfiable,
			)
		}
		result = result[:maximum]
	}

	if err := validateEffectiveInstance(materializer.semantic, schema, result, path); err != nil {
		if isMaterializationReferenceError(err) {
			return nil, err
		}

		if isDefinitelyUnsatisfiable(schema) {
			return nil, newMaterializationError(
				MaterializationCodeUnsatisfiable,
				MaterializationCategoryUnsatisfiableSchema,
				validationPath(err, path),
				ErrMaterializationUnsatisfiable,
			)
		}

		return nil, newMaterializationError(
			MaterializationCodeUnsupported,
			MaterializationCategoryUnsupportedMaterialization,
			validationPath(err, path),
			ErrUnsupportedMaterialization,
		)
	}

	return result, nil
}

// arrayPrefixLength returns the longest tuple prefix across simultaneous terms.
// A longer prefix in any term makes those indexes mandatory for the combined schema.
func arrayPrefixLength(items []effectiveArrayItems) int {
	length := 0
	for _, current := range items {
		if current.PrefixItems != nil && len(*current.PrefixItems) > length {
			length = len(*current.PrefixItems)
		}
	}

	return length
}

// arrayItemSchemaAt combines every item constraint applying at one index.
// A missing schema from one term means that term imposes no item restriction;
// only schemas explicitly selected by a term participate in the conjunction.
func arrayItemSchemaAt(items []effectiveArrayItems, index int) (effectiveSchema, bool, error) {
	var selected []effectiveSchema
	for _, current := range items {
		var item *effectiveSchema
		switch {
		case current.PrefixItems != nil && index < len(*current.PrefixItems):
			item = &(*current.PrefixItems)[index]
		case current.Items != nil:
			item = current.Items
		case current.AdditionalItems != nil:
			item = current.AdditionalItems
		}

		if item != nil {
			selected = append(selected, *item)
		}
	}

	return combinedOptionalArraySchemas(selected)
}

// combinedOptionalArraySchemas combines selected item schemas
// while preserving the distinction between "no restriction" and an explicit true/false schema.
func combinedOptionalArraySchemas(schemas []effectiveSchema) (effectiveSchema, bool, error) {
	if len(schemas) == 0 {
		return effectiveSchema{}, false, nil
	}

	combined := schemas[0]
	for _, schema := range schemas[1:] {
		combined = combineEffectiveSchemas(combined, schema)
	}

	return combined, true, nil
}

// combineOptionalArraySchemas creates the schema required at an index
// that must satisfy both its ordinary item rule and one contains rule.
func combineOptionalArraySchemas(item effectiveSchema, hasItem bool, contains effectiveSchema) effectiveSchema {
	if !hasItem {
		return contains
	}

	return combineEffectiveSchemas(item, contains)
}

// arrayItemExamples returns only homogeneous-item examples.
// Tuple annotations are consumed at their own index by materializeEffective instead.
func arrayItemExamples(items []effectiveArrayItems) []any {
	var result []any
	for _, current := range items {
		if current.Items == nil {
			continue
		}
		for _, raw := range current.Items.keywordValues("examples") {
			result = append(result, asSlice(raw)...)
		}
	}

	return result
}

// hasUniqueArrayItem reports whether a candidate may be appended under uniqueItems.
func hasUniqueArrayItem(schema effectiveSchema, values []any, candidate any) bool {
	for _, term := range schema.terms {
		if term.Object != nil {
			if unique, ok := term.Object["uniqueItems"].(bool); ok && unique && containsJSONValue(values, candidate) {
				return false
			}
		}
	}

	return true
}

// materializeArrayValue builds one item and, if uniqueItems rejects the normal value,
// tries a bounded scalar alternative that is revalidated by its item schema.
func (materializer *exampleMaterializer) materializeArrayValue(
	item effectiveSchema,
	hasItem bool,
	values []any,
	arraySchema effectiveSchema,
	path schemaPath,
) (any, error) {
	var value any
	var err error
	if hasItem {
		value, err = materializer.materializeEffective(item, path.appendArrayItem())
	} else {
		value = nil
	}
	if err != nil {
		return nil, err
	}
	if hasUniqueArrayItem(arraySchema, values, value) {
		return value, nil
	}

	for attempt := 1; attempt <= len(values)+16; attempt++ {
		candidate, ok := uniqueArrayScalarCandidate(effectiveSchemaType(item), attempt)
		if !ok || containsJSONValue(values, candidate) {
			continue
		}
		if hasItem && validateEffectiveInstance(materializer.semantic, item, candidate, path.appendArrayItem()) != nil {
			continue
		}
		return candidate, nil
	}

	return nil, newMaterializationError(
		MaterializationCodeUnsupported,
		MaterializationCategoryUnsupportedMaterialization,
		materializationKeywordPath(path, "uniqueItems"),
		ErrUnsupportedMaterialization,
	)
}

// uniqueArrayScalarCandidate returns a bounded alternative only for scalar types
// whose values can be varied without inventing object or array structure.
func uniqueArrayScalarCandidate(schemaType string, value int) (any, bool) {
	switch schemaType {
	case "string":
		return fmt.Sprintf("<string>-%d", value), true
	case "number", "integer":
		return value, true
	case "boolean":
		return value%2 == 0, true
	case "":
		return value, true
	default:
		return nil, false
	}
}

// containsCount counts items that independently validate against one contains schema.
func containsCount(view *schemaSemanticView, schema effectiveSchema, values []any, path schemaPath) int {
	count := 0
	for _, value := range values {
		if validateEffectiveInstance(view, schema, value, path.appendArrayItem()) == nil {
			count++
		}
	}

	return count
}

// replaceableArrayItemIndex finds a non-prefix,
// non-matching item that can be replaced while preserving the mandatory tuple prefix.
func replaceableArrayItemIndex(
	values []any,
	prefixLength int,
	view *schemaSemanticView,
	schema effectiveSchema,
	path schemaPath,
) int {
	for index := len(values) - 1; index >= prefixLength; index-- {
		if validateEffectiveInstance(view, schema, values[index], path.appendArrayItem()) != nil {
			return index
		}
	}

	return -1
}

// arrayContainsContradiction detects an impossible contains interval.
func arrayContainsContradiction(requirements []effectiveArrayContains) bool {
	for _, requirement := range requirements {
		if requirement.HasMaximum && requirement.Minimum > requirement.Maximum {
			return true
		}
	}

	return false
}

// hasArrayItems reports whether at least one term declares array semantics.
func (materializer *exampleMaterializer) hasArrayItems(schema effectiveSchema) bool {
	items, err := schema.arrayItems(materializer.semantic)
	return err == nil && len(items) > 0
}

// arrayBounds returns the narrowest declared item-count interval.
func arrayBounds(schema effectiveSchema) (int, int) {
	minimum, maximum := 0, int(^uint(0)>>1)
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		if value, ok := integerKeyword(term.Object, "minItems"); ok && value > minimum {
			minimum = value
		}
		if value, ok := integerKeyword(term.Object, "maxItems"); ok && value < maximum {
			maximum = value
		}
	}

	return minimum, maximum
}

// knownPatternExample returns a safe value for a small built-in-style pattern.
func knownPatternExample(schema effectiveSchema) (string, bool) {
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		if asString(term.Object["pattern"]) == "^[0-2][0-9]:[0-5][0-9]$" {
			return "00:00", true
		}
	}

	return "", false
}

// validateEffectiveInstance validates the supported effective constraints.
func validateEffectiveInstance(
	view *schemaSemanticView,
	schema effectiveSchema,
	value any,
	path schemaPath,
) error {
	for _, term := range schema.terms {
		if term.Bool != nil {
			if !*term.Bool {
				return newCandidateValidationError(pathPointer(path), "schema", "false schema")
			}

			continue
		}

		if err := validateSchemaTerm(view, term.Object, value, path); err != nil {
			return err
		}
	}

	if err := validateEffectiveUnevaluatedItems(view, schema, value, path); err != nil {
		return err
	}

	for _, group := range schema.compositionGroups {
		valid := 0
		for _, branch := range group.branches {
			if err := validateEffectiveInstance(view, branch, value, path); err == nil {
				valid++
			}
		}

		switch {
		case group.kind == schemaCompositionAnyOf && valid == 0:
			return newCandidateValidationError(
				materializationKeywordPath(path, "anyOf"),
				"anyOf",
				"no branch applies")
		case group.kind == schemaCompositionOneOf && valid != 1:
			return newCandidateValidationError(
				materializationKeywordPath(path, "oneOf"),
				"oneOf",
				"branch count is not one")
		}
	}

	return nil
}

// validateSchemaTerm validates constraints from one simultaneous schema term.
func validateSchemaTerm(view *schemaSemanticView, object map[string]any, value any, path schemaPath) error {
	if object == nil {
		return nil
	}

	if rawType, ok := object["type"]; ok && !matchesSchemaType(value, rawType) {
		return newCandidateValidationError(
			materializationKeywordPath(path, "type"),
			"type",
			"value has the wrong type")
	}
	if expected, ok := object["const"]; ok && !equalJSONValue(value, expected) {
		return newCandidateValidationError(
			materializationKeywordPath(path, "const"),
			"const",
			"value differs from const")
	}
	if values := asSlice(object["enum"]); len(values) > 0 && !containsJSONValue(values, value) {
		return newCandidateValidationError(
			materializationKeywordPath(path, "enum"),
			"enum",
			"value is not an enum member")
	}

	if err := validateStringTerm(object, value, path); err != nil {
		return err
	}
	if err := validateNumberTerm(object, value, path); err != nil {
		return err
	}
	if err := validateObjectTerm(view, object, value, path); err != nil {
		return err
	}
	if err := validateArrayTerm(view, object, value, path); err != nil {
		return err
	}

	return nil
}

// validateStringTerm validates basic Unicode string constraints.
func validateStringTerm(object map[string]any, value any, path schemaPath) error {
	text, ok := value.(string)
	if !ok {
		return nil
	}

	length := utf8.RuneCountInString(text)
	if minimum, ok := integerKeyword(object, "minLength"); ok && length < minimum {
		return newCandidateValidationError(
			materializationKeywordPath(path, "minLength"),
			"minLength",
			"string is too short")
	}

	if maximum, ok := integerKeyword(object, "maxLength"); ok && length > maximum {
		return newCandidateValidationError(
			materializationKeywordPath(path, "maxLength"),
			"maxLength",
			"string is too long")
	}

	if pattern := asString(object["pattern"]); pattern != "" {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return newCandidateValidationError(
				materializationKeywordPath(path, "pattern"),
				"pattern",
				err.Error())
		}

		if !compiled.MatchString(text) {
			return newCandidateValidationError(
				materializationKeywordPath(path, "pattern"),
				"pattern",
				"string does not match pattern")
		}
	}

	return nil
}

// validateNumberTerm validates basic numeric interval constraints.
func validateNumberTerm(object map[string]any, value any, path schemaPath) error {
	number, ok := asNumber(value)
	if !ok {
		return nil
	}

	if minimum, ok := asNumber(object["minimum"]); ok {
		exclusive := false
		if flag, isBool := object["exclusiveMinimum"].(bool); isBool {
			exclusive = flag
		} else if bound, isNumber := asNumber(object["exclusiveMinimum"]); isNumber {
			minimum = bound
			exclusive = true
		}

		if (exclusive && number <= minimum) || (!exclusive && number < minimum) {
			return newCandidateValidationError(
				materializationKeywordPath(path, "minimum"),
				"minimum",
				"number is below minimum")
		}
	} else if minimum, ok := asNumber(object["exclusiveMinimum"]); ok && number <= minimum {
		return newCandidateValidationError(
			materializationKeywordPath(path, "exclusiveMinimum"),
			"exclusiveMinimum",
			"number is below exclusive minimum")
	}

	if maximum, ok := asNumber(object["maximum"]); ok {
		exclusive := false
		if flag, isBool := object["exclusiveMaximum"].(bool); isBool {
			exclusive = flag
		} else if bound, isNumber := asNumber(object["exclusiveMaximum"]); isNumber {
			maximum = bound
			exclusive = true
		}

		if (exclusive && number >= maximum) || (!exclusive && number > maximum) {
			return newCandidateValidationError(
				materializationKeywordPath(path, "maximum"),
				"maximum",
				"number is above maximum")
		}
	} else if maximum, ok := asNumber(object["exclusiveMaximum"]); ok && number >= maximum {
		return newCandidateValidationError(
			materializationKeywordPath(path, "exclusiveMaximum"),
			"exclusiveMaximum",
			"number is above exclusive maximum")
	}

	if multiple, ok := asNumber(object["multipleOf"]); ok && multiple > 0 {
		quotient := number / multiple
		if math.Abs(quotient-math.Round(quotient)) > 1e-9 {
			return newCandidateValidationError(
				materializationKeywordPath(path, "multipleOf"),
				"multipleOf",
				"number is not a multiple")
		}
	}

	return nil
}

// validateObjectTerm validates required, property, and additional-property rules.
func validateObjectTerm(view *schemaSemanticView, object map[string]any, value any, path schemaPath) error {
	properties, isObject := value.(map[string]any)
	if !isObject {
		return nil
	}

	if minimum, ok := integerKeyword(object, "minProperties"); ok && len(properties) < minimum {
		return newCandidateValidationError(
			materializationKeywordPath(path, "minProperties"),
			"minProperties",
			"object has too few properties")
	}

	if maximum, ok := integerKeyword(object, "maxProperties"); ok && len(properties) > maximum {
		return newCandidateValidationError(
			materializationKeywordPath(path, "maxProperties"),
			"maxProperties",
			"object has too many properties")
	}

	declared := mapSchemaValues(object["properties"])
	for _, name := range asStringSlice(object["required"]) {
		if _, ok := properties[name]; !ok {
			return newCandidateValidationError(
				materializationKeywordPath(path, "required"),
				"required",
				"property is missing")
		}
	}

	for name, rawSchema := range declared {
		instance, ok := properties[name]
		if !ok {
			continue
		}

		nested, err := view.effective(rawSchema)
		if err != nil {
			return materializationReferenceError(err, path.appendProperty(name))
		}

		if err := validateEffectiveInstance(view, nested, instance, path.appendProperty(name)); err != nil {
			return err
		}
	}

	patterns := objectPatterns(object)
	for name, instance := range properties {
		matched := false
		if _, ok := declared[name]; ok {
			matched = true
		}

		for pattern, compiled := range patterns {
			if !compiled.MatchString(name) {
				continue
			}

			matched = true
			raw, ok := object["patternProperties"].(map[string]any)[pattern]
			if !ok {
				continue
			}

			schema, valid := toSchemaValue(raw)
			if !valid {
				continue
			}

			nested, err := view.effective(schema)
			if err != nil {
				return materializationReferenceError(err, path.appendProperty(name))
			}

			if err := validateEffectiveInstance(view, nested, instance, path.appendProperty(name)); err != nil {
				return err
			}
		}

		if matched {
			continue
		}

		additional, exists := object["additionalProperties"]
		if !exists {
			continue
		}

		if flag, ok := additional.(bool); ok {
			if !flag {
				return newCandidateValidationError(
					materializationKeywordPath(path, "additionalProperties"),
					"additionalProperties",
					"property is forbidden")
			}
			continue
		}

		schema, ok := toSchemaValue(additional)
		if !ok {
			continue
		}

		nested, err := view.effective(schema)
		if err != nil {
			return materializationReferenceError(err, path.appendProperty(name))
		}

		if err := validateEffectiveInstance(view, nested, instance, path.appendProperty(name)); err != nil {
			return err
		}
	}

	if rawNames, ok := object["propertyNames"]; ok {
		nameSchema, valid := toSchemaValue(rawNames)
		if valid {
			effective, err := view.effective(nameSchema)
			if err != nil {
				return materializationReferenceError(err, path.appendProperty("propertyNames"))
			}
			for name := range properties {
				if err := validateEffectiveInstance(view, effective, name, path.appendProperty(name)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateArrayTerm validates one term's tuple/item rules and its contains count.
// Effective unevaluatedItems validation runs after all terms are known.
func validateArrayTerm(view *schemaSemanticView, object map[string]any, value any, path schemaPath) error {
	items, ok := value.([]any)
	if !ok {
		return nil
	}

	minimum, hasMinimum := integerKeyword(object, "minItems")
	maximum, hasMaximum := integerKeyword(object, "maxItems")
	if hasMinimum && len(items) < minimum {
		return newCandidateValidationError(
			materializationKeywordPath(path, "minItems"),
			"minItems",
			"array is too short")
	}

	if hasMaximum && len(items) > maximum {
		return newCandidateValidationError(
			materializationKeywordPath(path, "maxItems"),
			"maxItems",
			"array is too long")
	}

	if flag, ok := object["uniqueItems"].(bool); ok && flag {
		for index, item := range items {
			if containsJSONValue(items[:index], item) {
				return newCandidateValidationError(
					materializationKeywordPath(path, "uniqueItems"),
					"uniqueItems",
					"array contains duplicates")
			}
		}
	}

	effective := effectiveSchema{terms: []schemaValue{{Object: object}}}
	term, err := effective.arrayItems(view)
	if err != nil {
		return materializationReferenceError(err, path)
	}

	for index, item := range items {
		itemSchema, hasItem, err := arrayItemSchemaAt(term, index)
		if err != nil {
			return err
		}

		if hasItem {
			if err := validateEffectiveInstance(view, itemSchema, item, path.appendArrayItem()); err != nil {
				return err
			}
		}
	}

	contains, err := effective.arrayContains(view)
	if err != nil {
		return materializationReferenceError(err, path)
	}
	for _, requirement := range contains {
		count := containsCount(view, requirement.Schema, items, path)
		if count < requirement.Minimum {
			keyword := "contains"
			if requirement.Minimum != 1 {
				keyword = "minContains"
			}
			return newCandidateValidationError(
				materializationKeywordPath(path, keyword),
				keyword,
				"array does not contain enough matching items")
		}

		if requirement.HasMaximum && count > requirement.Maximum {
			return newCandidateValidationError(
				materializationKeywordPath(path, "maxContains"),
				"maxContains",
				"array contains too many matching items")
		}
	}

	return nil
}

// validateEffectiveUnevaluatedItems applies modern unevaluatedItems
// only after all conjunctive terms are visible,
// so an item evaluated by another term is not incorrectly treated as unevaluated.
func validateEffectiveUnevaluatedItems(
	view *schemaSemanticView,
	schema effectiveSchema,
	value any,
	path schemaPath,
) error {
	items, ok := value.([]any)
	if !ok || view.legacyArraySemantics() {
		return nil
	}

	unevaluated, err := schema.unevaluatedItems(view)
	if err != nil {
		return materializationReferenceError(err, path)
	}
	unevaluatedSchema, hasUnevaluated, _ := combinedOptionalArraySchemas(unevaluated)
	if !hasUnevaluated {
		return nil
	}

	ordinary, err := schema.arrayItems(view)
	if err != nil {
		return materializationReferenceError(err, path)
	}
	contains, err := schema.arrayContains(view)
	if err != nil {
		return materializationReferenceError(err, path)
	}

	for index, item := range items {
		// Prefix/items schemas and successful contains matches both mark
		// an item as evaluated for the purpose of unevaluatedItems.
		if arrayItemEvaluated(ordinary, index) || arrayItemMatchesContains(view, contains, item, path) {
			continue
		}

		if err := validateEffectiveInstance(view, unevaluatedSchema, item, path.appendArrayItem()); err != nil {
			return err
		}
	}

	return nil
}

// arrayItemEvaluated reports whether ordinary item keywords cover one index.
func arrayItemEvaluated(items []effectiveArrayItems, index int) bool {
	for _, current := range items {
		if current.PrefixItems != nil && index < len(*current.PrefixItems) {
			return true
		}
		if current.Items != nil || current.AdditionalItems != nil {
			return true
		}
	}

	return false
}

// arrayItemMatchesContains reports whether one item is evaluated by any contains clause.
func arrayItemMatchesContains(
	view *schemaSemanticView,
	contains []effectiveArrayContains,
	value any,
	path schemaPath,
) bool {
	for _, requirement := range contains {
		if validateEffectiveInstance(view, requirement.Schema, value, path.appendArrayItem()) == nil {
			return true
		}
	}

	return false
}

// objectPatterns compiles patternProperties without applying arbitrary synthesis.
func objectPatterns(object map[string]any) map[string]*regexp.Regexp {
	patterns := make(map[string]*regexp.Regexp)
	raw, ok := object["patternProperties"].(map[string]any)
	if !ok {
		return patterns
	}

	for pattern := range raw {
		if compiled, err := regexp.Compile(pattern); err == nil {
			patterns[pattern] = compiled
		}
	}

	return patterns
}

// effectiveSchemaType returns the first declared non-null type for synthesis.
func effectiveSchemaType(schema effectiveSchema) string {
	for _, value := range schema.types() {
		for _, name := range schemaTypeNames(value) {
			if name != "null" {
				return name
			}
		}
	}

	for _, value := range schema.types() {
		if names := schemaTypeNames(value); len(names) > 0 {
			return names[0]
		}
	}

	return ""
}

// schemaTypeNames normalizes one JSON Schema type declaration.
func schemaTypeNames(value any) []string {
	if name := asString(value); name != "" {
		return []string{strings.ToLower(name)}
	}

	var result []string
	for _, item := range asSlice(value) {
		if name := asString(item); name != "" {
			result = append(result, strings.ToLower(name))
		}
	}

	return result
}

// matchesSchemaType reports whether a JSON-like value satisfies a type declaration.
func matchesSchemaType(value, declaration any) bool {
	for _, name := range schemaTypeNames(declaration) {
		if matchesJSONType(value, name) {
			return true
		}
	}
	return false
}

// matchesJSONType reports whether a value has one JSON Schema primitive type.
func matchesJSONType(value any, name string) bool {
	switch name {
	case "null":
		return value == nil

	case "boolean":
		_, ok := value.(bool)
		return ok

	case "object":
		_, ok := value.(map[string]any)
		return ok

	case "array":
		_, ok := value.([]any)
		return ok

	case "string":
		_, ok := value.(string)
		return ok

	case "number":
		_, ok := asNumber(value)
		return ok

	case "integer":
		number, ok := asNumber(value)
		return ok && math.Trunc(number) == number

	default:
		return false
	}
}

// equalJSONValue compares decoded JSON values while normalizing numeric types.
func equalJSONValue(left, right any) bool {
	if leftNumber, leftOK := asNumber(left); leftOK {
		rightNumber, rightOK := asNumber(right)
		return rightOK && leftNumber == rightNumber
	}

	switch leftTyped := left.(type) {
	case map[string]any:
		rightTyped, ok := right.(map[string]any)
		if !ok || len(leftTyped) != len(rightTyped) {
			return false
		}

		for key, value := range leftTyped {
			if !equalJSONValue(value, rightTyped[key]) {
				return false
			}
		}

		return true

	case []any:
		rightTyped, ok := right.([]any)
		if !ok || len(leftTyped) != len(rightTyped) {
			return false
		}

		for index, value := range leftTyped {
			if !equalJSONValue(value, rightTyped[index]) {
				return false
			}
		}

		return true

	default:
		return left == right
	}
}

// containsJSONValue reports whether one JSON-like slice contains a value.
func containsJSONValue(values []any, target any) bool {
	for _, value := range values {
		if equalJSONValue(value, target) {
			return true
		}
	}

	return false
}

// integerKeyword reads a non-negative integer keyword.
func integerKeyword(object map[string]any, keyword string) (int, bool) {
	value, ok := asNumber(object[keyword])
	if !ok || value < 0 || math.Trunc(value) != value {
		return 0, false
	}

	return int(value), true
}

// hasSchemaKeyword reports whether any effective term declares a keyword.
func hasSchemaKeyword(schema effectiveSchema, keyword string) bool {
	for _, term := range schema.terms {
		if term.Object != nil {
			if _, ok := term.Object[keyword]; ok {
				return true
			}
		}
	}

	return false
}

// isFalseEffectiveSchema reports whether one conjunctive schema term forbids every value.
func isFalseEffectiveSchema(schema effectiveSchema) bool {
	for _, term := range schema.terms {
		if term.Bool != nil && !*term.Bool {
			return true
		}
	}

	return false
}

// candidateValidationKeyword extracts the failed validation keyword when available.
func candidateValidationKeyword(err error) string {
	var validation *candidateValidationError
	if errors.As(err, &validation) {
		return validation.keyword
	}

	return ""
}

// candidateValidationError describes why one candidate was rejected.
type candidateValidationError struct {
	path    string
	keyword string
	reason  string
}

// Error implements error.
func (err *candidateValidationError) Error() string {
	return fmt.Sprintf("candidate violates %s: %s", err.keyword, err.reason)
}

// newCandidateValidationError creates a deterministic candidate failure.
func newCandidateValidationError(path, keyword, reason string) *candidateValidationError {
	return &candidateValidationError{path: path, keyword: keyword, reason: reason}
}

// newMaterializationError creates a structured generation failure.
func newMaterializationError(
	code MaterializationErrorCode,
	category MaterializationErrorCategory,
	path string,
	cause error,
) *MaterializationError {
	return &MaterializationError{Code: code, Category: category, Path: path, Cause: cause}
}

// materializationReferenceError converts resolver failures to generation failures.
func materializationReferenceError(err error, path schemaPath) error {
	var reference *schemaReferenceError
	if !errors.As(err, &reference) {
		return err
	}

	code := MaterializationCodeUnresolvedReference
	category := MaterializationCategoryUnresolvedReference

	switch reference.Kind {
	case schemaReferenceExternal:
		code = MaterializationCodeExternalReference
		category = MaterializationCategoryUnsupportedExternalReference

	case schemaReferenceCycle:
		code = MaterializationCodeReferenceRecursion
		category = MaterializationCategoryReferenceCycle
	}

	return newMaterializationError(code, category, materializationKeywordPath(path, "$ref"), err)
}

// isMaterializationReferenceError reports whether an error is reference-related.
func isMaterializationReferenceError(err error) bool {
	var materialization *MaterializationError
	return errors.As(err, &materialization) &&
		(materialization.Code == MaterializationCodeUnresolvedReference ||
			materialization.Code == MaterializationCodeExternalReference ||
			materialization.Code == MaterializationCodeReferenceRecursion)
}

// validationPath extracts a candidate failure path.
func validationPath(err error, fallback schemaPath) string {
	var validation *candidateValidationError
	if errors.As(err, &validation) {
		return validation.path
	}

	return pathPointer(fallback)
}

// isDefinitelyUnsatisfiable detects contradictions that need no search.
func isDefinitelyUnsatisfiable(schema effectiveSchema) bool {
	var intersection map[string]struct{}
	for _, declaration := range schema.types() {
		current := make(map[string]struct{})
		for _, name := range schemaTypeNames(declaration) {
			current[name] = struct{}{}
		}
		if intersection == nil {
			intersection = current
			continue
		}

		for name := range intersection {
			if _, ok := current[name]; !ok {
				delete(intersection, name)
			}
		}
	}

	if intersection != nil && len(intersection) == 0 {
		return true
	}

	if constants := schema.consts(); len(constants) > 0 {
		for _, enums := range schema.enums() {
			members := asSlice(enums)
			for _, constant := range constants {
				if !containsJSONValue(members, constant) {
					return true
				}
			}
		}
	}

	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		minimum, minOK := integerKeyword(term.Object, "minItems")
		maximum, maxOK := integerKeyword(term.Object, "maxItems")
		if minOK && maxOK && minimum > maximum {
			return true
		}

		minimum, minOK = integerKeyword(term.Object, "minLength")
		maximum, maxOK = integerKeyword(term.Object, "maxLength")
		if minOK && maxOK && minimum > maximum {
			return true
		}

		minimum, minOK = integerKeyword(term.Object, "minProperties")
		maximum, maxOK = integerKeyword(term.Object, "maxProperties")
		if minOK && maxOK && minimum > maximum {
			return true
		}
	}

	return false
}

// pathPointer returns a root-relative JSON Pointer for a schema location.
func pathPointer(path schemaPath) string {
	if len(path.segments) == 0 {
		return "#"
	}

	var builder strings.Builder
	builder.WriteByte('#')
	for _, segment := range path.segments {
		builder.WriteByte('/')
		if segment.kind == schemaPathArrayItem {
			builder.WriteString("items")
			continue
		}

		builder.WriteString(strings.NewReplacer("~", "~0", "/", "~1").Replace(segment.value))
	}

	return builder.String()
}

// materializationKeywordPath appends one keyword to an instance/schema path.
func materializationKeywordPath(path schemaPath, keyword string) string {
	return materializationKeywordPathValue(path, keyword)
}

// materializationKeywordPathValue returns a keyword pointer for a path.
func materializationKeywordPathValue(path schemaPath, keyword string) string {
	return pathPointer(path.append(schemaPathSegment{kind: schemaPathProperty, value: keyword}))
}
