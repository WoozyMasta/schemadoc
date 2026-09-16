// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"errors"
	"fmt"
	"math"
	"regexp"
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

	if typeName == "object" || len(properties) > 0 || len(required) > 0 {
		return materializer.synthesizeObject(properties, required, path)
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

// synthesizeObject materializes selected declared properties.
func (materializer *exampleMaterializer) synthesizeObject(
	properties map[string]effectiveSchema,
	required []string,
	path schemaPath,
) (map[string]any, error) {
	values := make(map[string]schemaValue, len(properties))
	for name, property := range properties {
		values[name] = property.asSchemaValue()
	}

	keys := propertyOrder(required, values)
	if materializer.mode == ExampleModeRequired {
		keys = requiredPropertyOrder(required, values)
	}

	if len(required) > 0 {
		for _, name := range required {
			if _, ok := properties[name]; !ok {
				return nil, newMaterializationError(
					MaterializationCodeUnsupportedRequired,
					MaterializationCategoryUnsupportedRequired,
					materializationKeywordPath(path, "required"),
					ErrUnsupportedRequiredSemantics,
				)
			}
		}
	}

	result := make(map[string]any, len(keys))
	for _, name := range keys {
		value, err := materializer.materializeEffective(properties[name], path.appendProperty(name))
		if err != nil {
			return nil, err
		}
		result[name] = value
	}

	return result, nil
}

// scalarPlaceholder returns fallback value for a known scalar type.
func scalarPlaceholder(schemaType string) (any, bool) {
	value, ok := exampleScalarPlaceholders[schemaType]
	return value, ok
}

// synthesizeArray materializes tuple prefixes or one homogeneous item.
func (materializer *exampleMaterializer) synthesizeArray(schema effectiveSchema, path schemaPath) ([]any, error) {
	items, err := schema.arrayItems(materializer.semantic)
	if err != nil {
		return nil, materializationReferenceError(err, path)
	}
	if len(items) == 0 {
		return []any{}, nil
	}

	current := items[0]
	result := make([]any, 0)
	if current.PrefixItems != nil {
		for _, item := range *current.PrefixItems {
			value, err := materializer.materializeEffective(item, path.appendArrayItem())
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
	}

	hasItemExamples := false
	if current.Items != nil {
		for _, raw := range current.Items.keywordValues("examples") {
			for _, example := range asSlice(raw) {
				if err := validateEffectiveInstance(
					materializer.semantic,
					*current.Items,
					example,
					path.appendArrayItem(),
				); err == nil {
					result = append(result, cloneJSONValue(example))
					hasItemExamples = true
				}
			}
		}

		if !hasItemExamples {
			value, err := materializer.materializeEffective(*current.Items, path.appendArrayItem())
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
	}

	minimum, maximum := arrayBounds(schema)
	if minimum > maximum || len(result) > maximum {
		return nil, newMaterializationError(
			MaterializationCodeUnsatisfiable,
			MaterializationCategoryUnsatisfiableSchema,
			materializationKeywordPath(path, "minItems"),
			ErrMaterializationUnsatisfiable,
		)
	}

	for len(result) < minimum {
		switch {
		case current.Items != nil:
			value, err := materializer.materializeEffective(*current.Items, path.appendArrayItem())
			if err != nil {
				return nil, err
			}
			result = append(result, value)

		case current.AdditionalItems != nil:
			value, err := materializer.materializeEffective(*current.AdditionalItems, path.appendArrayItem())
			if err != nil {
				return nil, err
			}
			result = append(result, value)

		default:
			result = append(result, nil)
		}
	}

	if len(result) > maximum {
		result = result[:maximum]
	}

	return result, nil
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

// validateArrayTerm validates tuple, homogeneous-item, and length rules.
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

	term, err := (effectiveSchema{terms: []schemaValue{{Object: object}}}).arrayItems(view)
	if err != nil {
		return materializationReferenceError(err, path)
	}

	if len(term) > 0 {
		current := term[0]
		for index, item := range items {
			var itemSchema *effectiveSchema

			switch {
			case current.PrefixItems != nil && index < len(*current.PrefixItems):
				itemSchema = &(*current.PrefixItems)[index]

			case current.Items != nil:
				itemSchema = current.Items

			case current.AdditionalItems != nil:
				itemSchema = current.AdditionalItems
			}

			if itemSchema == nil {
				continue
			}

			if err := validateEffectiveInstance(view, *itemSchema, item, path.appendArrayItem()); err != nil {
				return err
			}
		}
	}

	return nil
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
