// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/schemadoc

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/woozymasta/schemadoc"
	"github.com/woozymasta/schemadoc/cmd/schemadoc/buildcfg"
	"go.yaml.in/yaml/v3"
)

const (
	buildConfigSchemaID   = "urn:schemadoc:config:v1"
	buildConfigSchemaPath = "cmd/schemadoc/doc/config.schema.json"
)

//go:embed doc/config.schema.json
var buildConfigSchemaJSON []byte

// renderBuildConfigExample renders build config example content.
func renderBuildConfigExample(
	mode schemadoc.ExampleMode,
	format schemadoc.ExampleFormat,
) ([]byte, error) {
	schemaBytes, err := readBuildConfigSchemaBytes()
	if err != nil {
		return nil, err
	}

	content, err := schemadoc.GenerateExample(schemaBytes, mode, format)
	if err != nil {
		return nil, fmt.Errorf("generate config example: %w", err)
	}
	if format == schemadoc.ExampleFormatYAML {
		content = addYAMLSchemaComment(content, schemaBytes, buildConfigSchemaPath)
	}

	return content, nil
}

// loadBuildConfig reads YAML config documents and selects documents by index.
//
// Index behavior:
// * 0     -> all documents
// * 1..N  -> selected document (1-based)
func loadBuildConfig(path string, configIndex int) ([]buildcfg.Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	schema, err := compileBuildConfigSchema()
	if err != nil {
		return nil, err
	}

	return decodeBuildConfigDocuments(content, configIndex, schema)
}

// decodeBuildConfigDocuments decodes YAML stream and returns selected documents.
func decodeBuildConfigDocuments(
	content []byte,
	configIndex int,
	schema *jsonschema.Schema,
) ([]buildcfg.Config, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	documents := make([]buildcfg.Config, 0, 4)
	var jobs []map[string]any
	var defaults map[string]any
	defaultsFound := false
	partialSchema, err := compileBuildDocumentSchema()
	if err != nil {
		return nil, err
	}

	for {
		var document any
		err := decoder.Decode(&document)
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("decode config yaml: %w", err)
		}

		if document == nil {
			continue
		}

		normalized := normalizeYAMLNode(document)
		job, ok := normalized.(map[string]any)
		if !ok {
			return nil, errBuildDocumentNotObject
		}

		if value, found := job["defaults"]; found {
			if defaultsFound {
				return nil, errDuplicateBuildDefaults
			}

			defaultsFound = true
			if len(job) != 1 {
				return nil, errMixedBuildDefaults
			}

			defaults, ok = value.(map[string]any)
			if !ok {
				return nil, errInvalidBuildDefaults
			}

			normalized = defaults
		} else {
			jobs = append(jobs, job)
		}

		if err := validateBuildConfigDocument(partialSchema, normalized); err != nil {
			return nil, err
		}
	}

	for index, job := range jobs {
		merged := mergeBuildDefaults(defaults, job)
		if err := validateBuildConfigDocument(schema, merged); err != nil {
			return nil, fmt.Errorf("config job %d: %w", index+1, err)
		}

		item, err := decodeBuildConfigDocument(merged)
		if err != nil {
			return nil, err
		}

		documents = append(documents, item)
	}

	if len(documents) == 0 {
		return nil, errors.New("config has no documents")
	}

	if configIndex < 0 {
		return nil, fmt.Errorf("config index %d out of range [0..%d]", configIndex, len(documents))
	}

	if configIndex == 0 {
		return documents, nil
	}

	if configIndex > len(documents) {
		return nil, fmt.Errorf(
			"config index %d out of range [0..%d]",
			configIndex,
			len(documents),
		)
	}

	return []buildcfg.Config{documents[configIndex-1]}, nil
}

// compileBuildDocumentSchema permits missing and removed fields during input validation.
// The original embedded schema still validates each fully merged job.
func compileBuildDocumentSchema() (*jsonschema.Schema, error) {
	var document map[string]any
	if err := decodeJSONWithNumbers(buildConfigSchemaJSON, &document); err != nil {
		return nil, err
	}

	relaxBuildConfigSchema(document)
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(buildConfigSchemaID, document); err != nil {
		return nil, err
	}

	return compiler.Compile(buildConfigSchemaID)
}

// relaxBuildConfigSchema preserves field constraints but allows partial maps and null removal.
// It changes only a fresh in-memory copy, never the generated schema file.
func relaxBuildConfigSchema(schema map[string]any) {
	delete(schema, "required")

	for _, keyword := range []string{"$defs", "properties"} {
		children, _ := schema[keyword].(map[string]any)
		for name, value := range children {
			child, ok := value.(map[string]any)
			if !ok {
				continue
			}

			relaxBuildConfigSchema(child)
			if keyword == "properties" {
				children[name] = map[string]any{"anyOf": []any{child, map[string]any{"type": "null"}}}
			}
		}
	}

	for _, keyword := range []string{"items", "additionalProperties"} {
		if child, ok := schema[keyword].(map[string]any); ok {
			relaxBuildConfigSchema(child)
		}
	}
}

// mergeBuildDefaults makes independent maps; null removes inherited keys and arrays replace.
func mergeBuildDefaults(defaults, overrides map[string]any) map[string]any {
	merged := make(map[string]any, len(defaults)+len(overrides))
	maps.Copy(merged, defaults)
	maps.Copy(merged, overrides)
	for key, value := range merged {
		if value == nil {
			delete(merged, key)
			continue
		}

		if object, ok := value.(map[string]any); ok {
			base, _ := defaults[key].(map[string]any)
			merged[key] = mergeBuildDefaults(base, object)
		}
	}

	return merged
}

// compileBuildConfigSchema compiles embedded schema for config validation.
func compileBuildConfigSchema() (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	var schemaDocument any
	schemaBytes, err := readBuildConfigSchemaBytes()
	if err != nil {
		return nil, err
	}

	if err := decodeJSONWithNumbers(schemaBytes, &schemaDocument); err != nil {
		return nil, fmt.Errorf("decode config schema: %w", err)
	}

	if err := compiler.AddResource(buildConfigSchemaID, schemaDocument); err != nil {
		return nil, fmt.Errorf("register embedded config schema: %w", err)
	}

	schema, err := compiler.Compile(buildConfigSchemaID)
	if err != nil {
		return nil, fmt.Errorf("compile embedded config schema: %w", err)
	}

	return schema, nil
}

// readBuildConfigSchemaBytes returns embedded build config schema bytes.
func readBuildConfigSchemaBytes() ([]byte, error) {
	if len(buildConfigSchemaJSON) == 0 {
		return nil, errors.New("embedded config schema is empty")
	}

	return append([]byte(nil), buildConfigSchemaJSON...), nil
}

// validateBuildConfigDocument checks one decoded document against compiled schema.
func validateBuildConfigDocument(schema *jsonschema.Schema, document any) error {
	if schema == nil {
		return errors.New("validate config: schema is nil")
	}

	if err := schema.Validate(document); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	return nil
}

// decodeBuildConfigDocument converts validated dynamic document to typed config.
func decodeBuildConfigDocument(document any) (buildcfg.Config, error) {
	raw, err := json.Marshal(document)
	if err != nil {
		return buildcfg.Config{}, fmt.Errorf("encode config document: %w", err)
	}

	var config buildcfg.Config
	if err := json.Unmarshal(raw, &config); err != nil {
		return buildcfg.Config{}, fmt.Errorf("decode config document: %w", err)
	}

	if err := validateBuildConfigStages(config); err != nil {
		return buildcfg.Config{}, err
	}

	return config, nil
}

// validateBuildConfigStages checks dependencies between declared stages.
func validateBuildConfigStages(config buildcfg.Config) error {
	if config.Mod2Doc != nil && config.Mod2Schema == nil {
		return errMod2DocRequiresMod2Schema
	}

	return nil
}

// normalizeYAMLNode recursively converts YAML map keys to strings.
func normalizeYAMLNode(node any) any {
	switch typed := node.(type) {
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, value := range typed {
			normalized[fmt.Sprint(key)] = normalizeYAMLNode(value)
		}
		return normalized

	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, value := range typed {
			normalized[key] = normalizeYAMLNode(value)
		}
		return normalized

	case []any:
		for index := range typed {
			typed[index] = normalizeYAMLNode(typed[index])
		}
		return typed

	default:
		return node
	}
}
