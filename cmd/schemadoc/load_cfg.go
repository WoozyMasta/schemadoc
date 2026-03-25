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
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/woozymasta/schemadoc"
	"github.com/woozymasta/schemadoc/cmd/schemadoc/buildcfg"
	"go.yaml.in/yaml/v3"
)

const buildConfigSchemaID = "urn:schemadoc:config:v1"

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
		if err := validateBuildConfigDocument(schema, normalized); err != nil {
			return nil, err
		}

		item, err := decodeBuildConfigDocument(normalized)
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

// compileBuildConfigSchema compiles embedded schema for config validation.
func compileBuildConfigSchema() (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	var schemaDocument any
	schemaBytes, err := readBuildConfigSchemaBytes()
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(schemaBytes, &schemaDocument); err != nil {
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

	return config, nil
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
