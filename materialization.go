// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package schemadoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const maxGeneratedStringLength = 4096

const (
	maxCompositionCandidates = 64
	maxCompositionDepth      = 32
)

const materializationSchemaURL = "urn:schemadoc:materialization-schema"

// MaterializationErrorCode identifies a stable example-generation failure.
type MaterializationErrorCode string

const (
	// MaterializationCodeUnsatisfiable means no valid instance was found.
	MaterializationCodeUnsatisfiable MaterializationErrorCode = "unsatisfiable"
	// MaterializationCodeUnresolvedReference means a local reference target is missing.
	MaterializationCodeUnresolvedReference MaterializationErrorCode = "unresolved_reference"
	// MaterializationCodeExternalReference means a network or external resource is required.
	MaterializationCodeExternalReference MaterializationErrorCode = "unsupported_external_reference"
	// MaterializationCodeUnsupportedReference means a valid local reference form is unsupported.
	MaterializationCodeUnsupportedReference MaterializationErrorCode = "unsupported_reference"
	// MaterializationCodeInvalidReference means a local reference fragment is malformed.
	MaterializationCodeInvalidReference MaterializationErrorCode = "invalid_reference"
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
	// MaterializationCategoryUnsupportedReference identifies an unsupported local reference form.
	MaterializationCategoryUnsupportedReference MaterializationErrorCategory = "unsupported_reference"
	// MaterializationCategoryInvalidReference identifies a malformed local reference fragment.
	MaterializationCategoryInvalidReference MaterializationErrorCategory = "invalid_reference"
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
// Path is a logical instance/materialization location rendered in JSON Pointer-like notation;
// it is not a JSON Schema pointer.
type MaterializationError struct {
	Cause    error
	Code     MaterializationErrorCode
	Category MaterializationErrorCategory
	Path     string
}

// exampleMaterializer selects and validates one deterministic example value.
type exampleMaterializer struct {
	validatorError   error
	semantic         *schemaSemanticView
	activeRefs       map[string]int
	rootValidator    *jsonschema.Schema
	mode             ExampleMode
	compositionDepth int
}

// materializationCandidate records an explicit value and its source keyword.
type materializationCandidate struct {
	value  any
	source string
}

// candidateValidationError describes why one candidate was rejected.
type candidateValidationError struct {
	path    string
	keyword string
	reason  string
}

// exactNumericBounds stores the combined inclusive/exclusive numeric interval.
type exactNumericBounds struct {
	lower, upper       *big.Rat
	lowerExclusive     bool
	upperExclusive     bool
	hasLower, hasUpper bool
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
	case ErrUnsupportedSchemaReference:
		return err.Code == MaterializationCodeUnsupportedReference
	case ErrInvalidSchemaPointer:
		return err.Code == MaterializationCodeInvalidReference
	case ErrGeneratedValueInvalid:
		return err.Code == MaterializationCodeGeneratedValueInvalid
	default:
		return false
	}
}

// newExampleMaterializer creates a materializer for one parsed document.
func newExampleMaterializer(doc schemaDocument, mode ExampleMode) *exampleMaterializer {
	validator, err := compileMaterializationValidator(doc.Raw)
	return &exampleMaterializer{
		semantic:       newSchemaSemanticViewPointer(doc),
		mode:           mode,
		activeRefs:     make(map[string]int),
		rootValidator:  validator,
		validatorError: err,
	}
}

// materialize builds one valid value or returns a structured failure.
func (materializer *exampleMaterializer) materialize(node schemaValue) (any, error) {
	value, err := materializer.materializeAt(node, schemaPath{})
	if err != nil {
		return nil, err
	}

	if err := materializer.validateRoot(value); err != nil {
		return nil, err
	}

	return value, nil
}

// compileMaterializationValidator compiles one in-memory root schema
// without installing a URL loader, so final validation cannot perform implicit I/O.
func compileMaterializationValidator(raw any) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(materializationSchemaURL, raw); err != nil {
		return nil, fmt.Errorf("register schema: %w", err)
	}

	validator, err := compiler.Compile(materializationSchemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	return validator, nil
}

// validateRoot applies the authoritative independent validator once per output.
func (materializer *exampleMaterializer) validateRoot(value any) error {
	if materializer.validatorError != nil {
		return newMaterializationError(
			MaterializationCodeUnsupported,
			MaterializationCategoryUnsupportedMaterialization,
			"#",
			materializer.validatorError,
		)
	}

	if err := materializer.rootValidator.Validate(value); err != nil {
		return newMaterializationError(
			MaterializationCodeGeneratedValueInvalid,
			MaterializationCategoryGeneratedValueInvalid,
			"#",
			fmt.Errorf("final schema validation: %w", err),
		)
	}

	return nil
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
	if len(effective.deferredRefs) > 0 {
		return materializer.materializeDeferredReference(effective, path)
	}

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

// materializeDeferredReference follows a recursive edge only when it is not already active;
// this lets a finite composition branch win over recursion.
func (materializer *exampleMaterializer) materializeDeferredReference(
	effective effectiveSchema,
	path schemaPath,
) (any, error) {
	var value any
	hasValue := false

	for _, ref := range effective.deferredRefs {
		if materializer.activeRefs[ref] > 0 {
			return nil, newMaterializationError(
				MaterializationCodeReferenceRecursion,
				MaterializationCategoryReferenceCycle,
				materializationKeywordPath(path, "$ref"),
				ErrSchemaReferenceCycle,
			)
		}

		target, err := materializer.semantic.resolver.lookup(ref)
		if err != nil {
			return nil, materializationReferenceError(err, path)
		}

		materializer.activeRefs[ref]++
		candidate, err := materializer.materializeAt(target, path)
		materializer.activeRefs[ref]--
		if materializer.activeRefs[ref] <= 0 {
			delete(materializer.activeRefs, ref)
		}
		if err != nil {
			return nil, err
		}

		if hasValue {
			if !equalJSONValue(value, candidate) {
				return nil, newMaterializationError(
					MaterializationCodeUnsupported,
					MaterializationCategoryUnsupportedMaterialization,
					materializationKeywordPath(path, "$ref"),
					ErrUnsupportedMaterialization,
				)
			}
		} else {
			value = candidate
			hasValue = true
		}
	}

	local := effective
	local.deferredRefs = nil
	if err := validateEffectiveInstance(materializer.semantic, local, value, path); err != nil {
		return nil, err
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

	if typeName == "string" {
		return materializer.synthesizeString(schema, path)
	}

	if typeName == "number" || typeName == "integer" {
		return materializer.synthesizeNumber(schema, path)
	}

	if value, ok := scalarPlaceholder(typeName); ok {
		return value, nil
	}

	return nil, nil
}

// synthesizeString tries readable format and conservative pattern candidates
// before adjusting the generic placeholder to the declared Unicode length.
func (materializer *exampleMaterializer) synthesizeString(schema effectiveSchema, path schemaPath) (string, error) {
	candidates := stringCandidates(schema)

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		if err := validateEffectiveInstance(materializer.semantic, schema, candidate, path); err == nil {
			return candidate, nil
		} else if isMaterializationReferenceError(err) {
			return "", err
		}
	}

	if isDefinitelyUnsatisfiable(schema) {
		return "", newMaterializationError(
			MaterializationCodeUnsatisfiable,
			MaterializationCategoryUnsatisfiableSchema,
			unsatisfiableConstraintPath(schema, path),
			ErrMaterializationUnsatisfiable,
		)
	}

	keyword := "minLength"
	if hasSchemaKeyword(schema, "pattern") {
		keyword = "pattern"
	}
	return "", newMaterializationError(
		MaterializationCodeUnsupported,
		MaterializationCategoryUnsupportedMaterialization,
		materializationKeywordPath(path, keyword),
		ErrUnsupportedMaterialization,
	)
}

// stringCandidates returns the finite fallback
// set shared by scalar synthesis and composition search.
func stringCandidates(schema effectiveSchema) []string {
	candidates := make([]string, 0, 8)
	minimum, maximum := stringBounds(schema)
	if value, ok := knownPatternExample(schema); ok {
		candidates = append(candidates, value)
	}
	if value, ok := formatStringExample(schema); ok {
		candidates = append(candidates, value)
	}
	for _, value := range simplePatternExamples(schema) {
		candidates = append(candidates, value)
		if minimum <= maximum && minimum > 1 && minimum <= maxGeneratedStringLength {
			candidates = append(candidates, strings.Repeat(value, minimum))
		}
	}
	candidates = append(candidates, "<string>")

	if minimum <= maximum && minimum <= maxGeneratedStringLength {
		candidates = append(candidates, strings.Repeat("a", minimum))
	}

	return candidates
}

// synthesizeNumber tries boundary and multipleOf-derived values in stable order.
func (materializer *exampleMaterializer) synthesizeNumber(schema effectiveSchema, path schemaPath) (any, error) {
	candidates := numberCandidates(schema)
	for _, candidate := range candidates {
		if effectiveSchemaType(schema) == "integer" {
			number, ok := asExactNumber(candidate)
			if !ok || !number.IsInt() {
				continue
			}
		}
		if err := validateEffectiveInstance(materializer.semantic, schema, candidate, path); err == nil {
			return candidate, nil
		} else if isMaterializationReferenceError(err) {
			return nil, err
		}
	}

	if isDefinitelyUnsatisfiable(schema) {
		return nil, newMaterializationError(
			MaterializationCodeUnsatisfiable,
			MaterializationCategoryUnsatisfiableSchema,
			unsatisfiableConstraintPath(schema, path),
			ErrMaterializationUnsatisfiable,
		)
	}

	return nil, newMaterializationError(
		MaterializationCodeUnsupported,
		MaterializationCategoryUnsupportedMaterialization,
		pathPointer(path),
		ErrUnsupportedMaterialization,
	)
}

// synthesizeComposition evaluates branch-generated candidates against the whole schema.
func (materializer *exampleMaterializer) synthesizeComposition(schema effectiveSchema, path schemaPath) (any, error) {
	if materializer.compositionDepth >= maxCompositionDepth {
		return nil, materializationCompositionSearchError(schema, path)
	}

	materializer.compositionDepth++
	defer func() { materializer.compositionDepth-- }()

	candidates := make([]any, 0, maxCompositionCandidates)
	for _, group := range schema.compositionGroups {
		for _, branch := range group.branches {
			materializer.appendCompositionCandidates(&candidates, branch, path)
		}
	}

	for _, candidate := range candidates {
		if err := validateEffectiveInstance(materializer.semantic, schema, candidate, path); err == nil {
			return candidate, nil
		} else if isMaterializationReferenceError(err) {
			return nil, err
		}
	}

	if isDefinitelyUnsatisfiable(schema) {
		return nil, newMaterializationError(
			MaterializationCodeUnsatisfiable,
			MaterializationCategoryUnsatisfiableSchema,
			pathPointer(path),
			ErrMaterializationUnsatisfiable,
		)
	}

	return nil, materializationCompositionSearchError(schema, path)
}

// appendCompositionCandidates gathers a bounded set of branch-local values.
// The whole composition is checked again by the caller.
func (materializer *exampleMaterializer) appendCompositionCandidates(
	candidates *[]any,
	branch effectiveSchema,
	path schemaPath,
) {
	appendCandidate := func(value any) {
		if len(*candidates) >= maxCompositionCandidates {
			return
		}
		if err := validateEffectiveInstance(materializer.semantic, branch, value, path); err != nil {
			return
		}
		for _, existing := range *candidates {
			if equalJSONValue(existing, value) {
				return
			}
		}
		*candidates = append(*candidates, value)
	}

	for _, candidate := range materializer.explicitCandidates(branch) {
		appendCandidate(candidate.value)
	}

	switch effectiveSchemaType(branch) {
	case "number", "integer":
		for _, candidate := range numberCandidates(branch) {
			appendCandidate(candidate)
		}
	case "string":
		for _, candidate := range stringCandidates(branch) {
			appendCandidate(candidate)
		}
	default:
		if candidate, ok := scalarPlaceholder(effectiveSchemaType(branch)); ok {
			appendCandidate(candidate)
		}
	}

	// Object and array branches need nested schemas materialized;
	// an error here does not invalidate other composition branches.
	if value, err := materializer.materializeEffective(branch, path); err == nil {
		appendCandidate(value)
	}
}

// materializationCompositionSearchError distinguishes bounded search failure
// from a proof that the composition has no valid instance.
func materializationCompositionSearchError(schema effectiveSchema, path schemaPath) error {
	keyword := "anyOf"
	for _, group := range schema.compositionGroups {
		if group.kind == schemaCompositionOneOf {
			keyword = "oneOf"
			break
		}
	}

	return newMaterializationError(
		MaterializationCodeUnsupported,
		MaterializationCategoryUnsupportedMaterialization,
		materializationKeywordPath(path, keyword),
		ErrUnsupportedMaterialization,
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
	required = materializer.objectDependentRequired(schema, properties, required)
	required = materializer.objectDependentSchemaRequired(schema, properties, required)
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
	if !hasAdditional {
		unevaluated, err := schema.unevaluatedProperties(materializer.semantic)
		if err != nil {
			return nil, err
		}
		additional, hasAdditional = combinedEffectiveSchema(unevaluated)
	}
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

// objectDependentRequired expands dependencies for properties selected by the mode.
func (materializer *exampleMaterializer) objectDependentRequired(
	schema effectiveSchema,
	properties map[string]effectiveSchema,
	required []string,
) []string {
	result := append([]string(nil), required...)
	seen := make(map[string]struct{}, len(result))
	for _, name := range result {
		seen[name] = struct{}{}
	}

	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}
		dependencies, ok := term.Object["dependentRequired"].(map[string]any)
		if !ok {
			continue
		}

		for _, trigger := range sortedKeys(dependencies) {
			property, exists := properties[trigger]
			if !exists || isFalseEffectiveSchema(property) {
				continue
			}
			if materializer.mode == ExampleModeRequired && !slices.Contains(result, trigger) {
				continue
			}

			for _, dependency := range asStringSlice(dependencies[trigger]) {
				if _, exists := seen[dependency]; exists {
					continue
				}
				result = append(result, dependency)
				seen[dependency] = struct{}{}
			}
		}
	}

	return result
}

// objectDependentSchemaRequired adds direct required properties from active dependent schemas.
func (materializer *exampleMaterializer) objectDependentSchemaRequired(
	schema effectiveSchema,
	properties map[string]effectiveSchema,
	required []string,
) []string {
	result := append([]string(nil), required...)
	seen := make(map[string]struct{}, len(result))
	for _, name := range result {
		seen[name] = struct{}{}
	}

	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}
		dependencies, ok := term.Object["dependentSchemas"].(map[string]any)
		if !ok {
			continue
		}

		for _, trigger := range sortedKeys(dependencies) {
			property, exists := properties[trigger]
			if !exists || isFalseEffectiveSchema(property) {
				continue
			}
			if materializer.mode == ExampleModeRequired && !slices.Contains(result, trigger) {
				continue
			}

			node, valid := toSchemaValue(dependencies[trigger])
			if !valid {
				continue
			}

			nested, err := materializer.semantic.effective(node)
			if err != nil {
				continue
			}

			for _, dependency := range nested.required() {
				if _, exists := seen[dependency]; exists {
					continue
				}
				result = append(result, dependency)
				seen[dependency] = struct{}{}
			}
		}
	}

	return result
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

// stringBounds returns the narrowest Unicode length interval across all terms.
func stringBounds(schema effectiveSchema) (int, int) {
	minimum, maximum := 0, int(^uint(0)>>1)
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}
		if value, ok := integerKeyword(term.Object, "minLength"); ok && value > minimum {
			minimum = value
		}
		if value, ok := integerKeyword(term.Object, "maxLength"); ok && value < maximum {
			maximum = value
		}
	}

	return minimum, maximum
}

// formatStringExample returns readable examples for common annotation formats.
func formatStringExample(schema effectiveSchema) (string, bool) {
	format := strings.ToLower(asStringValue(schema.annotation("format")))
	switch format {
	case "date":
		return "2000-01-01", true
	case "date-time":
		return "2000-01-01T00:00:00Z", true
	case "duration":
		return "P1D", true
	case "email", "idn-email":
		return "user@example.com", true
	case "hostname", "idn-hostname":
		return "example.com", true
	case "ipv4":
		return "192.0.2.1", true
	case "ipv6":
		return "2001:db8::1", true
	case "time":
		return "00:00:00Z", true
	case "uri", "uri-reference", "uri-template", "url":
		return "https://example.com", true
	case "uuid":
		return "00000000-0000-4000-8000-000000000000", true
	default:
		return "", false
	}
}

// asStringValue extracts a string annotation without changing schema accessors.
func asStringValue(value any, ok bool) string {
	if !ok {
		return ""
	}

	return asString(value)
}

// simplePatternExamples keeps one candidate for every supported pattern term
// so conjunctive patterns can converge on a value satisfying them all.
func simplePatternExamples(schema effectiveSchema) []string {
	values := make([]string, 0)
	seen := make(map[string]struct{})
	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		pattern := asString(term.Object["pattern"])
		if value, ok := simplePatternValue(pattern); ok {
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}

	return values
}

// simplePatternValue creates a candidate only for a small allowlist of regex shapes.
func simplePatternValue(pattern string) (string, bool) {
	switch pattern {
	case "^[a-z]+$", "^[A-Z]+$", "^[0-9]+$", "^[A-Za-z0-9_-]+$", "^[a-z][a-z0-9_]*$":
		return map[string]string{
			"^[a-z]+$":          "a",
			"^[A-Z]+$":          "A",
			"^[0-9]+$":          "0",
			"^[A-Za-z0-9_-]+$":  "a",
			"^[a-z][a-z0-9_]*$": "a",
		}[pattern], true
	}

	for _, class := range []struct {
		prefix string
		value  string
	}{
		{prefix: "^[a-z]{", value: "a"},
		{prefix: "^[A-Z]{", value: "A"},
		{prefix: "^[0-9]{", value: "0"},
	} {
		if !strings.HasPrefix(pattern, class.prefix) || !strings.HasSuffix(pattern, "}$") {
			continue
		}

		count, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(pattern, class.prefix), "}$"))
		if err == nil && count > 0 && count <= maxGeneratedStringLength {
			return strings.Repeat(class.value, count), true
		}
	}

	if strings.HasPrefix(pattern, "^") && strings.HasSuffix(pattern, "$") {
		literal := strings.TrimSuffix(strings.TrimPrefix(pattern, "^"), "$")
		if literal != "" && !strings.ContainsAny(literal, `\\.*+?()[]{}|^$`) {
			return literal, true
		}
	}

	return "", false
}

// numberCandidates returns stable boundary and multipleOf candidates.
func numberCandidates(schema effectiveSchema) []any {
	result := []any{json.Number("0"), json.Number("1"), json.Number("-1")}
	integer := effectiveSchemaType(schema) == "integer"
	bounds := numericBounds(schema)
	appendNumber := func(result []any, value *big.Rat) []any {
		encoded, ok := exactNumberJSON(value)
		if !ok {
			return result
		}

		return append(result, encoded)
	}

	if bounds.hasLower {
		candidate := new(big.Rat).Set(bounds.lower)
		if integer {
			candidate.SetInt(exactRatCeil(candidate))
			if bounds.lowerExclusive && candidate.Cmp(bounds.lower) <= 0 {
				candidate.Add(candidate, big.NewRat(1, 1))
			}
		} else if bounds.lowerExclusive {
			candidate.Add(candidate, exactDecimalUnit(candidate))
		}
		result = appendNumber(result, candidate)
	}

	if bounds.hasUpper {
		candidate := new(big.Rat).Set(bounds.upper)
		if integer {
			candidate.SetInt(exactRatFloor(candidate))
			if bounds.upperExclusive && candidate.Cmp(bounds.upper) >= 0 {
				candidate.Sub(candidate, big.NewRat(1, 1))
			}
		} else if bounds.upperExclusive {
			candidate.Sub(candidate, exactDecimalUnit(candidate))
		}
		result = appendNumber(result, candidate)
	}

	if bounds.hasLower && bounds.hasUpper && bounds.lower.Cmp(bounds.upper) < 0 && !integer {
		midpoint := new(big.Rat).Add(bounds.lower, bounds.upper)
		midpoint.Quo(midpoint, big.NewRat(2, 1))
		result = appendNumber(result, midpoint)
	}

	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		multiple, ok := asExactNumber(term.Object["multipleOf"])
		if !ok || multiple.Sign() <= 0 {
			continue
		}

		result = appendNumber(result, multiple)
		result = appendNumber(result, new(big.Rat).Neg(multiple))

		if bounds.hasLower {
			factor := exactRatCeil(new(big.Rat).Quo(bounds.lower, multiple))
			candidate := new(big.Rat).Mul(multiple, new(big.Rat).SetInt(factor))
			if bounds.lowerExclusive && candidate.Cmp(bounds.lower) <= 0 {
				factor.Add(factor, big.NewInt(1))
				candidate.Mul(multiple, new(big.Rat).SetInt(factor))
			}
			result = appendNumber(result, candidate)
		}

		if bounds.hasUpper {
			factor := exactRatFloor(new(big.Rat).Quo(bounds.upper, multiple))
			candidate := new(big.Rat).Mul(multiple, new(big.Rat).SetInt(factor))
			if bounds.upperExclusive && candidate.Cmp(bounds.upper) >= 0 {
				factor.Sub(factor, big.NewInt(1))
				candidate.Mul(multiple, new(big.Rat).SetInt(factor))
			}
			result = appendNumber(result, candidate)
		}
	}

	return result
}

// numericBounds returns the combined inclusive/exclusive numeric interval.
func numericBounds(schema effectiveSchema) exactNumericBounds {
	bounds := exactNumericBounds{}

	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		if value, ok := asExactNumber(term.Object["minimum"]); ok {
			exclusive := false
			if flag, isBool := term.Object["exclusiveMinimum"].(bool); isBool {
				exclusive = flag
			} else if bound, isNumber := asExactNumber(term.Object["exclusiveMinimum"]); isNumber {
				value, exclusive = bound, true
			}
			updateLowerBound(&bounds, value, exclusive)
		} else if value, ok := asExactNumber(term.Object["exclusiveMinimum"]); ok {
			updateLowerBound(&bounds, value, true)
		}

		if value, ok := asExactNumber(term.Object["maximum"]); ok {
			exclusive := false
			if flag, isBool := term.Object["exclusiveMaximum"].(bool); isBool {
				exclusive = flag
			} else if bound, isNumber := asExactNumber(term.Object["exclusiveMaximum"]); isNumber {
				value, exclusive = bound, true
			}
			updateUpperBound(&bounds, value, exclusive)
		} else if value, ok := asExactNumber(term.Object["exclusiveMaximum"]); ok {
			updateUpperBound(&bounds, value, true)
		}
	}

	return bounds
}

// updateLowerBound keeps the strictest lower numeric bound.
func updateLowerBound(bounds *exactNumericBounds, value *big.Rat, exclusive bool) {
	comparison := 0
	if bounds.hasLower {
		comparison = value.Cmp(bounds.lower)
	}
	if !bounds.hasLower || comparison > 0 ||
		(comparison == 0 && exclusive && !bounds.lowerExclusive) {
		bounds.lower, bounds.hasLower = value, true
		bounds.lowerExclusive = exclusive
	}
}

// updateUpperBound keeps the strictest upper numeric bound.
func updateUpperBound(bounds *exactNumericBounds, value *big.Rat, exclusive bool) {
	comparison := 0
	if bounds.hasUpper {
		comparison = value.Cmp(bounds.upper)
	}
	if !bounds.hasUpper || comparison < 0 ||
		(comparison == 0 && exclusive && !bounds.upperExclusive) {
		bounds.upper, bounds.hasUpper = value, true
		bounds.upperExclusive = exclusive
	}
}

// exactRatCeil returns the least integer greater than or equal to value.
func exactRatCeil(value *big.Rat) *big.Int {
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(value.Num(), value.Denom(), remainder)
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}

	return quotient
}

// exactRatFloor returns the greatest integer less than or equal to value.
func exactRatFloor(value *big.Rat) *big.Int {
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(value.Num(), value.Denom(), remainder)
	if remainder.Sign() < 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}

	return quotient
}

// exactDecimalUnit returns one decimal place smaller than value's precision.
func exactDecimalUnit(value *big.Rat) *big.Rat {
	scale, ok := exactDecimalScale(value.Denom())
	if !ok {
		return big.NewRat(1, 1000000)
	}

	unit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	return new(big.Rat).SetFrac(big.NewInt(1), unit)
}

// exactNumberJSON serializes a finite decimal rational as a JSON number.
func exactNumberJSON(value *big.Rat) (json.Number, bool) {
	scale, ok := exactDecimalScale(value.Denom())
	if !ok {
		return "", false
	}

	return json.Number(value.FloatString(scale)), true
}

// exactDecimalScale returns the minimum decimal scale for a finite rational.
func exactDecimalScale(value *big.Int) (int, bool) {
	denominator := new(big.Int).Set(value)
	twoScale := 0
	for denominator.Bit(0) == 0 {
		denominator.Rsh(denominator, 1)
		twoScale++
	}

	fiveScale := 0
	five := big.NewInt(5)
	for new(big.Int).Mod(denominator, five).Sign() == 0 {
		denominator.Quo(denominator, five)
		fiveScale++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return 0, false
	}

	return max(twoScale, fiveScale), true
}

// synthesizeArray materializes a useful array within the declared item bounds,
// then satisfies contains and item-count requirements without exceeding maxItems.
// Tuple positions constrain the indexes that are present;
// they do not impose a minimum length.
// Each emitted index receives the conjunction of all applicable item schemas,
// so heterogeneous tuple positions are not treated as homogeneous.
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
	// Tuple and prefix schemas constrain existing positions only.
	// Limit the illustrative prefix to maxItems
	// instead of treating omitted positions as a contradiction.
	prefixLength = min(prefixLength, maximum)
	result := make([]any, 0)
	for index := range prefixLength {
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
			// A tuple position is optional unless minItems reaches it.
			// If its schema cannot produce a value, retain the valid shorter prefix
			// and let final validation decide whether the minimum is still met.
			if len(result) >= minimum {
				break
			}

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
		// Prefer an existing match;
		// when the requirement still lacks enough matching values
		// append or replace an emitted item.
		for containsCount(materializer.semantic, requirement.Schema, result, path) < requirement.Minimum {
			if len(result) >= maximum {
				replaced := false
				for _, index := range replaceableArrayItemIndices(result, materializer.semantic, requirement.Schema, path) {
					item, hasItem, err := arrayItemSchemaAt(items, index)
					if err != nil {
						return nil, err
					}

					candidateSchema := combineOptionalArraySchemas(item, hasItem, requirement.Schema)
					value, err := materializer.materializeArrayValue(candidateSchema, true, result[:index], schema, path)
					if err != nil {
						if isMaterializationReferenceError(err) {
							return nil, err
						}

						continue
					}

					result[index] = value
					replaced = true
					break
				}

				if replaced {
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
		// Positions beyond an emitted tuple prefix use items/additionalItems first.
		// In modern drafts, unevaluatedItems is the fallback
		// for indexes outside prefixItems when no items schema applies.
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

// arrayPrefixLength returns the longest described tuple prefix across simultaneous terms.
// The result is an illustrative generation limit;
// it does not imply that every position must be present in the instance.
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

// replaceableArrayItemIndices returns non-matching emitted items in reverse order
// so contains replacement remains deterministic
// while allowing an earlier tuple position when a later position is incompatible.
func replaceableArrayItemIndices(
	values []any,
	view *schemaSemanticView,
	schema effectiveSchema,
	path schemaPath,
) []int {
	indices := make([]int, 0, len(values))
	for index := range slices.Backward(values) {
		if validateEffectiveInstance(view, schema, values[index], path.appendArrayItem()) != nil {
			indices = append(indices, index)
		}
	}

	return indices
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
	if len(schema.deferredRefs) > 0 {
		return newCandidateValidationError(
			materializationKeywordPath(path, "$ref"),
			"$ref",
			"recursive reference is deferred")
	}

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
	if err := validateNotTerm(view, object, value, path); err != nil {
		return err
	}
	if err := validateConditionalTerm(view, object, value, path); err != nil {
		return err
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

// validateNotTerm rejects values accepted by the nested not schema.
func validateNotTerm(view *schemaSemanticView, object map[string]any, value any, path schemaPath) error {
	raw, exists := object["not"]
	if !exists {
		return nil
	}

	node, valid := toSchemaValue(raw)
	if !valid {
		return nil
	}

	nested, err := view.effective(node)
	if err != nil {
		return materializationReferenceError(err, path)
	}
	if err := validateEffectiveInstance(view, nested, value, path); err == nil {
		return newCandidateValidationError(
			materializationKeywordPath(path, "not"),
			"not",
			"value matches the forbidden schema")
	} else if isMaterializationReferenceError(err) {
		return err
	}

	return nil
}

// validateConditionalTerm applies then or else according to the if condition.
func validateConditionalTerm(view *schemaSemanticView, object map[string]any, value any, path schemaPath) error {
	rawCondition, exists := object["if"]
	if !exists {
		return nil
	}

	conditionNode, valid := toSchemaValue(rawCondition)
	if !valid {
		return nil
	}
	condition, err := view.effective(conditionNode)
	if err != nil {
		return materializationReferenceError(err, path)
	}
	conditionErr := validateEffectiveInstance(view, condition, value, path)
	if isMaterializationReferenceError(conditionErr) {
		return conditionErr
	}
	conditionMatches := conditionErr == nil

	keyword := "else"
	if conditionMatches {
		keyword = "then"
	}
	rawBranch, exists := object[keyword]
	if !exists {
		return nil
	}

	branchNode, valid := toSchemaValue(rawBranch)
	if !valid {
		return nil
	}
	branch, err := view.effective(branchNode)
	if err != nil {
		return materializationReferenceError(err, path)
	}

	return validateEffectiveInstance(view, branch, value, path)
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
	number, ok := asExactNumber(value)
	if !ok {
		return nil
	}

	if minimum, ok := asExactNumber(object["minimum"]); ok {
		exclusive := false
		if flag, isBool := object["exclusiveMinimum"].(bool); isBool {
			exclusive = flag
		} else if bound, isNumber := asExactNumber(object["exclusiveMinimum"]); isNumber {
			minimum = bound
			exclusive = true
		}

		comparison := number.Cmp(minimum)
		if (exclusive && comparison <= 0) || (!exclusive && comparison < 0) {
			return newCandidateValidationError(
				materializationKeywordPath(path, "minimum"),
				"minimum",
				"number is below minimum")
		}
	} else if minimum, ok := asExactNumber(object["exclusiveMinimum"]); ok && number.Cmp(minimum) <= 0 {
		return newCandidateValidationError(
			materializationKeywordPath(path, "exclusiveMinimum"),
			"exclusiveMinimum",
			"number is below exclusive minimum")
	}

	if maximum, ok := asExactNumber(object["maximum"]); ok {
		exclusive := false
		if flag, isBool := object["exclusiveMaximum"].(bool); isBool {
			exclusive = flag
		} else if bound, isNumber := asExactNumber(object["exclusiveMaximum"]); isNumber {
			maximum = bound
			exclusive = true
		}

		comparison := number.Cmp(maximum)
		if (exclusive && comparison >= 0) || (!exclusive && comparison > 0) {
			return newCandidateValidationError(
				materializationKeywordPath(path, "maximum"),
				"maximum",
				"number is above maximum")
		}
	} else if maximum, ok := asExactNumber(object["exclusiveMaximum"]); ok && number.Cmp(maximum) >= 0 {
		return newCandidateValidationError(
			materializationKeywordPath(path, "exclusiveMaximum"),
			"exclusiveMaximum",
			"number is above exclusive maximum")
	}

	if multiple, ok := asExactNumber(object["multipleOf"]); ok && multiple.Sign() > 0 {
		quotient := new(big.Rat).Quo(number, multiple)
		if !quotient.IsInt() {
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

	if err := validateDependentRequired(object, properties, path); err != nil {
		return err
	}
	if err := validateDependentSchemas(view, object, properties, path); err != nil {
		return err
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

	if err := validateUnevaluatedProperties(view, object, properties, declared, patterns, path); err != nil {
		return err
	}

	return nil
}

// validateDependentRequired enforces property dependencies on object instances.
func validateDependentRequired(
	object map[string]any,
	properties map[string]any,
	path schemaPath,
) error {
	dependencies, ok := object["dependentRequired"].(map[string]any)
	if !ok {
		return nil
	}

	for trigger, raw := range dependencies {
		if _, exists := properties[trigger]; !exists {
			continue
		}

		for _, dependency := range asStringSlice(raw) {
			if _, exists := properties[dependency]; exists {
				continue
			}

			return newCandidateValidationError(
				materializationKeywordPath(path, "dependentRequired"),
				"dependentRequired",
				"dependent property is missing")
		}
	}

	return nil
}

// validateDependentSchemas applies the schema selected by each present trigger property.
func validateDependentSchemas(
	view *schemaSemanticView,
	object map[string]any,
	properties map[string]any,
	path schemaPath,
) error {
	dependencies, ok := object["dependentSchemas"].(map[string]any)
	if !ok {
		return nil
	}

	for trigger, raw := range dependencies {
		if _, exists := properties[trigger]; !exists {
			continue
		}

		node, valid := toSchemaValue(raw)
		if !valid {
			continue
		}
		nested, err := view.effective(node)
		if err != nil {
			return materializationReferenceError(err, path)
		}
		if err := validateEffectiveInstance(view, nested, properties, path); err != nil {
			return err
		}
	}

	return nil
}

// validateUnevaluatedProperties validates properties not covered by this term's applicators.
func validateUnevaluatedProperties(
	view *schemaSemanticView,
	object map[string]any,
	properties map[string]any,
	declared map[string]schemaValue,
	patterns map[string]*regexp.Regexp,
	path schemaPath,
) error {
	if view.dialect != schemaDialect201909 && view.dialect != schemaDialect202012 {
		return nil
	}

	raw, exists := object["unevaluatedProperties"]
	if !exists {
		return nil
	}
	node, valid := toSchemaValue(raw)
	if !valid {
		return nil
	}
	unevaluated, err := view.effective(node)
	if err != nil {
		return materializationReferenceError(err, path)
	}

	evaluated := make(map[string]struct{}, len(properties))
	for name := range declared {
		evaluated[name] = struct{}{}
	}
	for name := range properties {
		if _, exists := evaluated[name]; exists {
			continue
		}
		for _, pattern := range patterns {
			if pattern.MatchString(name) {
				evaluated[name] = struct{}{}
				break
			}
		}
	}

	if _, exists := object["additionalProperties"]; exists {
		for name := range properties {
			evaluated[name] = struct{}{}
		}
	}

	for name, value := range properties {
		if _, exists := evaluated[name]; exists {
			continue
		}
		if err := validateEffectiveInstance(view, unevaluated, value, path.appendProperty(name)); err != nil {
			return err
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
	if !ok || !view.arrayCapabilities().UnevaluatedItems {
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
		containsEvaluated := view.arrayCapabilities().ContainsEvaluatesItems &&
			arrayItemMatchesContains(view, contains, item, path)
		if arrayItemEvaluated(ordinary, index) || containsEvaluated {
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
		_, ok := asExactNumber(value)
		return ok

	case "integer":
		number, ok := asExactNumber(value)
		return ok && number.IsInt()

	default:
		return false
	}
}

// equalJSONValue compares decoded JSON values while normalizing numeric types.
func equalJSONValue(left, right any) bool {
	if leftNumber, leftOK := asExactNumber(left); leftOK {
		rightNumber, rightOK := asExactNumber(right)
		return rightOK && leftNumber.Cmp(rightNumber) == 0
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
	value, ok := asExactNumber(object[keyword])
	if !ok || value.Sign() < 0 || !value.IsInt() || !value.Num().IsInt64() {
		return 0, false
	}

	integer := value.Num().Int64()
	maximum := int64(^uint(0) >> 1)
	if integer > maximum {
		return 0, false
	}

	return int(integer), true
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

	case schemaReferenceUnsupported:
		code = MaterializationCodeUnsupportedReference
		category = MaterializationCategoryUnsupportedReference

	case schemaReferenceInvalid:
		code = MaterializationCodeInvalidReference
		category = MaterializationCategoryInvalidReference

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
			materialization.Code == MaterializationCodeUnsupportedReference ||
			materialization.Code == MaterializationCodeInvalidReference ||
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
	for _, group := range schema.compositionGroups {
		allBranchesUnsatisfiable := len(group.branches) > 0
		for _, branch := range group.branches {
			if !isFalseEffectiveSchema(branch) && !isDefinitelyUnsatisfiable(branch) {
				allBranchesUnsatisfiable = false
				break
			}
		}
		if allBranchesUnsatisfiable {
			return true
		}
	}

	if schemaTypesContradict(schema) {
		return true
	}
	if scalarEnumerationConstraintsContradict(schema) {
		return true
	}
	if numericConstraintsDefinitelyUnsatisfiable(schema) {
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

// scalarEnumerationConstraintsContradict checks intersections
// that can be decided without traversing nested schemas or resolving references.
func scalarEnumerationConstraintsContradict(schema effectiveSchema) bool {
	constants := schema.consts()
	if len(constants) > 1 {
		for _, value := range constants[1:] {
			if !equalJSONValue(constants[0], value) {
				return true
			}
		}
	}

	if len(constants) > 0 {
		constant := constants[0]
		for _, declaration := range schema.types() {
			if !matchesSchemaType(constant, declaration) {
				return true
			}
		}
	}

	enums := schema.enums()
	if len(enums) > 1 {
		members := asSlice(enums[0])
		for _, candidate := range members {
			admissible := true
			for _, other := range enums[1:] {
				if !containsJSONValue(asSlice(other), candidate) {
					admissible = false
					break
				}
			}

			if admissible {
				return false
			}
		}

		return true
	}

	if len(constants) > 0 {
		for _, raw := range enums {
			if !containsJSONValue(asSlice(raw), constants[0]) {
				return true
			}
		}
	}

	if len(constants) == 0 && len(enums) == 1 {
		for _, candidate := range asSlice(enums[0]) {
			valid := true
			for _, declaration := range schema.types() {
				if !matchesSchemaType(candidate, declaration) {
					valid = false
					break
				}
			}

			if valid {
				return false
			}
		}

		return true
	}

	return false
}

// numericConstraintsDefinitelyUnsatisfiable proves an empty numeric interval
// when a positive multipleOf cannot place even one value inside it.
// It only runs for exclusively numeric schemas;
// a schema that also admits strings or another type
// may still have valid instances despite numeric constraints.
func numericConstraintsDefinitelyUnsatisfiable(schema effectiveSchema) bool {
	typeName := effectiveSchemaType(schema)
	if typeName != "number" && typeName != "integer" {
		return false
	}

	bounds := numericBounds(schema)
	if bounds.hasLower && bounds.hasUpper &&
		(bounds.lower.Cmp(bounds.upper) > 0 ||
			(bounds.lower.Cmp(bounds.upper) == 0 &&
				(bounds.lowerExclusive || bounds.upperExclusive))) {
		return true
	}

	if typeName == "integer" && bounds.hasLower && bounds.hasUpper {
		first := exactRatCeil(bounds.lower)
		if bounds.lowerExclusive && new(big.Rat).SetInt(first).Cmp(bounds.lower) <= 0 {
			first.Add(first, big.NewInt(1))
		}
		last := exactRatFloor(bounds.upper)
		if bounds.upperExclusive && new(big.Rat).SetInt(last).Cmp(bounds.upper) >= 0 {
			last.Sub(last, big.NewInt(1))
		}
		if first.Cmp(last) > 0 {
			return true
		}
	}

	if !bounds.hasLower || !bounds.hasUpper {
		return false
	}

	for _, term := range schema.terms {
		if term.Object == nil {
			continue
		}

		multiple, ok := asExactNumber(term.Object["multipleOf"])
		if !ok || multiple.Sign() <= 0 {
			continue
		}

		first := exactRatCeil(new(big.Rat).Quo(bounds.lower, multiple))
		firstCandidate := new(big.Rat).Mul(multiple, new(big.Rat).SetInt(first))
		if bounds.lowerExclusive && firstCandidate.Cmp(bounds.lower) <= 0 {
			first.Add(first, big.NewInt(1))
		}
		last := exactRatFloor(new(big.Rat).Quo(bounds.upper, multiple))
		lastCandidate := new(big.Rat).Mul(multiple, new(big.Rat).SetInt(last))
		if bounds.upperExclusive && lastCandidate.Cmp(bounds.upper) >= 0 {
			last.Sub(last, big.NewInt(1))
		}
		if first.Cmp(last) > 0 {
			return true
		}
	}

	return false
}

// unsatisfiableConstraintPath points diagnostics at the first constraint
// that can prove the effective schema has no valid instance.
func unsatisfiableConstraintPath(schema effectiveSchema, path schemaPath) string {
	if schemaTypesContradict(schema) {
		return materializationKeywordPath(path, "type")
	}

	return pathPointer(path)
}

// schemaTypesContradict reports an empty intersection of simultaneous types.
func schemaTypesContradict(schema effectiveSchema) bool {
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

	return intersection != nil && len(intersection) == 0
}

// pathPointer renders a logical instance path in JSON Pointer-like notation.
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

// materializationKeywordPath appends one keyword to a logical instance path.
func materializationKeywordPath(path schemaPath, keyword string) string {
	return materializationKeywordPathValue(path, keyword)
}

// materializationKeywordPathValue returns a keyword location for a logical path.
func materializationKeywordPathValue(path schemaPath, keyword string) string {
	return pathPointer(path.append(schemaPathSegment{kind: schemaPathProperty, value: keyword}))
}
